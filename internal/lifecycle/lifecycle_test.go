package lifecycle_test

import (
	"encoding/json"
	"testing"
	"time"

	"xqs-plugin-rdp/internal/ipc"
	"xqs-plugin-rdp/internal/lifecycle"
)

// call вызывает обработчик напрямую, без диспетчера: жизненный цикл — это
// машина состояний, и тестировать её через транспорт значит тестировать две
// вещи сразу.
func call(t *testing.T, m *lifecycle.Manager, method, params string) (any, *ipc.Error) {
	t.Helper()
	h := m.HandlerFor(method)
	if h == nil {
		t.Fatalf("метод %q не зарегистрирован", method)
	}
	return h(ipc.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  method,
		Params:  json.RawMessage(params),
	})
}

func mustCall(t *testing.T, m *lifecycle.Manager, method, params string) any {
	t.Helper()
	res, rpcErr := call(t, m, method, params)
	if rpcErr != nil {
		t.Fatalf("%s: %+v", method, rpcErr)
	}
	return res
}

func TestInitializeReportsProtocolVersion(t *testing.T) {
	m := lifecycle.NewManager()
	res := mustCall(t, m, "initialize", `{"pluginApi":"1.0.0"}`)
	got, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("result типа %T, ожидалась карта", res)
	}
	if got["pluginApi"] != "1.0.0" {
		t.Fatalf("pluginApi = %v", got["pluginApi"])
	}
	if m.State() != lifecycle.StateInitialized {
		t.Fatalf("состояние %v, ожидалось StateInitialized", m.State())
	}
}

func TestNewManagerStartsInStateNew(t *testing.T) {
	if got := lifecycle.NewManager().State(); got != lifecycle.StateNew {
		t.Fatalf("состояние %v, ожидалось StateNew", got)
	}
}

// Порядок задаёт ядро. Activate до initialize означает, что мы неверно поняли
// контракт, и молча продолжить — худшее из возможного.
func TestActivateRequiresInitialize(t *testing.T) {
	m := lifecycle.NewManager()
	_, rpcErr := call(t, m, "activate", `{}`)
	if rpcErr == nil {
		t.Fatal("activate до initialize обязан быть ошибкой")
	}
	if rpcErr.Code != ipc.CodeInvalidRequest {
		t.Fatalf("code = %d, ожидался %d", rpcErr.Code, ipc.CodeInvalidRequest)
	}
}

func TestActivateThenDeactivateReturnsToInitialized(t *testing.T) {
	m := lifecycle.NewManager()
	mustCall(t, m, "initialize", `{"pluginApi":"1.0.0"}`)
	mustCall(t, m, "activate", `{}`)
	if m.State() != lifecycle.StateActive {
		t.Fatalf("состояние %v, ожидалось StateActive", m.State())
	}
	mustCall(t, m, "deactivate", `{}`)
	if m.State() != lifecycle.StateInitialized {
		t.Fatalf("состояние %v, ожидалось StateInitialized", m.State())
	}
}

func TestPingAnswersInAnyState(t *testing.T) {
	m := lifecycle.NewManager()
	mustCall(t, m, "ping", `{}`)
	if m.State() != lifecycle.StateNew {
		t.Fatalf("ping изменил состояние на %v", m.State())
	}
	mustCall(t, m, "initialize", `{"pluginApi":"1.0.0"}`)
	mustCall(t, m, "ping", `{}`)
	mustCall(t, m, "shutdown", `{}`)
	mustCall(t, m, "ping", `{}`)
}

func TestShutdownClosesDone(t *testing.T) {
	m := lifecycle.NewManager()
	mustCall(t, m, "initialize", `{"pluginApi":"1.0.0"}`)
	mustCall(t, m, "shutdown", `{}`)
	select {
	case <-m.Done():
	case <-time.After(time.Second):
		t.Fatal("Done() не закрылся: процесс не завершится и ядро убьёт его по таймауту")
	}
	if m.State() != lifecycle.StateStopped {
		t.Fatalf("состояние %v, ожидалось StateStopped", m.State())
	}
}

// Ядро может прислать shutdown повторно (например, на фоне собственного
// таймаута). Повтор обязан быть no-op, а не паникой на закрытом канале.
func TestShutdownIsIdempotent(t *testing.T) {
	m := lifecycle.NewManager()
	mustCall(t, m, "initialize", `{"pluginApi":"1.0.0"}`)
	mustCall(t, m, "shutdown", `{}`)
	mustCall(t, m, "shutdown", `{}`)
}

// Shutdown без initialize — законный путь: ядро вправе свернуть процесс, так и
// не дойдя до инициализации.
func TestShutdownWithoutInitialize(t *testing.T) {
	m := lifecycle.NewManager()
	mustCall(t, m, "shutdown", `{}`)
	select {
	case <-m.Done():
	case <-time.After(time.Second):
		t.Fatal("Done() не закрылся")
	}
}

func TestRegisterWiresAllMethodsIntoDispatcher(t *testing.T) {
	m := lifecycle.NewManager()
	for _, method := range []string{"initialize", "activate", "deactivate", "shutdown", "ping"} {
		if m.HandlerFor(method) == nil {
			t.Fatalf("метод %q не зарегистрирован", method)
		}
	}
	if m.HandlerFor("nope") != nil {
		t.Fatal("HandlerFor вернул обработчик для несуществующего метода")
	}
}
