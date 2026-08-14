package transport

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
)

// Purpose — закрытый перечень назначений канала, который валидирует хост.
// Объявлять их строками по месту вызова значило бы получать отказ capability
// из-за опечатки, причём в рантайме.
type Purpose string

const (
	PurposeEmbedStream Purpose = "embed-stream"
	PurposeTCPRelay    Purpose = "tcp-relay"
)

// Параметры окон и потолки фреймов по назначению (docs/plugin-api.md).
//
// Они разные не случайно: embed-stream рассчитан на фреймбуфер — окно шире, а
// фрейм вдвое-шестнадцатеро меньше, потому что кредит считается в фреймах, и
// восемь фреймов по мегабайту припарковали бы в памяти хоста 8 МБ на канал.
const (
	creditEmbedStream   = 8
	creditTCPRelay      = 4
	maxFrameEmbedStream = 64 << 10
	maxFrameTCPRelay    = 1 << 20
)

func purposeLimits(p Purpose) (credit, maxFrame int, ok bool) {
	switch p {
	case PurposeEmbedStream:
		return creditEmbedStream, maxFrameEmbedStream, true
	case PurposeTCPRelay:
		return creditTCPRelay, maxFrameTCPRelay, true
	default:
		return 0, 0, false
	}
}

// Host — то, что transport требует от шины.
//
// Интерфейс, а не *ipc.Dispatcher: он держит transport независимым от IPC и
// позволяет проверять открытие каналов на фейке. Заодно это тот шов, на
// котором видно, что transport умеет ровно две вещи — звать хост и слать байты.
type Host interface {
	Call(ctx context.Context, method string, params, result any) error
	SendChannelFrame(channelID uint32, payload []byte) error
}

// Manager открывает каналы и разводит по ним входящие фреймы.
type Manager struct {
	host Host

	mu      sync.RWMutex
	conns   map[uint32]*Conn
	windows map[uint32]*SendWindow
}

func NewManager(host Host) *Manager {
	return &Manager{
		host:    host,
		conns:   make(map[uint32]*Conn),
		windows: make(map[uint32]*SendWindow),
	}
}

type openParams struct {
	Purpose         string `json:"purpose"`
	ParentSessionID string `json:"parentSessionId"`
	Hint            string `json:"hint,omitempty"`
}

type openResult struct {
	ChannelID uint32 `json:"channelId"`
}

// Open открывает канал и отдаёт готовое к работе соединение.
//
// Контекст задаёт таймаут: у channel.open он 10 с, а не стандартные 5 — хост за
// это время дозванивается до цели. Ставить его здесь нельзя, потому что
// вызывающий вправе иметь свой, более короткий срок.
func (m *Manager) Open(ctx context.Context, purpose Purpose, sessionID, hint string) (*Conn, error) {
	credit, maxFrame, ok := purposeLimits(purpose)
	if !ok {
		return nil, fmt.Errorf("transport: неизвестное назначение канала %q", purpose)
	}

	var res openResult
	err := m.host.Call(ctx, "channel.open", openParams{
		Purpose:         string(purpose),
		ParentSessionID: sessionID,
		Hint:            hint,
	}, &res)
	if err != nil {
		return nil, fmt.Errorf("transport: открытие канала %s: %w", purpose, err)
	}

	sendWin := NewSendWindow(credit)
	id := res.ChannelID
	recvWin := NewRecvWindow(credit, func(n int) { m.grant(id, n) })
	conn := NewConn(id, Addr{Purpose: string(purpose), ChannelID: id}, m.host, sendWin, recvWin, maxFrame, credit)

	m.mu.Lock()
	m.conns[id] = conn
	m.windows[id] = sendWin
	m.mu.Unlock()

	return conn, nil
}

// OnFrame принимает всё, что пришло не на управляющую плоскость.
//
// Подключается к ipc.Dispatcher.OnChannel в композиционном корне.
func (m *Manager) OnFrame(channelID uint32, kind byte, payload []byte) {
	switch kind {
	case kindChannelData:
		m.deliver(channelID, payload)
	case kindCredit:
		m.applyCredit(channelID, payload)
	default:
		// Неизвестный kind — нарушение, но реагирует на него ядро: оно уже
		// закрыло бы stdio, если бы фрейм был неверным. Нам остаётся не
		// притворяться, что мы его поняли.
		slog.Warn("transport: фрейм неизвестного вида", "channelId", channelID, "kind", kind)
	}
}

func (m *Manager) deliver(channelID uint32, payload []byte) {
	m.mu.RLock()
	conn := m.conns[channelID]
	m.mu.RUnlock()
	if conn == nil {
		// Канал уже закрыт: после channel.close обе стороны роняют его фреймы
		// как no-op, а не как ошибку.
		return
	}
	if err := conn.Deliver(payload); err != nil {
		slog.Debug("transport: фрейм отброшен", "channelId", channelID, "err", err.Error())
	}
}

// applyCredit разбирает кредитное обновление: [4B channelId][4B credit].
func (m *Manager) applyCredit(channelID uint32, payload []byte) {
	if len(payload) != 8 {
		slog.Warn("transport: кредитный фрейм неверной длины", "channelId", channelID, "len", len(payload))
		return
	}
	inner := binary.BigEndian.Uint32(payload[0:4])
	credit := binary.BigEndian.Uint32(payload[4:8])
	// Идентификатор есть и в заголовке, и в теле. Расхождение означает, что мы
	// неверно поняли формат, — и молча выбрать один из двух хуже, чем сказать.
	if inner != channelID {
		slog.Warn("transport: кредит адресован другому каналу",
			"headerChannelId", channelID, "payloadChannelId", inner)
		return
	}

	m.mu.RLock()
	win := m.windows[channelID]
	m.mu.RUnlock()
	if win == nil {
		return
	}
	win.Grant(int(credit))
}

// grant отправляет кредитное обновление хосту.
func (m *Manager) grant(channelID uint32, n int) {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[0:4], channelID)
	binary.BigEndian.PutUint32(payload[4:8], uint32(n))
	if err := m.host.SendChannelFrame(channelID, payload); err != nil {
		slog.Warn("transport: не удалось выдать кредит", "channelId", channelID, "err", err.Error())
	}
}

type closeParams struct {
	ChannelID uint32 `json:"channelId"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
}

// Close закрывает канал и уведомляет хост.
//
// Идемпотентен: ядро закрывает каналы само при сносе сессии, и второй вызов —
// обычное дело, а не ошибка.
func (m *Manager) Close(ctx context.Context, channelID uint32, reason string) error {
	m.mu.Lock()
	conn := m.conns[channelID]
	delete(m.conns, channelID)
	delete(m.windows, channelID)
	m.mu.Unlock()

	if conn == nil {
		return nil
	}
	_ = conn.Close()
	return m.host.Call(ctx, "channel.close", closeParams{ChannelID: channelID, Reason: reason}, nil)
}

// OnRemoteClose обрабатывает channel.close, пришедший от хоста.
//
// Отдельно от Close: здесь уведомлять хост не нужно — он и есть инициатор, — а
// перепутать эти два пути значило бы отправить ему закрытие в ответ на его же
// закрытие.
func (m *Manager) OnRemoteClose(channelID uint32) {
	m.mu.Lock()
	conn := m.conns[channelID]
	delete(m.conns, channelID)
	delete(m.windows, channelID)
	m.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
}

// CloseAll закрывает всё локально, без обращений к хосту.
//
// Нужен на сносе сессии: ядро закрывает каналы синхронно ДО удаления объекта
// сессии, и рассчитывать, что мы успеем ещё что-то ему сказать, нельзя.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	conns := make([]*Conn, 0, len(m.conns))
	for id, c := range m.conns {
		conns = append(conns, c)
		delete(m.conns, id)
		delete(m.windows, id)
	}
	m.mu.Unlock()

	for _, c := range conns {
		_ = c.Close()
	}
}

// Виды фреймов дублируются здесь как константы пакета, чтобы transport не
// импортировал ipc ради двух байтов: зависимость ради констант перевесила бы
// то, что она стоит.
const (
	kindChannelData byte = 0x02
	kindCredit      byte = 0x03
)
