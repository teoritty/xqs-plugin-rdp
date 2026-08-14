package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"xqs-plugin-rdp/internal/ipc"
	"xqs-plugin-rdp/internal/proxy"
	"xqs-plugin-rdp/internal/relay"
	"xqs-plugin-rdp/internal/transport"
)

// Таймауты вызовов хоста. Разные, потому что разное и делают: channel.open
// ждёт дозвона до цели, остальные — только записи в очередь.
const (
	rpcTimeout         = 5 * time.Second
	channelOpenTimeout = 10 * time.Second
)

// tunnelID — единственный туннель, который мы регистрируем.
//
// Не шесть, как советует plugin-api.md для RDP: виртуальные каналы RDP
// мультиплексируются внутри самого протокола, и наружу это одно соединение.
const tunnelID = "main"

// Host — то, что сессии нужно от шины.
type Host interface {
	Call(ctx context.Context, method string, params, result any) error
	Notify(method string, params any) error
}

// Channels — то, что сессии нужно от транспорта.
type Channels interface {
	Open(ctx context.Context, purpose transport.Purpose, sessionID, hint string) (*transport.Conn, error)
	CloseAll()
}

// Manager ведёт одну сессию: при isolation per-session процесс обслуживает
// ровно её.
type Manager struct {
	host     Host
	channels Channels

	mu       sync.Mutex
	active   *active
	coupling *relay.Coupling
}

type active struct {
	id     string
	cancel context.CancelFunc
	done   chan struct{}
}

func NewManager(host Host, channels Channels) *Manager {
	return &Manager{host: host, channels: channels}
}

// Register вешает обработчики сессии на диспетчер.
func (m *Manager) Register(d *ipc.Dispatcher) {
	d.Handle("session.connect", m.onConnect)
	d.Handle("session.disconnect", m.onDisconnect)
	d.Handle("session.tunnelBackpressure", m.onBackpressure)
	d.Handle("session.tunnelResume", m.onResume)
	d.Handle("session.embedActivity", m.onEmbedActivity)
	d.Handle("session.embedViewport", m.onEmbedViewport)
}

// onConnect отвечает НЕМЕДЛЕННО и всю работу уносит в горутину.
//
// Это не оптимизация: session.connect — синхронный RPC с таймаутом 5 секунд, а
// дозвон, TLS и три сообщения NTLM его легко перекрывают. Ответить после работы
// значит гарантированно не ответить вовремя.
func (m *Manager) onConnect(req ipc.Request) (any, *ipc.Error) {
	parsed, err := parseConnect(req.Params)
	if err != nil {
		return nil, &ipc.Error{Code: ipc.CodeInvalidParams, Message: err.Error()}
	}

	ctx, cancel := context.WithCancel(context.Background())
	a := &active{id: parsed.sessionID, cancel: cancel, done: make(chan struct{})}

	m.mu.Lock()
	if m.active != nil {
		// Crash-recovery присылает session.connect повторно. Старое состояние
		// не переиспользуется: токен отозван, каналы закрыты, и притворяться,
		// что что-то из этого ещё живо, — прямой путь к сессии-призраку.
		m.mu.Unlock()
		m.teardown()
		m.mu.Lock()
	}
	m.active = a
	m.mu.Unlock()

	go m.run(ctx, a, parsed)
	return map[string]bool{"accepted": true}, nil
}

// run проводит подключение целиком. Всё, что здесь возвращает ошибку, уходит
// пользователю через updateState — иначе вкладка останется в connecting
// навсегда.
func (m *Manager) run(ctx context.Context, a *active, parsed request) {
	defer close(a.done)
	// Пароль не переживает эту функцию ни на одном пути выхода.
	defer zeroCredentials(&parsed.target.Credentials)
	defer func() {
		if rec := recover(); rec != nil {
			// Паника в горутине не видна ядру как ошибка сессии: без этого
			// вкладка зависла бы в connecting навсегда.
			slog.Error("session: паника при подключении", "panic", fmt.Sprint(rec))
			m.fail(a.id, errors.New("внутренняя ошибка плагина"))
		}
	}()

	if err := m.connect(ctx, a, parsed); err != nil {
		if ctx.Err() != nil {
			// Сессию закрыл пользователь — это не сбой.
			slog.Debug("session: подключение отменено", "sessionId", a.id)
			return
		}
		slog.Error("session: подключение не удалось", "sessionId", a.id, "err", err.Error())
		m.fail(a.id, err)
		return
	}
}

func (m *Manager) connect(ctx context.Context, a *active, parsed request) error {
	// 1. Регистрация embed. Обязательна и первична: токен минтится здесь, и
	// при crash-recovery он новый — старый отозван.
	if err := m.registerEmbed(ctx, a.id); err != nil {
		return err
	}

	// 2. Канал к браузеру. hint — идентификатор туннеля.
	browser, err := m.openChannel(ctx, transport.PurposeEmbedStream, a.id, tunnelID)
	if err != nil {
		return err
	}
	// 3. Канал к серверу. hint — «хост:порт»; дозванивается хост, не мы.
	server, err := m.openChannel(ctx, transport.PurposeTCPRelay, a.id, parsed.target.Address)
	if err != nil {
		return err
	}

	// 4. Рукопожатие. Здесь и только здесь живёт пароль.
	serverTLS, err := proxy.Handshake(ctx, browser, server, parsed.target)
	if err != nil {
		// Браузеру надо сказать, что не вышло: иначе он ждёт ответа, которого
		// не будет, и пользователь видит пустой экран вместо причины.
		proxy.ReportError(browser)
		return err
	}

	// 5. Готово. С этого момента поток не разбирается.
	if err := m.updateState(ctx, a.id, "ready", ""); err != nil {
		return err
	}

	m.mu.Lock()
	m.coupling = relay.NewCoupling(server.RecvWindow())
	m.mu.Unlock()

	if err := relay.Pump(browser, serverTLS); err != nil {
		return err
	}
	// Поток закончился штатно: сервер закрыл соединение или пользователь ушёл.
	m.disconnected(a.id)
	return nil
}

type registerEmbedParams struct {
	SessionID string   `json:"sessionId"`
	UIEntry   string   `json:"uiEntry"`
	TunnelIDs []string `json:"tunnelIds"`
}

func (m *Manager) registerEmbed(ctx context.Context, sessionID string) error {
	// Результат (токен и URL) нам не нужен: хост сам отдаёт их фронтенду, а
	// страница выводит адрес туннеля из собственного location.
	err := m.call(ctx, rpcTimeout, "session.registerEmbed", registerEmbedParams{
		SessionID: sessionID,
		UIEntry:   "ui/rdp.html",
		TunnelIDs: []string{tunnelID},
	})
	if err != nil {
		return fmt.Errorf("session: registerEmbed: %w", err)
	}
	return nil
}

type updateStateParams struct {
	SessionID string `json:"sessionId"`
	State     string `json:"state"`
	Error     string `json:"error,omitempty"`
}

func (m *Manager) updateState(ctx context.Context, sessionID, state, errText string) error {
	err := m.call(ctx, rpcTimeout, "session.updateState", updateStateParams{
		SessionID: sessionID,
		State:     state,
		Error:     errText,
	})
	if err != nil {
		return fmt.Errorf("session: updateState(%s): %w", state, err)
	}
	return nil
}

// fail сообщает пользователю причину. Контекст здесь свой: тот, что был у
// подключения, уже отменён, а сказать всё равно надо.
func (m *Manager) fail(sessionID string, cause error) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	if err := m.updateState(ctx, sessionID, "error", cause.Error()); err != nil {
		slog.Error("session: не удалось сообщить об ошибке", "err", err.Error())
	}
}

func (m *Manager) disconnected(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	if err := m.updateState(ctx, sessionID, "disconnected", ""); err != nil {
		slog.Debug("session: не удалось сообщить о завершении", "err", err.Error())
	}
}

func (m *Manager) onDisconnect(ipc.Request) (any, *ipc.Error) {
	m.teardown()
	return nil, nil
}

// teardown сворачивает сессию. Идемпотентен: ядро закрывает каналы само, и
// прийти сюда дважды — обычное дело.
func (m *Manager) teardown() {
	m.mu.Lock()
	a := m.active
	m.active = nil
	m.coupling = nil
	m.mu.Unlock()

	if a == nil {
		return
	}
	a.cancel()
	m.channels.CloseAll()
	select {
	case <-a.done:
	case <-time.After(2 * time.Second):
		slog.Warn("session: подключение не завершилось вовремя", "sessionId", a.id)
	}
}

// Backpressure от браузера приводит в действие связку окон: перестаём выдавать
// кредиты каналу к серверу, и давление доходит до самого сервера.
func (m *Manager) onBackpressure(ipc.Request) (any, *ipc.Error) {
	m.mu.Lock()
	c := m.coupling
	m.mu.Unlock()
	if c != nil {
		c.Pause()
	}
	return nil, nil
}

func (m *Manager) onResume(ipc.Request) (any, *ipc.Error) {
	m.mu.Lock()
	c := m.coupling
	m.mu.Unlock()
	if c != nil {
		c.Resume()
	}
	return nil, nil
}

// Уход фокуса НЕ рвёт соединение — это прямое требование документации ядра.
// Отрисовку приостанавливает сама страница по embed.suspend от хоста.
func (m *Manager) onEmbedActivity(ipc.Request) (any, *ipc.Error) { return nil, nil }

// Размер вьюпорта страница получает от хоста напрямую через postMessage и сама
// вызывает resize у клиента. Дублировать это здесь нечем: плагин после точки
// отсечения поток не разбирает.
func (m *Manager) onEmbedViewport(ipc.Request) (any, *ipc.Error) { return nil, nil }

// Done позволяет дождаться завершения сессии. Нужен композиционному корню.
func (m *Manager) Done() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == nil {
		return nil
	}
	return m.active.done
}

// call — синхронный вызов хоста с собственным сроком.
//
// Обёртка существует ради defer cancel: раздавать контексты со сроком без
// отмены значит копить таймеры на каждую сессию, а забыть cancel в одном из
// пяти мест вызова — вопрос времени.
func (m *Manager) openChannel(parent context.Context, purpose transport.Purpose, sessionID, hint string) (*transport.Conn, error) {
	ctx, cancel := context.WithTimeout(parent, channelOpenTimeout)
	defer cancel()
	return m.channels.Open(ctx, purpose, sessionID, hint)
}

func (m *Manager) call(parent context.Context, d time.Duration, method string, params any) error {
	ctx, cancel := context.WithTimeout(parent, d)
	defer cancel()
	return m.host.Call(ctx, method, params, nil)
}
