package transport_test

import (
	"testing"

	"xqs-plugin-rdp/internal/transport"
)

// recordingSender запоминает сам срез, а не его содержимое: проверяется
// владение памятью, а не байты.
type recordingSender struct {
	frames [][]byte
}

func (s *recordingSender) SendChannelFrame(_ uint32, payload []byte) error {
	s.frames = append(s.frames, payload)
	return nil
}

// Conn отдаёт отправителю срез вызывающего БЕЗ копии — это решение, а не
// недосмотр, и оно держится на контракте FrameSender: фрейм уходит синхронно.
//
// Тест закрепляет обе половины. Если однажды копию вернут «на всякий случай»,
// сюда придёт 32 КБ мусора на каждый кадр — ровно то, что вызывало просадку
// при всплеске кадров.
func TestWritePassesTheCallersBufferWithoutCopying(t *testing.T) {
	sender := &recordingSender{}
	conn := transport.NewConn(1, transport.Addr{}, sender,
		transport.NewSendWindow(4), transport.NewRecvWindow(8, func(int) {}), 64<<10, 4)
	defer conn.Close()

	payload := []byte("кадр")
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(sender.frames) != 1 {
		t.Fatalf("фреймов %d, ожидался 1", len(sender.frames))
	}
	if &sender.frames[0][0] != &payload[0] {
		t.Fatal("фрейм скопирован — контракт FrameSender позволяет обойтись без этого")
	}
}

// Большая запись режется по потолку фрейма, и каждый кусок — это окно
// вызывающего, а не новая память. Превышение потолка ядро считает нарушением
// протокола и убивает процесс, поэтому режем именно мы.
func TestWriteSplitsAtTheFrameCeilingWithoutCopying(t *testing.T) {
	sender := &recordingSender{}
	const maxFrame = 1024
	conn := transport.NewConn(1, transport.Addr{}, sender,
		transport.NewSendWindow(8), transport.NewRecvWindow(8, func(int) {}), maxFrame, 4)
	defer conn.Close()

	payload := make([]byte, maxFrame*2+7)
	for i := range payload {
		payload[i] = byte(i)
	}
	n, err := conn.Write(payload)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("записано %d из %d", n, len(payload))
	}
	if len(sender.frames) != 3 {
		t.Fatalf("фреймов %d, ожидалось 3", len(sender.frames))
	}
	for i, f := range sender.frames {
		if len(f) > maxFrame {
			t.Fatalf("фрейм %d длиной %d — сверх потолка %d", i, len(f), maxFrame)
		}
	}
	if &sender.frames[0][0] != &payload[0] {
		t.Fatal("первый кусок скопирован")
	}
	if &sender.frames[1][0] != &payload[maxFrame] {
		t.Fatal("второй кусок не является окном исходного буфера")
	}
}
