package ipc_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"xqs-plugin-rdp/internal/ipc"
)

func writeReq(t *testing.T, buf *bytes.Buffer, body string) {
	t.Helper()
	if err := ipc.WriteFrame(buf, ipc.Frame{Kind: ipc.KindJSONRPC, ChannelID: 0, Payload: []byte(body)}); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
}

func readResp(t *testing.T, buf *bytes.Buffer) map[string]json.RawMessage {
	t.Helper()
	f, err := ipc.ReadFrame(buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if f.Kind != ipc.KindJSONRPC || f.ChannelID != 0 {
		t.Fatalf("ответ ушёл не на управляющую плоскость: kind=%#x ch=%d", f.Kind, f.ChannelID)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(f.Payload, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return m
}

func TestDispatcherRoutesControlPlaneToHandler(t *testing.T) {
	var in, out bytes.Buffer
	writeReq(t, &in, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)

	d := ipc.NewDispatcher(&out)
	d.Handle("ping", func(ipc.Request) (any, *ipc.Error) {
		return map[string]string{"pong": "yes"}, nil
	})
	if err := d.Serve(&in); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	m := readResp(t, &out)
	var result map[string]string
	if err := json.Unmarshal(m["result"], &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}
	if result["pong"] != "yes" {
		t.Fatalf("result = %v", result)
	}
}

func TestDispatcherRoutesBinaryFramesToSink(t *testing.T) {
	var in, out bytes.Buffer
	if err := ipc.WriteFrame(&in, ipc.Frame{Kind: ipc.KindChannelData, ChannelID: 5, Payload: []byte{1, 2, 3}}); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	var gotID uint32
	var gotKind byte
	var gotPayload []byte
	d := ipc.NewDispatcher(&out)
	d.OnChannel(func(id uint32, kind byte, p []byte) {
		gotID, gotKind, gotPayload = id, kind, append([]byte(nil), p...)
	})
	if err := d.Serve(&in); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if gotID != 5 || gotKind != ipc.KindChannelData || !bytes.Equal(gotPayload, []byte{1, 2, 3}) {
		t.Fatalf("sink получил id=%d kind=%#x payload=% x", gotID, gotKind, gotPayload)
	}
}

// JSON-RPC на ненулевом канале — не управляющая плоскость. Канал 0 определяет
// плоскость, а не kind: иначе чужой канал смог бы вызывать наши методы.
func TestDispatcherTreatsNonZeroChannelAsData(t *testing.T) {
	var in, out bytes.Buffer
	if err := ipc.WriteFrame(&in, ipc.Frame{Kind: ipc.KindJSONRPC, ChannelID: 9, Payload: []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)}); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	called := false
	d := ipc.NewDispatcher(&out)
	d.Handle("ping", func(ipc.Request) (any, *ipc.Error) {
		called = true
		return nil, nil
	})
	sunk := false
	d.OnChannel(func(uint32, byte, []byte) { sunk = true })
	if err := d.Serve(&in); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if called {
		t.Fatal("метод вызван с ненулевого канала — плоскость определяется channelId, а не kind")
	}
	if !sunk {
		t.Fatal("фрейм не доехал до sink")
	}
}

func TestDispatcherDoesNotAnswerNotifications(t *testing.T) {
	var in, out bytes.Buffer
	writeReq(t, &in, `{"jsonrpc":"2.0","method":"session.disconnect"}`)

	called := false
	d := ipc.NewDispatcher(&out)
	d.Handle("session.disconnect", func(ipc.Request) (any, *ipc.Error) {
		called = true
		return nil, nil
	})
	if err := d.Serve(&in); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if !called {
		t.Fatal("обработчик нотификации не вызван")
	}
	if out.Len() != 0 {
		t.Fatalf("на нотификацию отправлен ответ: % x", out.Bytes())
	}
}

// Неизвестная нотификация — норма: хост шлёт то, что ему нужно, и молчание
// правильнее ошибки, на которую он всё равно не смотрит.
func TestDispatcherIgnoresUnknownNotification(t *testing.T) {
	var in, out bytes.Buffer
	writeReq(t, &in, `{"jsonrpc":"2.0","method":"session.somethingNew"}`)

	d := ipc.NewDispatcher(&out)
	if err := d.Serve(&in); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("на неизвестную нотификацию отправлен ответ: % x", out.Bytes())
	}
}

func TestDispatcherReturnsMethodNotFound(t *testing.T) {
	var in, out bytes.Buffer
	writeReq(t, &in, `{"jsonrpc":"2.0","id":2,"method":"nope"}`)

	d := ipc.NewDispatcher(&out)
	if err := d.Serve(&in); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	m := readResp(t, &out)
	var e ipc.Error
	if err := json.Unmarshal(m["error"], &e); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if e.Code != ipc.CodeMethodNotFound {
		t.Fatalf("code = %d, ожидался %d", e.Code, ipc.CodeMethodNotFound)
	}
}

func TestDispatcherReturnsParseError(t *testing.T) {
	var in, out bytes.Buffer
	writeReq(t, &in, `{ not json`)

	d := ipc.NewDispatcher(&out)
	if err := d.Serve(&in); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	m := readResp(t, &out)
	var e ipc.Error
	if err := json.Unmarshal(m["error"], &e); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if e.Code != ipc.CodeParse {
		t.Fatalf("code = %d, ожидался %d", e.Code, ipc.CodeParse)
	}
}

// Паника обработчика не должна ронять процесс: ядро увидело бы смерть как крэш
// и запустило recovery, а пользователь — мигнувшую вкладку.
func TestDispatcherRecoversHandlerPanic(t *testing.T) {
	var in, out bytes.Buffer
	writeReq(t, &in, `{"jsonrpc":"2.0","id":3,"method":"boom"}`)

	d := ipc.NewDispatcher(&out)
	d.Handle("boom", func(ipc.Request) (any, *ipc.Error) { panic("x") })
	if err := d.Serve(&in); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	m := readResp(t, &out)
	var e ipc.Error
	if err := json.Unmarshal(m["error"], &e); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if e.Code != ipc.CodeInternal {
		t.Fatalf("код %d, ожидался %d", e.Code, ipc.CodeInternal)
	}
}

// Паника в обработчике НОТИФИКАЦИИ тоже не роняет процесс, но и ответа не
// порождает: отвечать на нотификацию нельзя даже когда очень хочется.
func TestDispatcherRecoversPanicInNotificationWithoutAnswering(t *testing.T) {
	var in, out bytes.Buffer
	writeReq(t, &in, `{"jsonrpc":"2.0","method":"boom"}`)

	d := ipc.NewDispatcher(&out)
	d.Handle("boom", func(ipc.Request) (any, *ipc.Error) { panic("x") })
	if err := d.Serve(&in); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("на нотификацию отправлен ответ: % x", out.Bytes())
	}
}

func TestDispatcherStopsOnProtocolViolation(t *testing.T) {
	var out bytes.Buffer
	raw := []byte{0x00, 0x00, 0x00, 0x01, 0x09, 0x00, 0x00, 0x00, 0x00, 0xAA} // kind 0x09 зарезервирован
	d := ipc.NewDispatcher(&out)
	err := d.Serve(bytes.NewReader(raw))
	if !errors.Is(err, ipc.ErrProtocol) {
		t.Fatalf("err = %v, ожидалась ErrProtocol — продолжать после нарушения нельзя", err)
	}
}

// Чистый конец потока — это нормальное завершение, а не ошибка: хост закрыл
// stdio, потому что сессия кончилась.
func TestDispatcherServeReturnsNilOnEOF(t *testing.T) {
	var out bytes.Buffer
	d := ipc.NewDispatcher(&out)
	if err := d.Serve(bytes.NewReader(nil)); err != nil {
		t.Fatalf("Serve на пустом потоке вернул %v, ожидался nil", err)
	}
}

func TestDispatcherServeReportsTruncation(t *testing.T) {
	var out bytes.Buffer
	raw := []byte{0x00, 0x00, 0x00, 0x04, ipc.KindChannelData, 0x00, 0x00, 0x00, 0x01, 0xAA}
	d := ipc.NewDispatcher(&out)
	err := d.Serve(bytes.NewReader(raw))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v, ожидалась io.ErrUnexpectedEOF", err)
	}
}

func TestDispatcherNotifyWritesToControlPlane(t *testing.T) {
	var out bytes.Buffer
	d := ipc.NewDispatcher(&out)
	if err := d.Notify("session.updateState", map[string]string{"state": "ready"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	m := readResp(t, &out)
	if _, ok := m["id"]; ok {
		t.Fatal("нотификация не должна нести id")
	}
	var method string
	if err := json.Unmarshal(m["method"], &method); err != nil {
		t.Fatalf("Unmarshal method: %v", err)
	}
	if method != "session.updateState" {
		t.Fatalf("method = %q", method)
	}
}
