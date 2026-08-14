// Package lifecycle держит машину состояний плагина: то, что ядро вызывает до
// и после всякой полезной работы.
package lifecycle

import (
	"sync"

	"xqs-plugin-rdp/internal/ipc"
)

// pluginAPIVersion — версия Plugin API, под которую написан плагин. Совпадает с
// requires.pluginApi в манифесте; расхождение между ними — дефект сборки.
const pluginAPIVersion = "1.0.0"

// State — состояние процесса плагина.
type State int

const (
	StateNew State = iota
	StateInitialized
	StateActive
	StateStopped
)

func (s State) String() string {
	switch s {
	case StateNew:
		return "new"
	case StateInitialized:
		return "initialized"
	case StateActive:
		return "active"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Manager обслуживает методы жизненного цикла и хранит текущее состояние.
//
// Он ничего не знает ни про RDP, ни про каналы: его единственная забота —
// порядок, в котором ядро нас поднимает и опускает.
type Manager struct {
	mu       sync.Mutex
	state    State
	done     chan struct{}
	doneOnce sync.Once
	handlers map[string]ipc.Handler
}

func NewManager() *Manager {
	m := &Manager{
		state: StateNew,
		done:  make(chan struct{}),
	}
	m.handlers = map[string]ipc.Handler{
		"initialize": m.initialize,
		"activate":   m.activate,
		"deactivate": m.deactivate,
		"shutdown":   m.shutdown,
		"ping":       m.ping,
	}
	return m
}

// Register вешает методы жизненного цикла на диспетчер.
func (m *Manager) Register(d *ipc.Dispatcher) {
	for method, h := range m.handlers {
		d.Handle(method, h)
	}
}

// HandlerFor отдаёт обработчик метода напрямую. Нужен композиционному корню и
// тестам, которым диспетчер только мешает.
func (m *Manager) HandlerFor(method string) ipc.Handler { return m.handlers[method] }

func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Done закрывается ровно один раз, когда ядро попросило нас завершиться.
func (m *Manager) Done() <-chan struct{} { return m.done }

func (m *Manager) initialize(ipc.Request) (any, *ipc.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = StateInitialized
	return map[string]any{"pluginApi": pluginAPIVersion}, nil
}

func (m *Manager) activate(ipc.Request) (any, *ipc.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Порядок задаёт ядро. Activate вне StateInitialized означает, что мы
	// неверно поняли контракт, и молча продолжить — худшее из возможного:
	// дальше поедет сессия поверх непроинициализированного процесса.
	if m.state != StateInitialized {
		return nil, &ipc.Error{
			Code:    ipc.CodeInvalidRequest,
			Message: "activate in state " + m.state.String() + ", expected initialized",
		}
	}
	m.state = StateActive
	return nil, nil
}

func (m *Manager) deactivate(ipc.Request) (any, *ipc.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateActive {
		m.state = StateInitialized
	}
	return nil, nil
}

// shutdown идемпотентен: ядро вправе прислать его повторно, в том числе на фоне
// собственного таймаута, и второй вызов обязан быть no-op, а не паникой на
// закрытом канале. Он также законен без initialize — процесс могут свернуть,
// так и не дойдя до инициализации.
func (m *Manager) shutdown(ipc.Request) (any, *ipc.Error) {
	m.mu.Lock()
	m.state = StateStopped
	m.mu.Unlock()
	m.doneOnce.Do(func() { close(m.done) })
	return nil, nil
}

// ping отвечает в любом состоянии и ничего не меняет: это проверка живости
// процесса, а не часть протокола запуска.
func (m *Manager) ping(ipc.Request) (any, *ipc.Error) {
	return map[string]any{"ok": true}, nil
}

// APIVersion сообщает версию Plugin API, под которую написан плагин. Она же
// стоит в requires.pluginApi манифеста, и тест композиционного корня следит,
// чтобы эти два места не разъехались.
func APIVersion() string { return pluginAPIVersion }
