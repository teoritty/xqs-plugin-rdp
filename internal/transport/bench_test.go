package transport_test

import (
	"testing"

	"xqs-plugin-rdp/internal/transport"
)

// discardSender — приёмник фреймов без собственной стоимости: измеряем путь
// записи канала, а не скорость io.Discard.
type discardSender struct{ n int }

func (d *discardSender) SendChannelFrame(_ uint32, payload []byte) error {
	d.n += len(payload)
	return nil
}

// BenchmarkConnWrite меряет горячий путь: то, что происходит на КАЖДЫЙ кадр
// RDP по дороге к браузеру. Здесь важны не столько наносекунды, сколько
// аллокации: при всплеске кадров именно они превращаются в работу сборщика и
// в просадку.
func BenchmarkConnWrite(b *testing.B) {
	sizes := map[string]int{
		"12KiB": 12 << 10,
		"32KiB": 32 << 10,
	}
	for name, size := range sizes {
		b.Run(name, func(b *testing.B) {
			sender := &discardSender{}
			// Кредита заведомо хватает: измеряется путь без ожидания.
			sendWin := transport.NewSendWindow(b.N + 16)
			recvWin := transport.NewRecvWindow(8, func(int) {})
			conn := transport.NewConn(1, transport.Addr{}, sender, sendWin, recvWin, 64<<10, 8)
			defer conn.Close()

			payload := make([]byte, size)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := conn.Write(payload); err != nil {
					b.Fatalf("Write: %v", err)
				}
			}
		})
	}
}
