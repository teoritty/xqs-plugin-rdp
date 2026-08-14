// Package transport поднимает над бинарным каналом ядра то, что нужно
// остальному плагину: кредитные окна и полноценный net.Conn.
//
// Полноценный — не для красоты: поверх этого соединения работает crypto/tls,
// а tls.Client принимает net.Conn целиком, вместе с дедлайнами и адресами.
package transport

import (
	"context"
	"errors"
	"sync"
)

// ErrClosed — окно закрыто; ожидающие разбужены, новые операции не проходят.
var ErrClosed = errors.New("transport: канал закрыт")

// SendWindow — исходящая половина кредитного окна.
//
// Смысл один: не отправить больше, чем разрешил получатель. Превышение — не
// «немного лишнего трафика», а протокольное нарушение, за которое ядро
// закрывает stdio и убивает процесс. Поэтому на нуле мы ждём, а не проскакиваем.
type SendWindow struct {
	mu     sync.Mutex
	cond   *sync.Cond
	credit int
	closed bool
}

func NewSendWindow(initial int) *SendWindow {
	w := &SendWindow{credit: initial}
	w.cond = sync.NewCond(&w.mu)
	return w
}

// TryAcquire забирает кредит, если он есть прямо сейчас, и никогда не ждёт.
//
// Существует ради горячего пути: при работающей сессии кредит обычно есть, а
// полный Acquire ради этого поднимает сторожевую горутину и канал — на каждый
// кадр. При всплеске кадров это тысячи горутин в секунду, то есть работа
// сборщика ровно в тот момент, когда её меньше всего ждут.
func (w *SendWindow) TryAcquire() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.credit <= 0 {
		return false
	}
	w.credit--
	return true
}

// Acquire забирает один кредит, ожидая при необходимости.
//
// Ожидание прерывается отменой контекста и закрытием окна. Первое важно не
// меньше второго: без него закрытие сессии повисало бы на отправителе, который
// ждёт кредита, а выдать его уже некому.
func (w *SendWindow) Acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.TryAcquire() {
		return nil
	}

	// sync.Cond не умеет просыпаться по контексту, поэтому будим его сами.
	// Сторож живёт ровно столько, сколько длится ожидание.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			w.cond.Broadcast()
		case <-done:
		}
	}()

	w.mu.Lock()
	defer w.mu.Unlock()
	for {
		if w.closed {
			return ErrClosed
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if w.credit > 0 {
			w.credit--
			return nil
		}
		w.cond.Wait()
	}
}

// Grant добавляет кредит, выданный получателем.
func (w *SendWindow) Grant(n int) {
	if n <= 0 {
		return
	}
	w.mu.Lock()
	w.credit += n
	w.mu.Unlock()
	w.cond.Broadcast()
}

func (w *SendWindow) Available() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.credit
}

func (w *SendWindow) Close() {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	w.cond.Broadcast()
}

// GrantFunc отправляет кредитное обновление другой стороне.
type GrantFunc func(n int)

// RecvWindow — приёмная половина, и она интереснее исходящей.
//
// Исходящая просто ведёт учёт. Приёмная — это ПОЛИТИКА: удержание грантов
// является единственным рычагом давления на источник. Перестать выдавать
// кредиты по каналу к RDP-серверу значит заставить ядро остановить чтение из
// TCP, а сервер — притормозить. Связка окон (relay/coupling) строится ровно на
// этом, поэтому Hold и Release здесь такие же полноправные операции, как
// Consumed.
type RecvWindow struct {
	mu       sync.Mutex
	pending  int // потреблено, но ещё не подтверждено выдачей гранта
	holds    int
	grant    GrantFunc
	capacity int
}

func NewRecvWindow(capacity int, grant GrantFunc) *RecvWindow {
	return &RecvWindow{capacity: capacity, grant: grant}
}

// Consumed отмечает, что n фреймов вычитаны и их место в окне освободилось.
//
// Грант уходит немедленно, если окно не удержано. Копить их «для эффективности»
// нельзя: невыданный кредит — это остановленный источник, и задержка здесь
// оборачивается не экономией трафика, а замершей картинкой.
func (r *RecvWindow) Consumed(n int) {
	if n <= 0 {
		return
	}
	r.mu.Lock()
	r.pending += n
	send := 0
	if r.holds == 0 {
		send, r.pending = r.pending, 0
	}
	grant := r.grant
	r.mu.Unlock()

	if send > 0 && grant != nil {
		grant(send)
	}
}

// Hold удерживает гранты. Вложенный: связка вправе давить на окно из
// нескольких мест, и одно освобождение не должно отпускать давление целиком.
func (r *RecvWindow) Hold() {
	r.mu.Lock()
	r.holds++
	r.mu.Unlock()
}

// Release снимает одно удержание и, если оно было последним, отдаёт всё
// накопленное.
//
// Лишний Release — дефект вызывающего, но он не должен уводить счётчик в
// минус: тогда следующий Hold не сработал бы, и окно перестало бы удерживаться
// вообще. Отказ тише самой ошибки хуже ошибки.
func (r *RecvWindow) Release() {
	r.mu.Lock()
	if r.holds > 0 {
		r.holds--
	}
	send := 0
	if r.holds == 0 {
		send, r.pending = r.pending, 0
	}
	grant := r.grant
	r.mu.Unlock()

	if send > 0 && grant != nil {
		grant(send)
	}
}

// Held сообщает, удерживается ли окно. Нужно связке и диагностике.
func (r *RecvWindow) Held() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.holds > 0
}

// Capacity — размер окна, выданный при открытии канала.
func (r *RecvWindow) Capacity() int { return r.capacity }
