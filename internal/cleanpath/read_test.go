package cleanpath_test

import (
	"bytes"
	"io"
	"testing"

	"xqs-plugin-rdp/internal/cleanpath"
)

// Граница сообщения задаётся DER, а не транспортом: сразу за
// RDCleanPath-запросом по тому же потоку идут байты RDP, и прочитать лишнее
// значит потерять их начало.
func TestReadMessageStopsAtDERBoundary(t *testing.T) {
	msg, err := cleanpath.BuildResponse([]byte{0x03, 0x00}, [][]byte{{0x30, 0x00}}, "h")
	if err != nil {
		t.Fatalf("BuildResponse: %v", err)
	}
	trailing := []byte{0xAA, 0xBB, 0xCC}
	r := bytes.NewReader(append(append([]byte(nil), msg...), trailing...))

	got, err := cleanpath.ReadMessage(r)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if !bytes.Equal(got, msg) {
		t.Fatalf("прочитано % x", got)
	}
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(rest, trailing) {
		t.Fatalf("в потоке осталось % x — ReadMessage забрал лишнее", rest)
	}
}

// Сообщение может прийти по частям: WebSocket вправе его разрезать, и
// полагаться на «придёт одним куском» нельзя.
func TestReadMessageHandlesSplitDelivery(t *testing.T) {
	msg, err := cleanpath.BuildResponse(bytes.Repeat([]byte{0x41}, 300), nil, "host")
	if err != nil {
		t.Fatalf("BuildResponse: %v", err)
	}
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		for i := 0; i < len(msg); i += 7 {
			end := i + 7
			if end > len(msg) {
				end = len(msg)
			}
			if _, err := pw.Write(msg[i:end]); err != nil {
				return
			}
		}
	}()

	got, err := cleanpath.ReadMessage(pr)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if !bytes.Equal(got, msg) {
		t.Fatalf("собрано %d байт из %d", len(got), len(msg))
	}
}

func TestReadMessageShortFormLength(t *testing.T) {
	msg := []byte{0x30, 0x03, 0x02, 0x01, 0x07}
	got, err := cleanpath.ReadMessage(bytes.NewReader(msg))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if !bytes.Equal(got, msg) {
		t.Fatalf("прочитано % x", got)
	}
}

func TestReadMessageRejectsBadInput(t *testing.T) {
	cases := map[string][]byte{
		"пусто":                {},
		"обрыв в заголовке":    {0x30},
		"не SEQUENCE":          {0x02, 0x01, 0x07},
		"неопределённая длина": {0x30, 0x80},
		"длина в 5 байт":       {0x30, 0x85, 1, 2, 3, 4, 5},
		// Длину пишет страница в iframe — недоверенная сторона. Потолок обязан
		// сработать до аллокации.
		"сверх потолка": {0x30, 0x83, 0xFF, 0xFF, 0xFF},
		"тело оборвано": {0x30, 0x10, 0x01, 0x02},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := cleanpath.ReadMessage(bytes.NewReader(in)); err == nil {
				t.Fatal("обязана быть ошибка")
			}
		})
	}
}
