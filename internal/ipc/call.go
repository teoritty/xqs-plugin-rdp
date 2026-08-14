package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
)

// ErrCallFailed — хост ответил объектом error.
type ErrCallFailed struct {
	Method string
	Err    *Error
}

func (e *ErrCallFailed) Error() string {
	return fmt.Sprintf("ipc: %s отклонён хостом: %s (код %d)", e.Method, e.Err.Message, e.Err.Code)
}

// caller хранит вызовы, ждущие ответа.
//
// Живёт отдельным типом внутри диспетчера, потому что у него своя причина для
// изменения: сопоставление ответов с запросами — задача, не связанная ни с
// маршрутизацией входящих вызовов, ни с фреймингом.
type caller struct {
	nextID  atomic.Uint64
	mu      sync.Mutex
	pending map[string]chan *incoming
}

func newCaller() *caller {
	return &caller{pending: make(map[string]chan *incoming)}
}

func (c *caller) register() (string, chan *incoming) {
	id := strconv.FormatUint(c.nextID.Add(1), 10)
	ch := make(chan *incoming, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	return id, ch
}

func (c *caller) forget(id string) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// deliver отдаёт ответ ожидающему. Сообщает, нашёлся ли адресат: ответ на
// неизвестный id означает, что мы либо уже сдались по таймауту, либо неверно
// сопоставляем — и то и другое стоит увидеть в логе, а не проглотить.
func (c *caller) deliver(msg *incoming) bool {
	id := string(msg.ID)
	c.mu.Lock()
	ch, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.mu.Unlock()
	if !ok {
		return false
	}
	ch <- msg
	return true
}

// Call выполняет синхронный вызов хоста.
//
// Таймаут задаётся контекстом вызывающего, а не зашит здесь: у методов он
// разный (channel.open — 10 с, остальные — 5 с), и зашитое значение пришлось бы
// либо занижать, либо завышать для всех сразу.
//
// result может быть nil, если ответ не нужен.
func (d *Dispatcher) Call(ctx context.Context, method string, params any, result any) error {
	id, ch := d.caller.register()
	defer d.caller.forget(id)

	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("ipc: параметры %s: %w", method, err)
		}
		rawParams = b
	}
	body, err := json.Marshal(Request{
		JSONRPC: jsonRPCVersion,
		ID:      json.RawMessage(id),
		Method:  method,
		Params:  rawParams,
	})
	if err != nil {
		return fmt.Errorf("ipc: сборка %s: %w", method, err)
	}

	d.mu.Lock()
	err = WriteFrame(d.w, Frame{Kind: KindJSONRPC, ChannelID: 0, Payload: body})
	d.mu.Unlock()
	if err != nil {
		return fmt.Errorf("ipc: отправка %s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("ipc: %s: %w", method, ctx.Err())
	case msg := <-ch:
		if msg.Error != nil {
			return &ErrCallFailed{Method: method, Err: msg.Error}
		}
		if result == nil {
			return nil
		}
		if len(msg.Result) == 0 {
			return errors.New("ipc: " + method + ": пустой результат")
		}
		if err := json.Unmarshal(msg.Result, result); err != nil {
			return fmt.Errorf("ipc: разбор результата %s: %w", method, err)
		}
		return nil
	}
}

// SendChannelFrame отправляет данные бинарного канала.
//
// Кредитом здесь не управляем: это делает transport.Conn, который знает своё
// окно. Смешивать учёт кредита с записью означало бы держать одну и ту же
// политику в двух местах.
func (d *Dispatcher) SendChannelFrame(channelID uint32, payload []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return WriteFrame(d.w, Frame{Kind: KindChannelData, ChannelID: channelID, Payload: payload})
}

// SendCreditFrame отправляет кредитное обновление (kind=0x03).
//
// Отдельный метод, а не флаг у SendChannelFrame: вид фрейма — это протокол, а
// не параметр. Спутать их стоило одной сессии: кредит, ушедший как данные
// канала, доезжает до другого конца как восемь байт мусора в начале потока —
// браузер закрывает соединение, RDP-сервер обрывает его как нарушение.
func (d *Dispatcher) SendCreditFrame(channelID uint32, payload []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return WriteFrame(d.w, Frame{Kind: KindCredit, ChannelID: channelID, Payload: payload})
}
