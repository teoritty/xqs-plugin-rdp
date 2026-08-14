package session

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"xqs-plugin-rdp/internal/ipc"
	"xqs-plugin-rdp/internal/transport"
)

// fakeHost записывает вызовы к хосту: по ним видно и порядок шагов, и то, что
// пользователю в итоге сказали.
type fakeHost struct {
	mu     sync.Mutex
	calls  []call
	failOn map[string]error
	// results — ответы хоста по методам. Нужны там, где ответ несёт решение:
	// у trust.verifyPeer от него зависит, пойдёт ли дальше пароль.
	results map[string]any
}

type call struct {
	method string
	params any
}

func newFakeHost() *fakeHost {
	return &fakeHost{failOn: map[string]error{}, results: map[string]any{}}
}

func (h *fakeHost) Call(_ context.Context, method string, params, result any) error {
	h.mu.Lock()
	h.calls = append(h.calls, call{method: method, params: params})
	err := h.failOn[method]
	res, hasRes := h.results[method]
	h.mu.Unlock()
	if err != nil {
		return err
	}
	if !hasRes || result == nil {
		return nil
	}
	// Через JSON, а не присваиванием: так же, как это делает настоящая шина,
	// и тег поля проверяется заодно.
	raw, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, result)
}

func (h *fakeHost) Notify(string, any) error { return nil }

// callsTo отбирает вызовы одного метода — вместе с параметрами, потому что у
// доверия проверять надо именно их.
func (h *fakeHost) callsTo(method string) []call {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []call
	for _, c := range h.calls {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

func (h *fakeHost) methods() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.calls))
	for _, c := range h.calls {
		out = append(out, c.method)
	}
	return out
}

// lastState возвращает последнее состояние, сообщённое пользователю.
func (h *fakeHost) lastState(t *testing.T) updateStateParams {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.calls) - 1; i >= 0; i-- {
		if h.calls[i].method == "session.updateState" {
			p, ok := h.calls[i].params.(updateStateParams)
			if !ok {
				t.Fatalf("params типа %T", h.calls[i].params)
			}
			return p
		}
	}
	t.Fatal("session.updateState не вызывался — вкладка осталась бы в connecting навсегда")
	return updateStateParams{}
}

type fakeChannels struct {
	mu       sync.Mutex
	opened   []transport.Purpose
	hints    []string
	err      error
	closeAll int
}

func (c *fakeChannels) Open(_ context.Context, purpose transport.Purpose, _, hint string) (*transport.Conn, error) {
	c.mu.Lock()
	c.opened = append(c.opened, purpose)
	c.hints = append(c.hints, hint)
	err := c.err
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	// Реального канала тесты не требуют: до работы с ним ни один из них не
	// доходит, а возвращать полуживой Conn значило бы проверять его, а не
	// сессию.
	return nil, errors.New("канал недоступен в тесте")
}

func (c *fakeChannels) CloseAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeAll++
}

func (c *fakeChannels) snapshot() ([]transport.Purpose, []string, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]transport.Purpose(nil), c.opened...), append([]string(nil), c.hints...), c.closeAll
}

func connectRequest(t *testing.T, params map[string]any) ipc.Request {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return ipc.Request{Method: "session.connect", Params: raw}
}

func validParams() map[string]any {
	return map[string]any{
		"sessionId": "s-1",
		"protocol":  "rdp",
		"host":      "192.168.31.38",
		"port":      3389,
		"fields": map[string]string{
			"username": "admin",
			"password": "hunter2",
			"domain":   "CORP",
		},
	}
}

// session.connect — синхронный RPC с таймаутом 5 секунд, а дозвон, TLS и три
// сообщения NTLM его перекрывают. Ответ обязан вернуться немедленно.
func TestConnectAcceptsImmediately(t *testing.T) {
	m := NewManager(newFakeHost(), &fakeChannels{})

	start := time.Now()
	res, rpcErr := m.onConnect(connectRequest(t, validParams()))
	elapsed := time.Since(start)

	if rpcErr != nil {
		t.Fatalf("onConnect: %+v", rpcErr)
	}
	accepted, ok := res.(map[string]bool)
	if !ok || !accepted["accepted"] {
		t.Fatalf("result = %v, ожидалось {accepted:true}", res)
	}
	if elapsed > time.Second {
		t.Fatalf("onConnect занял %v — при таймауте RPC в 5 с работа обязана уходить в горутину", elapsed)
	}
	m.teardown()
}

func TestConnectRejectsBadParams(t *testing.T) {
	cases := map[string]map[string]any{
		"нет sessionId": {"host": "h", "fields": map[string]string{"username": "u"}},
		"нет host":      {"sessionId": "s", "fields": map[string]string{"username": "u"}},
		"нет логина":    {"sessionId": "s", "host": "h", "fields": map[string]string{}},
		"дикий порт":    {"sessionId": "s", "host": "h", "port": 70000, "fields": map[string]string{"username": "u"}},
	}
	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			m := NewManager(newFakeHost(), &fakeChannels{})
			_, rpcErr := m.onConnect(connectRequest(t, params))
			if rpcErr == nil {
				t.Fatal("неверные параметры приняты")
			}
			if rpcErr.Code != ipc.CodeInvalidParams {
				t.Fatalf("код %d, ожидался %d", rpcErr.Code, ipc.CodeInvalidParams)
			}
		})
	}
}

// Порт 0 — законный случай: хост подставляет defaultPort из манифеста. В
// дозвоне ноль выглядел бы как «любой порт», поэтому подставляем сами.
func TestConnectFillsDefaultPort(t *testing.T) {
	params := validParams()
	params["port"] = 0
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parsed, err := parseConnect(raw)
	if err != nil {
		t.Fatalf("parseConnect: %v", err)
	}
	if parsed.target.Address != "192.168.31.38:3389" {
		t.Fatalf("Address = %q", parsed.target.Address)
	}
}

// Адрес берётся из fields/host, а НЕ из того, что пришлёт браузер. Проверяется
// здесь, потому что это граница доверия, а не деталь разбора.
func TestConnectBuildsTargetFromHostFields(t *testing.T) {
	raw, err := json.Marshal(validParams())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parsed, err := parseConnect(raw)
	if err != nil {
		t.Fatalf("parseConnect: %v", err)
	}
	if parsed.sessionID != "s-1" {
		t.Fatalf("sessionID = %q", parsed.sessionID)
	}
	if parsed.target.Address != "192.168.31.38:3389" {
		t.Fatalf("Address = %q", parsed.target.Address)
	}
	// Разбор параметров доверие не назначает: его подключает жизненный цикл,
	// у которого есть шина к ядру. Цель, собранная здесь, обязана быть
	// непригодной к подключению до этого момента.
	if parsed.target.VerifyPeer != nil {
		t.Fatal("разбор параметров назначил проверку доверия сам")
	}
	if parsed.target.Credentials.Username != "admin" || parsed.target.Credentials.Domain != "CORP" {
		t.Fatalf("Credentials = %+v", parsed.target.Credentials)
	}
	if string(parsed.target.Credentials.Password) != "hunter2" {
		t.Fatalf("пароль не извлечён")
	}
}

// Пароль обязан зануляться на КАЖДОМ пути выхода. Секрет, переживший неудачное
// подключение, ничем не лучше секрета, пережившего удачное.
func TestPasswordIsZeroedOnEveryExit(t *testing.T) {
	raw, err := json.Marshal(validParams())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parsed, err := parseConnect(raw)
	if err != nil {
		t.Fatalf("parseConnect: %v", err)
	}
	pw := parsed.target.Credentials.Password
	if len(pw) == 0 {
		t.Fatal("пароль пуст — тест не проверил бы ничего")
	}
	// Держим ссылку на исходный массив, чтобы увидеть именно зануление, а не
	// подмену среза на nil.
	backing := pw[:cap(pw)]

	zeroCredentials(&parsed.target.Credentials)

	if parsed.target.Credentials.Password != nil {
		t.Fatal("ссылка на пароль не сброшена")
	}
	for i, b := range backing {
		if b != 0 {
			t.Fatalf("байт %d пароля не занулён: %#x", i, b)
		}
	}
}

// Ошибка на любом шаге обязана дойти до пользователя как состояние error:
// иначе вкладка навсегда останется в connecting.
func TestFailureReachesTheUser(t *testing.T) {
	h := newFakeHost()
	h.failOn["session.registerEmbed"] = errors.New("token minting failed")
	m := NewManager(h, &fakeChannels{})

	if _, rpcErr := m.onConnect(connectRequest(t, validParams())); rpcErr != nil {
		t.Fatalf("onConnect: %+v", rpcErr)
	}
	waitDone(t, m)

	got := h.lastState(t)
	if got.State != "error" {
		t.Fatalf("состояние %q, ожидалось error", got.State)
	}
	if got.Error == "" {
		t.Fatal("причина не сообщена — пользователь увидит «ошибка» без объяснения")
	}
	if got.SessionID != "s-1" {
		t.Fatalf("sessionId = %q", got.SessionID)
	}
}

// registerEmbed обязателен и первичен: токен минтится там, и открывать каналы
// до него бессмысленно.
func TestRegisterEmbedHappensBeforeChannels(t *testing.T) {
	h := newFakeHost()
	ch := &fakeChannels{}
	m := NewManager(h, ch)

	if _, rpcErr := m.onConnect(connectRequest(t, validParams())); rpcErr != nil {
		t.Fatalf("onConnect: %+v", rpcErr)
	}
	waitDone(t, m)

	methods := h.methods()
	if len(methods) == 0 || methods[0] != "session.registerEmbed" {
		t.Fatalf("первый вызов %v, ожидался session.registerEmbed", methods)
	}
	purposes, hints, _ := ch.snapshot()
	if len(purposes) == 0 || purposes[0] != transport.PurposeEmbedStream {
		t.Fatalf("первым открыт канал %v, ожидался embed-stream", purposes)
	}
	if hints[0] != tunnelID {
		t.Fatalf("hint для embed-stream = %q, ожидался %q", hints[0], tunnelID)
	}
}

// Снос идемпотентен: ядро закрывает каналы само, и прийти сюда дважды — норма,
// а не повод для паники на закрытом канале.
func TestTeardownIsIdempotent(t *testing.T) {
	m := NewManager(newFakeHost(), &fakeChannels{})
	if _, rpcErr := m.onConnect(connectRequest(t, validParams())); rpcErr != nil {
		t.Fatalf("onConnect: %+v", rpcErr)
	}
	waitDone(t, m)

	m.teardown()
	m.teardown()
	if _, rpcErr := m.onDisconnect(ipc.Request{}); rpcErr != nil {
		t.Fatalf("onDisconnect: %+v", rpcErr)
	}
}

// Backpressure до готовности сессии не должен падать: хост вправе прислать его
// когда угодно, в том числе когда связки ещё нет.
func TestBackpressureBeforeReadyIsHarmless(t *testing.T) {
	m := NewManager(newFakeHost(), &fakeChannels{})
	if _, rpcErr := m.onBackpressure(ipc.Request{}); rpcErr != nil {
		t.Fatalf("onBackpressure: %+v", rpcErr)
	}
	if _, rpcErr := m.onResume(ipc.Request{}); rpcErr != nil {
		t.Fatalf("onResume: %+v", rpcErr)
	}
}

// Уход фокуса НЕ рвёт сессию — прямое требование документации ядра.
func TestEmbedActivityDoesNotTearDown(t *testing.T) {
	m := NewManager(newFakeHost(), &fakeChannels{})
	if _, rpcErr := m.onConnect(connectRequest(t, validParams())); rpcErr != nil {
		t.Fatalf("onConnect: %+v", rpcErr)
	}
	if _, rpcErr := m.onEmbedActivity(ipc.Request{}); rpcErr != nil {
		t.Fatalf("onEmbedActivity: %+v", rpcErr)
	}
	if _, rpcErr := m.onEmbedViewport(ipc.Request{}); rpcErr != nil {
		t.Fatalf("onEmbedViewport: %+v", rpcErr)
	}
	if m.Done() == nil {
		t.Fatal("сессия свёрнута из-за ухода фокуса")
	}
	m.teardown()
}

// Регистрация обработчиков — это контракт с ядром: пропущенный метод означает
// -32601 на сообщение, которое хост считает обязательным.
func TestRegisterCoversTheWholeSessionContract(t *testing.T) {
	d := ipc.NewDispatcher(nil)
	NewManager(newFakeHost(), &fakeChannels{}).Register(d)
	for _, method := range []string{
		"session.connect",
		"session.disconnect",
		"session.tunnelBackpressure",
		"session.tunnelResume",
		"session.embedActivity",
		"session.embedViewport",
	} {
		if !d.Handles(method) {
			t.Fatalf("метод %q не зарегистрирован", method)
		}
	}
}

func waitDone(t *testing.T, m *Manager) {
	t.Helper()
	done := m.Done()
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("подключение не завершилось")
	}
}
