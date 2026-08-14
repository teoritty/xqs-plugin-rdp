package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// Handler обслуживает один метод управляющей плоскости. Возвращённая ошибка
// становится объектом error в ответе; для нотификаций оба значения
// отбрасываются.
type Handler func(Request) (any, *Error)

// ChannelSink получает всё, что пришло не на управляющую плоскость: данные
// каналов и кредитные обновления.
type ChannelSink func(channelID uint32, kind byte, payload []byte)

// Dispatcher разводит входящий поток шины на две плоскости и сериализует
// исходящую запись.
type Dispatcher struct {
	// mu защищает запись: stdout один на всех, а фреймы обязаны выходить
	// целиком, без перемешивания.
	mu       sync.Mutex
	w        io.Writer
	handlers map[string]Handler
	sink     ChannelSink
	caller   *caller
}

func NewDispatcher(w io.Writer) *Dispatcher {
	return &Dispatcher{w: w, handlers: make(map[string]Handler), caller: newCaller()}
}

// Handle регистрирует обработчик метода. Вызывается до Serve, из одной
// горутины.
func (d *Dispatcher) Handle(method string, h Handler) { d.handlers[method] = h }

// OnChannel регистрирует приёмник бинарных фреймов.
func (d *Dispatcher) OnChannel(sink ChannelSink) { d.sink = sink }

// Serve читает фреймы до конца потока.
//
// Возвращает nil на чистом конце потока — это штатное завершение, хост закрыл
// stdio. Возвращает ErrProtocol на нарушении: продолжать после него нельзя,
// ядро уже считает нас сломанными.
func (d *Dispatcher) Serve(r io.Reader) error {
	for {
		f, err := ReadFrame(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		// Плоскость определяет channelId, а не kind: иначе фрейм с чужого
		// канала мог бы вызвать наш метод.
		if f.ChannelID == 0 && f.Kind == KindJSONRPC {
			d.serveRPC(f.Payload)
			continue
		}
		if d.sink != nil {
			d.sink(f.ChannelID, f.Kind, f.Payload)
		}
	}
}

// incoming — сообщение управляющей плоскости до того, как решено, что это.
//
// В JSON-RPC 2.0 запрос и ответ различаются наличием method, а не отдельным
// признаком. Разбирать их в один тип и решать по нему — единственный способ не
// принять ответ хоста за вызов к нам.
type incoming struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

func (d *Dispatcher) serveRPC(payload []byte) {
	var msg incoming
	if err := json.Unmarshal(payload, &msg); err != nil {
		d.write(NewError(nil, CodeParse, "malformed JSON-RPC"))
		return
	}
	// Нет method — это ответ на наш вызов.
	if msg.Method == "" {
		if len(msg.ID) == 0 {
			slog.Warn("ipc: сообщение без method и без id — отброшено")
			return
		}
		if !d.caller.deliver(&msg) {
			// Либо мы уже сдались по таймауту, либо сопоставляем неверно.
			// Молча проглотить значило бы скрыть второе.
			slog.Warn("ipc: ответ на неизвестный вызов", "id", string(msg.ID))
		}
		return
	}

	req := Request{JSONRPC: msg.JSONRPC, ID: msg.ID, Method: msg.Method, Params: msg.Params}
	h, ok := d.handlers[req.Method]
	if !ok {
		// Неизвестная нотификация — норма: хост шлёт то, что ему нужно, и
		// молчание правильнее ошибки, на которую он не смотрит.
		if !req.IsNotification() {
			d.write(NewError(req.ID, CodeMethodNotFound, "unknown method "+req.Method))
		}
		return
	}
	result, rpcErr := d.call(h, req)
	if req.IsNotification() {
		return
	}
	if rpcErr != nil {
		d.write(Response{ID: req.ID, Error: rpcErr})
		return
	}
	d.write(NewResult(req.ID, result))
}

// call изолирует панику обработчика: процесс переживает её, ядро видит обычную
// ошибку RPC вместо крэша и не запускает crash-recovery.
func (d *Dispatcher) call(h Handler, req Request) (result any, rpcErr *Error) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("ipc: паника в обработчике", "method", req.Method, "panic", fmt.Sprint(rec))
			result, rpcErr = nil, &Error{Code: CodeInternal, Message: "internal error"}
		}
	}()
	return h(req)
}

func (d *Dispatcher) write(resp Response) {
	body, err := json.Marshal(resp)
	if err != nil {
		slog.Error("ipc: не удалось сериализовать ответ", "err", err.Error())
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := WriteFrame(d.w, Frame{Kind: KindJSONRPC, ChannelID: 0, Payload: body}); err != nil {
		slog.Error("ipc: не удалось записать ответ", "err", err.Error())
	}
}

// Notify отправляет хосту нотификацию. Ответа на неё не будет и не должно быть.
func (d *Dispatcher) Notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	body, err := json.Marshal(Request{JSONRPC: jsonRPCVersion, Method: method, Params: raw})
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return WriteFrame(d.w, Frame{Kind: KindJSONRPC, ChannelID: 0, Payload: body})
}

// Handles сообщает, зарегистрирован ли обработчик метода.
//
// Нужен тестам контракта: пропущенный метод сессии означает -32601 на
// сообщение, которое хост считает обязательным, и заметить это в рантайме
// дороже, чем проверить один раз.
func (d *Dispatcher) Handles(method string) bool {
	_, ok := d.handlers[method]
	return ok
}
