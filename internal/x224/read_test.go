package x224_test

import (
	"bytes"
	"io"
	"testing"

	"xqs-plugin-rdp/internal/x224"
)

// ReadPDU обязан взять ровно один пакет и не тронуть следующий байт: сразу за
// Connection Confirm по тому же потоку идёт TLS ClientHello, и лишний
// прочитанный байт увёл бы его у crypto/tls.
func TestReadPDUStopsAtPacketBoundary(t *testing.T) {
	// Connection Confirm с NEG_RSP, собранный здесь же: ReadPDU проверяется на
	// границе пакета, а не на чьей-то ещё функции.
	cc := []byte{
		0x03, 0x00, 0x00, 0x13,
		0x0E, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x08, 0x00, 0x02, 0x00, 0x00, 0x00,
	}
	trailing := []byte{0x16, 0x03, 0x01} // начало TLS-записи
	r := bytes.NewReader(append(append([]byte(nil), cc...), trailing...))

	got, err := x224.ReadPDU(r)
	if err != nil {
		t.Fatalf("ReadPDU: %v", err)
	}
	if !bytes.Equal(got, cc) {
		t.Fatalf("прочитано % x", got)
	}
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(rest, trailing) {
		t.Fatalf("в потоке осталось % x, ожидалось % x — ReadPDU забрал лишнее", rest, trailing)
	}
}

func TestReadPDURejectsBadInput(t *testing.T) {
	oversize := []byte{0x03, 0x00, 0xFF, 0xFF}
	cases := map[string][]byte{
		"пусто":                   {},
		"обрыв в заголовке":       {0x03, 0x00},
		"не TPKT":                 {0x04, 0x00, 0x00, 0x0B, 0x06, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00},
		"length меньше заголовка": {0x03, 0x00, 0x00, 0x02},
		// Длина приходит от недоверенной стороны: потолок обязан сработать до
		// аллокации, а не после.
		"length сверх потолка": append(oversize, make([]byte, 16)...),
		"тело оборвано":        {0x03, 0x00, 0x00, 0x13, 0x0E, 0xD0},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := x224.ReadPDU(bytes.NewReader(in)); err == nil {
				t.Fatal("обязана быть ошибка")
			}
		})
	}
}

func TestReadPDUAcceptsMaxSize(t *testing.T) {
	pdu := make([]byte, x224.MaxPDU)
	pdu[0] = 0x03
	pdu[2] = byte((x224.MaxPDU >> 8) & 0xFF)
	pdu[3] = byte(x224.MaxPDU & 0xFF)
	got, err := x224.ReadPDU(bytes.NewReader(pdu))
	if err != nil {
		t.Fatalf("пакет ровно по потолку отвергнут: %v", err)
	}
	if len(got) != x224.MaxPDU {
		t.Fatalf("прочитано %d байт", len(got))
	}
}
