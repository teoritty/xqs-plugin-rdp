package transport_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"xqs-plugin-rdp/internal/transport"
)

// fakeHost изображает шину: запоминает вызовы и отвечает по заданному правилу.
type fakeHost struct {
	mu       sync.Mutex
	calls    []string
	params   []any
	frames   map[uint32][][]byte
	openID   uint32
	callErr  error
	sendFail error
}

func newFakeHost(openID uint32) *fakeHost {
	return &fakeHost{frames: make(map[uint32][][]byte), openID: openID}
}

func (h *fakeHost) Call(_ context.Context, method string, params, result any) error {
	h.mu.Lock()
	h.calls = append(h.calls, method)
	h.params = append(h.params, params)
	err, openID := h.callErr, h.openID
	h.mu.Unlock()

	if err != nil {
		return err
	}
	if method == "channel.open" && result != nil {
		raw, mErr := json.Marshal(map[string]uint32{"channelId": openID})
		if mErr != nil {
			return mErr
		}
		return json.Unmarshal(raw, result)
	}
	return nil
}

func (h *fakeHost) SendChannelFrame(channelID uint32, payload []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sendFail != nil {
		return h.sendFail
	}
	h.frames[channelID] = append(h.frames[channelID], append([]byte(nil), payload...))
	return nil
}

func (h *fakeHost) sentOn(channelID uint32) [][]byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([][]byte, len(h.frames[channelID]))
	copy(out, h.frames[channelID])
	return out
}

func (h *fakeHost) methods() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.calls...)
}

func TestManagerOpenUsesPurposeLimits(t *testing.T) {
	cases := []struct {
		purpose  transport.Purpose
		maxFrame int
	}{
		// embed-stream режется на 64 KiB, а не на общий мегабайт: превышение
		// здесь — протокольное нарушение, а не backpressure.
		{transport.PurposeEmbedStream, 64 << 10},
		{transport.PurposeTCPRelay, 1 << 20},
	}
	for _, tc := range cases {
		t.Run(string(tc.purpose), func(t *testing.T) {
			h := newFakeHost(5)
			m := transport.NewManager(h)
			conn, err := m.Open(context.Background(), tc.purpose, "sess-1", "main")
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer conn.Close()

			// Пишем ровно на один байт больше потолка: должно уйти двумя фреймами.
			if _, err := conn.Write(make([]byte, tc.maxFrame+1)); err != nil {
				t.Fatalf("Write: %v", err)
			}
			frames := h.sentOn(5)
			if len(frames) != 2 {
				t.Fatalf("фреймов %d, ожидалось 2", len(frames))
			}
			if len(frames[0]) != tc.maxFrame {
				t.Fatalf("первый фрейм %d байт, ожидалось %d", len(frames[0]), tc.maxFrame)
			}
		})
	}
}

func TestManagerOpenSendsCorrectParams(t *testing.T) {
	h := newFakeHost(3)
	m := transport.NewManager(h)
	if _, err := m.Open(context.Background(), transport.PurposeTCPRelay, "sess-42", "main"); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := h.methods(); len(got) != 1 || got[0] != "channel.open" {
		t.Fatalf("вызовы %v", got)
	}
	raw, err := json.Marshal(h.params[0])
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var p struct {
		Purpose         string `json:"purpose"`
		ParentSessionID string `json:"parentSessionId"`
		Hint            string `json:"hint"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Purpose != "tcp-relay" || p.ParentSessionID != "sess-42" || p.Hint != "main" {
		t.Fatalf("параметры %+v", p)
	}
}

func TestManagerRejectsUnknownPurpose(t *testing.T) {
	m := transport.NewManager(newFakeHost(1))
	if _, err := m.Open(context.Background(), transport.Purpose("exec"), "s", ""); err == nil {
		t.Fatal("незаявленное назначение принято — хост отказал бы в рантайме")
	}
}

func TestManagerSurfacesOpenFailure(t *testing.T) {
	h := newFakeHost(1)
	h.callErr = errors.New("capability denied")
	m := transport.NewManager(h)
	if _, err := m.Open(context.Background(), transport.PurposeTCPRelay, "s", ""); err == nil {
		t.Fatal("отказ хоста проглочен")
	}
}

func TestManagerRoutesDataToConn(t *testing.T) {
	h := newFakeHost(11)
	m := transport.NewManager(h)
	conn, err := m.Open(context.Background(), transport.PurposeTCPRelay, "s", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	m.OnFrame(11, 0x02, []byte("payload"))
	buf := make([]byte, 16)
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf[:n]) != "payload" {
		t.Fatalf("прочитано %q", buf[:n])
	}
}

// Фрейм на закрытый канал — no-op, а не ошибка: у channel.close нулевой grace
// period, и разминуться с ним в полёте — норма.
func TestManagerDropsFramesForUnknownChannel(t *testing.T) {
	m := transport.NewManager(newFakeHost(1))
	m.OnFrame(999, 0x02, []byte("nobody home"))
	m.OnFrame(999, 0x03, make([]byte, 8))
}

func TestManagerAppliesCredit(t *testing.T) {
	h := newFakeHost(7)
	m := transport.NewManager(h)
	conn, err := m.Open(context.Background(), transport.PurposeTCPRelay, "s", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// Начальный кредит tcp-relay — 4 ФРЕЙМА, а не 4 байта: при потолке в
	// мегабайт короткая запись тратит ровно один кредит.
	for i := 0; i < 4; i++ {
		if _, err := conn.Write([]byte("x")); err != nil {
			t.Fatalf("Write #%d: %v", i, err)
		}
	}
	// Окно исчерпано — следующая запись обязана ждать пополнения.
	credit := make([]byte, 8)
	binary.BigEndian.PutUint32(credit[0:4], 7)
	binary.BigEndian.PutUint32(credit[4:8], 4)

	done := make(chan error, 1)
	go func() {
		_, err := conn.Write([]byte("y"))
		done <- err
	}()
	time.Sleep(30 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("запись на исчерпанном окне не заблокировалась: %v", err)
	default:
	}
	m.OnFrame(7, 0x03, credit)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Write после пополнения: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("кредит не применился — запись не разблокировалась")
	}
}

// Идентификатор канала есть и в заголовке, и в теле кредитного фрейма.
// Расхождение означает, что мы неверно поняли формат: применить кредит
// «куда-нибудь» хуже, чем не применить его вовсе.
func TestManagerIgnoresCreditAddressedElsewhere(t *testing.T) {
	h := newFakeHost(7)
	m := transport.NewManager(h)
	conn, err := m.Open(context.Background(), transport.PurposeTCPRelay, "s", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	credit := make([]byte, 8)
	binary.BigEndian.PutUint32(credit[0:4], 8) // не тот канал
	binary.BigEndian.PutUint32(credit[4:8], 100)
	m.OnFrame(7, 0x03, credit)

	// Тратим начальные 4 кредита по одному фрейму; пятая запись обязана
	// упереться в дедлайн, а не проехать на чужом кредите.
	for i := 0; i < 4; i++ {
		if _, err := conn.Write([]byte("x")); err != nil {
			t.Fatalf("Write #%d: %v", i, err)
		}
	}
	if err := conn.SetWriteDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	if _, err := conn.Write([]byte("x")); err == nil {
		t.Fatal("чужой кредит применён к нашему окну")
	}
}

func TestManagerIgnoresMalformedCredit(t *testing.T) {
	h := newFakeHost(7)
	m := transport.NewManager(h)
	conn, err := m.Open(context.Background(), transport.PurposeTCPRelay, "s", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()
	m.OnFrame(7, 0x03, []byte{1, 2, 3})
	m.OnFrame(7, 0x09, []byte{1})
}

// Прочитанные фреймы обязаны превращаться в кредит, уходящий хосту: иначе
// источник встанет и больше не поедет.
func TestManagerGrantsCreditAsDataIsRead(t *testing.T) {
	h := newFakeHost(4)
	m := transport.NewManager(h)
	conn, err := m.Open(context.Background(), transport.PurposeTCPRelay, "s", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	m.OnFrame(4, 0x02, []byte("abc"))
	buf := make([]byte, 8)
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}

	frames := h.sentOn(4)
	if len(frames) != 1 {
		t.Fatalf("хосту отправлено %d фреймов, ожидался 1 кредитный", len(frames))
	}
	if len(frames[0]) != 8 {
		t.Fatalf("кредитный фрейм %d байт, ожидалось 8", len(frames[0]))
	}
	if got := binary.BigEndian.Uint32(frames[0][0:4]); got != 4 {
		t.Fatalf("кредит адресован каналу %d", got)
	}
	if got := binary.BigEndian.Uint32(frames[0][4:8]); got != 1 {
		t.Fatalf("выдано %d кредитов, ожидался 1", got)
	}
}

func TestManagerCloseIsIdempotentAndNotifiesOnce(t *testing.T) {
	h := newFakeHost(2)
	m := transport.NewManager(h)
	if _, err := m.Open(context.Background(), transport.PurposeTCPRelay, "s", ""); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := m.Close(context.Background(), 2, "done"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := m.Close(context.Background(), 2, "done"); err != nil {
		t.Fatalf("повторный Close: %v", err)
	}
	methods := h.methods()
	closes := 0
	for _, mth := range methods {
		if mth == "channel.close" {
			closes++
		}
	}
	if closes != 1 {
		t.Fatalf("channel.close отправлен %d раз, ожидался 1", closes)
	}
}

// Закрытие, инициированное хостом, не должно порождать встречное закрытие:
// иначе мы отвечаем ему его же сообщением.
func TestManagerRemoteCloseDoesNotCallHost(t *testing.T) {
	h := newFakeHost(6)
	m := transport.NewManager(h)
	if _, err := m.Open(context.Background(), transport.PurposeTCPRelay, "s", ""); err != nil {
		t.Fatalf("Open: %v", err)
	}
	m.OnRemoteClose(6)
	for _, mth := range h.methods() {
		if mth == "channel.close" {
			t.Fatal("на закрытие хостом отправлено встречное channel.close")
		}
	}
}

// На сносе сессии ядро закрывает каналы синхронно ДО удаления объекта сессии,
// поэтому обращаться к нему уже нельзя — только закрыться локально.
func TestManagerCloseAllIsLocalOnly(t *testing.T) {
	h := newFakeHost(1)
	m := transport.NewManager(h)
	conn, err := m.Open(context.Background(), transport.PurposeTCPRelay, "s", "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	m.CloseAll()
	for _, mth := range h.methods() {
		if mth == "channel.close" {
			t.Fatal("CloseAll обратился к хосту")
		}
	}
	if _, err := conn.Read(make([]byte, 4)); err == nil {
		t.Fatal("соединение осталось живым после CloseAll")
	}
}
