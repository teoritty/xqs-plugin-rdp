package transport_test

import (
	"runtime"
	"testing"

	"xqs-plugin-rdp/internal/transport"
)

type countingSender struct{ frames int }

func (s *countingSender) SendChannelFrame(_ uint32, _ []byte) error {
	s.frames++
	return nil
}

// Долгая сессия не должна оставлять за собой горутины.
//
// Раньше каждый кадр поднимал две сторожевые горутины и два канала — на
// секунду видео это тысячи. Они завершались, но платить за них приходилось
// ровно в тот момент, когда кадров много, то есть при всплеске. Тест сторожит
// именно это: путь записи при доступном кредите обязан быть без горутин.
func TestWriteDoesNotSpawnGoroutinesPerFrame(t *testing.T) {
	sender := &countingSender{}
	const frames = 50000
	conn := transport.NewConn(1, transport.Addr{}, sender,
		transport.NewSendWindow(frames+8), transport.NewRecvWindow(8, func(int) {}), 64<<10, 8)
	defer conn.Close()

	payload := make([]byte, 4096)
	runtime.GC()
	before := runtime.NumGoroutine()

	for i := 0; i < frames; i++ {
		if _, err := conn.Write(payload); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	runtime.GC()
	after := runtime.NumGoroutine()
	if sender.frames != frames {
		t.Fatalf("отправлено %d кадров из %d", sender.frames, frames)
	}
	// Допуск на планировщик, но не на горутину-на-кадр: та дала бы порядки.
	if after > before+5 {
		t.Fatalf("горутин было %d, стало %d — путь записи их плодит", before, after)
	}
}

// Тот же путь, но без единой аллокации: мусор на кадр — это работа сборщика
// ровно во время всплеска кадров.
func TestWriteAllocatesNothingPerFrame(t *testing.T) {
	sender := &countingSender{}
	conn := transport.NewConn(1, transport.Addr{}, sender,
		transport.NewSendWindow(1<<20), transport.NewRecvWindow(8, func(int) {}), 64<<10, 8)
	defer conn.Close()

	payload := make([]byte, 16<<10)
	got := testing.AllocsPerRun(200, func() {
		if _, err := conn.Write(payload); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})
	if got != 0 {
		t.Fatalf("аллокаций на кадр: %.1f, ожидалось 0", got)
	}
}
