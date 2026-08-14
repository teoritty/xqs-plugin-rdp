package ipc_test

import (
	"encoding/json"
	"testing"

	"xqs-plugin-rdp/internal/ipc"
)

func TestRequestWithoutIDIsNotification(t *testing.T) {
	var req ipc.Request
	if err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","method":"session.disconnect"}`), &req); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !req.IsNotification() {
		t.Fatal("запрос без id — нотификация, на неё не отвечают")
	}
	if req.Method != "session.disconnect" {
		t.Fatalf("Method = %q", req.Method)
	}
}

func TestRequestWithIDIsNotNotification(t *testing.T) {
	var req ipc.Request
	if err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","id":7,"method":"ping"}`), &req); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if req.IsNotification() {
		t.Fatal("запрос с id требует ответа")
	}
}

// id обязан сохраняться байт в байт: JSON-RPC разрешает и число, и строку, а
// подмена типа ломает сопоставление ответа с запросом на стороне ядра.
func TestResponsePreservesIDVerbatim(t *testing.T) {
	for _, raw := range []string{`7`, `"abc-1"`, `0`, `"0"`} {
		out, err := json.Marshal(ipc.NewResult(json.RawMessage(raw), map[string]bool{"accepted": true}))
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var back struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(out, &back); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if back.JSONRPC != "2.0" {
			t.Fatalf("jsonrpc = %q", back.JSONRPC)
		}
		if string(back.ID) != raw {
			t.Fatalf("id = %s, ожидалось %s", back.ID, raw)
		}
	}
}

func TestErrorResponseHasNoResultKey(t *testing.T) {
	out, err := json.Marshal(ipc.NewError(json.RawMessage(`1`), ipc.CodeMethodNotFound, "no such method"))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := m["result"]; ok {
		t.Fatal("ответ с error не должен нести result")
	}
	if _, ok := m["error"]; !ok {
		t.Fatal("ответ обязан нести error")
	}
}

func TestResultResponseHasNoErrorKey(t *testing.T) {
	out, err := json.Marshal(ipc.NewResult(json.RawMessage(`1`), map[string]bool{"accepted": true}))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := m["error"]; ok {
		t.Fatal("успешный ответ не должен нести error")
	}
}

// Результат nil — законный ответ (например, у session.updateState). Он обязан
// остаться ответом-успехом, а не превратиться в пустой объект без result.
func TestNilResultStillProducesResultKey(t *testing.T) {
	out, err := json.Marshal(ipc.NewResult(json.RawMessage(`1`), nil))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := m["error"]; ok {
		t.Fatal("успешный ответ не должен нести error")
	}
	if got, ok := m["result"]; !ok || string(got) != "null" {
		t.Fatalf("result = %s (present=%v), ожидалось null", got, ok)
	}
}

func TestErrorCodesMatchJSONRPCSpec(t *testing.T) {
	pairs := map[int]int{
		ipc.CodeParse:          -32700,
		ipc.CodeInvalidRequest: -32600,
		ipc.CodeMethodNotFound: -32601,
		ipc.CodeInvalidParams:  -32602,
		ipc.CodeInternal:       -32603,
	}
	for got, want := range pairs {
		if got != want {
			t.Fatalf("код %d, ожидался %d", got, want)
		}
	}
}
