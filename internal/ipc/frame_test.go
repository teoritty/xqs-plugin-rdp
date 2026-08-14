package ipc_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"xqs-plugin-rdp/internal/ipc"
)

func TestFrameRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		f    ipc.Frame
	}{
		{"jsonrpc on control plane", ipc.Frame{Kind: ipc.KindJSONRPC, ChannelID: 0, Payload: []byte(`{"jsonrpc":"2.0"}`)}},
		{"channel data", ipc.Frame{Kind: ipc.KindChannelData, ChannelID: 7, Payload: []byte{0xDE, 0xAD}}},
		{"credit", ipc.Frame{Kind: ipc.KindCredit, ChannelID: 7, Payload: []byte{0, 0, 0, 7, 0, 0, 0, 4}}},
		{"empty payload", ipc.Frame{Kind: ipc.KindChannelData, ChannelID: 1, Payload: []byte{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := ipc.WriteFrame(&buf, tc.f); err != nil {
				t.Fatalf("WriteFrame: %v", err)
			}
			got, err := ipc.ReadFrame(&buf)
			if err != nil {
				t.Fatalf("ReadFrame: %v", err)
			}
			if got.Kind != tc.f.Kind || got.ChannelID != tc.f.ChannelID || !bytes.Equal(got.Payload, tc.f.Payload) {
				t.Fatalf("round-trip: got %+v want %+v", got, tc.f)
			}
		})
	}
}

// Заголовок обязан быть ровно [4B len][1B kind][4B channelId], big-endian.
// Это проверяется на байтах, а не через round-trip: round-trip пройдёт и при
// согласованно неверном формате, а ядро — нет.
func TestFrameHeaderIsBigEndianNineBytes(t *testing.T) {
	var buf bytes.Buffer
	if err := ipc.WriteFrame(&buf, ipc.Frame{Kind: ipc.KindChannelData, ChannelID: 0x01020304, Payload: []byte{0xAA}}); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	want := []byte{0x00, 0x00, 0x00, 0x01, 0x02, 0x01, 0x02, 0x03, 0x04, 0xAA}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("\n got % x\nwant % x", buf.Bytes(), want)
	}
}

func TestReadFrameRejectsReservedKind(t *testing.T) {
	// 0x04-0x0F зарезервированы и запрещены в v1; 0x00 и 0xFF тоже не наши.
	for _, kind := range []byte{0x00, 0x04, 0x0F, 0xFF} {
		raw := []byte{0x00, 0x00, 0x00, 0x01, kind, 0x00, 0x00, 0x00, 0x00, 0xAA}
		_, err := ipc.ReadFrame(bytes.NewReader(raw))
		if !errors.Is(err, ipc.ErrProtocol) {
			t.Fatalf("kind %#x: err = %v, ожидалась ErrProtocol", kind, err)
		}
	}
}

// Длина проверяется ДО аллокации: она приходит от недоверенной стороны, и
// доверчивый make() здесь — это OOM по чужой команде.
func TestReadFrameRejectsOversizedLength(t *testing.T) {
	raw := []byte{0xFF, 0xFF, 0xFF, 0xFF, ipc.KindChannelData, 0x00, 0x00, 0x00, 0x01}
	_, err := ipc.ReadFrame(bytes.NewReader(raw))
	if !errors.Is(err, ipc.ErrProtocol) {
		t.Fatalf("err = %v, ожидалась ErrProtocol", err)
	}
}

func TestReadFrameRejectsOversizedJSONRPC(t *testing.T) {
	raw := []byte{0x00, 0x04, 0x00, 0x01, ipc.KindJSONRPC, 0x00, 0x00, 0x00, 0x00} // 256 KiB + 1
	_, err := ipc.ReadFrame(bytes.NewReader(raw))
	if !errors.Is(err, ipc.ErrProtocol) {
		t.Fatalf("err = %v, ожидалась ErrProtocol: потолок JSON-RPC 256 KiB", err)
	}
}

// Кредит — фиксированные 8 байт: [4B channelId][4B credit]. Другая длина
// означает, что мы неверно поняли формат.
func TestReadFrameRejectsWrongCreditLength(t *testing.T) {
	raw := []byte{0x00, 0x00, 0x00, 0x09, ipc.KindCredit, 0x00, 0x00, 0x00, 0x01}
	_, err := ipc.ReadFrame(bytes.NewReader(raw))
	if !errors.Is(err, ipc.ErrProtocol) {
		t.Fatalf("err = %v, ожидалась ErrProtocol", err)
	}
}

// Обрыв на границе фрейма — это конец потока: ядро закрыло stdio. Это НЕ
// протокольное нарушение, и путать их нельзя: реакция на них разная.
func TestReadFrameTruncatedHeaderIsNotProtocolViolation(t *testing.T) {
	_, err := ipc.ReadFrame(bytes.NewReader([]byte{0x00, 0x00}))
	if err == nil {
		t.Fatal("усечённый заголовок обязан давать ошибку")
	}
	if errors.Is(err, ipc.ErrProtocol) {
		t.Fatal("обрыв на границе — конец потока, а не нарушение протокола")
	}
}

func TestReadFrameCleanEOF(t *testing.T) {
	_, err := ipc.ReadFrame(bytes.NewReader(nil))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, ожидался io.EOF на пустом потоке", err)
	}
}

func TestReadFrameTruncatedPayload(t *testing.T) {
	raw := []byte{0x00, 0x00, 0x00, 0x04, ipc.KindChannelData, 0x00, 0x00, 0x00, 0x01, 0xAA}
	_, err := ipc.ReadFrame(bytes.NewReader(raw))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v, ожидалась io.ErrUnexpectedEOF", err)
	}
}

// Отправка сверх потолка убивает процесс на стороне ядра, поэтому писатель
// обязан отказать сам, а не полагаться на то, что вызывающий посчитал верно.
func TestWriteFrameRefusesOversizedPayload(t *testing.T) {
	big := make([]byte, ipc.MaxChannelData+1)
	err := ipc.WriteFrame(io.Discard, ipc.Frame{Kind: ipc.KindChannelData, ChannelID: 1, Payload: big})
	if !errors.Is(err, ipc.ErrProtocol) {
		t.Fatalf("err = %v, ожидалась ErrProtocol", err)
	}
}

func TestWriteFrameRefusesReservedKind(t *testing.T) {
	err := ipc.WriteFrame(io.Discard, ipc.Frame{Kind: 0x07, ChannelID: 1, Payload: []byte{1}})
	if !errors.Is(err, ipc.ErrProtocol) {
		t.Fatalf("err = %v, ожидалась ErrProtocol", err)
	}
}

// Потолок embed-stream (64 KiB) ниже общего потолка kind=0x02 (1 MiB) и на
// уровне фрейма не виден: резать обязан вызывающий, который знает purpose.
// Тест фиксирует само наличие константы и её значение.
func TestEmbedStreamCeilingIs64KiB(t *testing.T) {
	if ipc.MaxEmbedStream != 64*1024 {
		t.Fatalf("MaxEmbedStream = %d, ожидалось 65536", ipc.MaxEmbedStream)
	}
	if ipc.MaxEmbedStream >= ipc.MaxChannelData {
		t.Fatal("потолок embed-stream обязан быть строго ниже общего потолка канала")
	}
}
