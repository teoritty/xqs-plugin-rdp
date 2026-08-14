package ipc_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"xqs-plugin-rdp/internal/ipc"
)

// hostPipe изображает хост: читает наши фреймы и отвечает на них по правилу,
// которое задаёт тест.
type hostPipe struct {
	toPlugin   *io.PipeReader
	toPluginW  *io.PipeWriter
	fromPlugin *io.PipeWriter
	fromRead   *io.PipeReader
}

func newHostPipe() *hostPipe {
	tr, tw := io.Pipe()
	fr, fw := io.Pipe()
	return &hostPipe{toPlugin: tr, toPluginW: tw, fromPlugin: fw, fromRead: fr}
}

// serveOnce читает один запрос плагина и отвечает на него.
func (h *hostPipe) serveOnce(t *testing.T, reply func(id json.RawMessage, method string) ipc.Response) {
	t.Helper()
	f, err := ipc.ReadFrame(h.fromRead)
	if err != nil {
		t.Errorf("хост: чтение запроса: %v", err)
		return
	}
	var req ipc.Request
	if err := json.Unmarshal(f.Payload, &req); err != nil {
		t.Errorf("хост: разбор запроса: %v", err)
		return
	}
	body, err := json.Marshal(reply(req.ID, req.Method))
	if err != nil {
		t.Errorf("хост: сборка ответа: %v", err)
		return
	}
	if err := ipc.WriteFrame(h.toPluginW, ipc.Frame{Kind: ipc.KindJSONRPC, ChannelID: 0, Payload: body}); err != nil {
		t.Errorf("хост: запись ответа: %v", err)
	}
}

func TestCallReturnsResult(t *testing.T) {
	h := newHostPipe()
	d := ipc.NewDispatcher(h.fromPlugin)
	go func() { _ = d.Serve(h.toPlugin) }()
	go h.serveOnce(t, func(id json.RawMessage, _ string) ipc.Response {
		return ipc.NewResult(id, map[string]any{"channelId": 42})
	})

	var got struct {
		ChannelID uint32 `json:"channelId"`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.Call(ctx, "channel.open", map[string]string{"purpose": "tcp-relay"}, &got); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got.ChannelID != 42 {
		t.Fatalf("channelId = %d", got.ChannelID)
	}
}

// Ошибка хоста обязана доходить целиком: код -32001 (отказ capability) и
// -32003 (лимит) означают разное и лечатся по-разному.
func TestCallSurfacesHostError(t *testing.T) {
	h := newHostPipe()
	d := ipc.NewDispatcher(h.fromPlugin)
	go func() { _ = d.Serve(h.toPlugin) }()
	go h.serveOnce(t, func(id json.RawMessage, _ string) ipc.Response {
		return ipc.NewError(id, -32001, "capability denied")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := d.Call(ctx, "channel.open", nil, nil)
	if err == nil {
		t.Fatal("отказ хоста проглочен")
	}
	var failed *ipc.ErrCallFailed
	if !errors.As(err, &failed) {
		t.Fatalf("err типа %T, ожидался *ipc.ErrCallFailed", err)
	}
	if failed.Err.Code != -32001 {
		t.Fatalf("код %d", failed.Err.Code)
	}
	if failed.Method != "channel.open" {
		t.Fatalf("метод %q — без него непонятно, что именно отклонено", failed.Method)
	}
}

// Без таймаута молчащий хост подвесил бы вкладку навсегда в состоянии
// connecting: пользователь увидел бы не ошибку, а вечное ожидание.
func TestCallRespectsContextDeadline(t *testing.T) {
	h := newHostPipe()
	d := ipc.NewDispatcher(h.fromPlugin)
	go func() { _ = d.Serve(h.toPlugin) }()
	// Хост читает запрос и не отвечает.
	go func() { _, _ = ipc.ReadFrame(h.fromRead) }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := d.Call(ctx, "channel.open", nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, ожидался context.DeadlineExceeded", err)
	}
}

// Ответы обязаны сопоставляться по id, а не по порядку прихода: хост вправе
// отвечать не в том порядке, в каком его спрашивали.
func TestCallsAreMatchedByIDNotByOrder(t *testing.T) {
	h := newHostPipe()
	d := ipc.NewDispatcher(h.fromPlugin)
	go func() { _ = d.Serve(h.toPlugin) }()

	go func() {
		// Собираем два запроса, отвечаем в обратном порядке.
		var ids []json.RawMessage
		var methods []string
		for i := 0; i < 2; i++ {
			f, err := ipc.ReadFrame(h.fromRead)
			if err != nil {
				t.Errorf("хост: %v", err)
				return
			}
			var req ipc.Request
			if err := json.Unmarshal(f.Payload, &req); err != nil {
				t.Errorf("хост: %v", err)
				return
			}
			ids = append(ids, req.ID)
			methods = append(methods, req.Method)
		}
		for i := len(ids) - 1; i >= 0; i-- {
			body, err := json.Marshal(ipc.NewResult(ids[i], map[string]string{"method": methods[i]}))
			if err != nil {
				t.Errorf("хост: %v", err)
				return
			}
			if err := ipc.WriteFrame(h.toPluginW, ipc.Frame{Kind: ipc.KindJSONRPC, ChannelID: 0, Payload: body}); err != nil {
				t.Errorf("хост: %v", err)
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	results := make([]string, 2)
	for i, method := range []string{"first.method", "second.method"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var got struct {
				Method string `json:"method"`
			}
			if err := d.Call(ctx, method, nil, &got); err != nil {
				t.Errorf("Call %s: %v", method, err)
				return
			}
			results[i] = got.Method
		}()
	}
	wg.Wait()
	if results[0] != "first.method" || results[1] != "second.method" {
		t.Fatalf("ответы перепутаны: %v", results)
	}
}

func TestSendChannelFrameWritesBinaryFrame(t *testing.T) {
	h := newHostPipe()
	d := ipc.NewDispatcher(h.fromPlugin)

	go func() {
		if err := d.SendChannelFrame(9, []byte{1, 2, 3}); err != nil {
			t.Errorf("SendChannelFrame: %v", err)
		}
	}()

	f, err := ipc.ReadFrame(h.fromRead)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if f.Kind != ipc.KindChannelData {
		t.Fatalf("kind = %#x, ожидался KindChannelData", f.Kind)
	}
	if f.ChannelID != 9 {
		t.Fatalf("channelId = %d", f.ChannelID)
	}
	if string(f.Payload) != string([]byte{1, 2, 3}) {
		t.Fatalf("payload = % x", f.Payload)
	}
}

// Ответ на неизвестный id не должен ронять диспетчер: он мог прийти после
// того, как мы сдались по таймауту.
func TestUnknownResponseIsIgnored(t *testing.T) {
	h := newHostPipe()
	d := ipc.NewDispatcher(h.fromPlugin)
	served := make(chan error, 1)
	go func() { served <- d.Serve(h.toPlugin) }()

	body, err := json.Marshal(ipc.NewResult(json.RawMessage(`"999"`), map[string]string{"x": "y"}))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := ipc.WriteFrame(h.toPluginW, ipc.Frame{Kind: ipc.KindJSONRPC, ChannelID: 0, Payload: body}); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	if err := h.toPluginW.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("Serve упал на неизвестном ответе: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve не завершился")
	}
}
