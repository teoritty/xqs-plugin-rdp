// Package relay перекачивает байты между браузером и RDP-сервером после того,
// как рукопожатие закончилось.
//
// С этого момента плагин поток НЕ разбирает. Это нормативный инвариант §1
// спеки: всё, что потребует понимания клиентских RDP PDU, — отдельная фаза с
// документированным исключением, а не строчка здесь.
package relay

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
)

// bufSize — размер буфера перекачки.
//
// Меньше потолка фрейма embed-stream (64 KiB) намеренно: так одна порция
// перекачки никогда не превращается в два фрейма, и поведение окна остаётся
// предсказуемым — один Read даёт один Write даёт один кредит.
const bufSize = 32 << 10

// Window — то, что релей умеет делать с кредитным окном.
//
// Интерфейс, а не *transport.RecvWindow: связка проверяется на фейке, где
// видно, когда именно давление приложено, а с настоящим окном это пришлось бы
// выводить по косвенным признакам.
type Window interface {
	Hold()
	Release()
	Held() bool
}

// Coupling связывает два кредитных окна.
//
// Смысл: труба к RDP-серверу регулируется по состоянию трубы к браузеру.
// Браузер перестал успевать → удерживаем гранты на канале к серверу → ядро
// паузит чтение из TCP → сервер притормаживает. Давление доходит до источника
// вместо того, чтобы копиться у нас.
//
// Это ядро дизайна релея, а не оптимизация. Без связки медленный браузер
// упирался бы в 120-секундный потолок подтверждений на стороне ядра — то есть
// пользователь видел бы замерший на две минуты экран вместо плавного
// притормаживания.
type Coupling struct {
	upstream Window // окно канала к серверу: его и удерживаем

	mu   sync.Mutex
	held bool
}

func NewCoupling(upstream Window) *Coupling {
	return &Coupling{upstream: upstream}
}

// Pause удерживает upstream. Идемпотентна: браузер может сообщать о заторе
// многократно, и каждое сообщение не должно добавлять по удержанию — иначе
// снять их все станет нечем.
func (c *Coupling) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.held {
		return
	}
	c.held = true
	c.upstream.Hold()
}

// Resume снимает удержание. Тоже идемпотентна, по той же причине.
func (c *Coupling) Resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.held {
		return
	}
	c.held = false
	c.upstream.Release()
}

// Paused сообщает текущее состояние.
func (c *Coupling) Paused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.held
}

// Pump перекачивает байты в обе стороны, пока одна из сторон не закончится.
//
// Возвращает первую ошибку, которая не является нормальным концом потока.
// Закрытие любой из сторон завершает обе: половина живой сессии бесполезна, а
// оставленная висеть горутина пережила бы саму сессию.
func Pump(browser, server io.ReadWriteCloser) error {
	var wg sync.WaitGroup
	errs := make([]error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		errs[0] = copyStream(server, browser, "browser→server")
		// Закрываем обе стороны: вторая половина без первой не нужна.
		_ = server.Close()
		_ = browser.Close()
	}()
	go func() {
		defer wg.Done()
		errs[1] = copyStream(browser, server, "server→browser")
		_ = browser.Close()
		_ = server.Close()
	}()
	wg.Wait()

	return errors.Join(errs[0], errs[1])
}

// copyStream — io.Copy со своим буфером и без превращения конца потока в
// ошибку.
func copyStream(dst io.Writer, src io.Reader, name string) error {
	buf := make([]byte, bufSize)
	_, err := io.CopyBuffer(dst, src, buf)
	switch {
	case err == nil, errors.Is(err, io.EOF):
		slog.Debug("relay: поток закончился", "direction", name)
		return nil
	case errors.Is(err, io.ErrClosedPipe), errors.Is(err, net.ErrClosed):
		// Встречное направление уже закрыло соединение — это не сбой.
		slog.Debug("relay: поток закрыт встречной стороной", "direction", name)
		return nil
	default:
		return err
	}
}
