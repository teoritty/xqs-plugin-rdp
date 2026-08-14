package transport

import (
	"sync"
	"time"
)

// deadline — истекающий срок, по которому просыпается заблокированная операция.
//
// У канальной трубы нет сокета, у которого можно попросить таймаут, поэтому
// дедлайны приходится строить самим. Это не заглушка ради удовлетворения
// интерфейса: crypto/tls ставит дедлайны всерьёз, и «дедлайн, который ничего не
// делает» превратился бы в зависшее навсегда TLS-рукопожатие.
//
// Устройство — как у pipeDeadline в стандартной библиотеке: канал, который
// закрывается по истечении срока, плюс таймер, который его закрывает.
// Ожидающий делает select по нему и по своим данным.
type deadline struct {
	mu     sync.Mutex
	timer  *time.Timer
	cancel chan struct{} // закрывается, когда срок истёк
}

func newDeadline() *deadline {
	return &deadline{cancel: make(chan struct{})}
}

// set устанавливает срок. Нулевое время снимает дедлайн.
//
// Семантика net.Conn требует, чтобы уже истёкший дедлайн продолжал заваливать
// операции, пока его не переустановят: «отвиснуть и поехать дальше» после
// таймаута — это не то, чего ждёт вызывающий, который таймаут поставил.
func (d *deadline) set(t time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	// Переустановка воскрешает канал: старый мог быть уже закрыт.
	select {
	case <-d.cancel:
		d.cancel = make(chan struct{})
	default:
	}

	if t.IsZero() {
		return
	}
	if dur := time.Until(t); dur > 0 {
		cancel := d.cancel
		d.timer = time.AfterFunc(dur, func() {
			d.mu.Lock()
			defer d.mu.Unlock()
			select {
			case <-cancel:
			default:
				close(cancel)
			}
		})
		return
	}
	// Срок в прошлом — операции обязаны падать немедленно, а не «почти сразу».
	close(d.cancel)
}

// wait возвращает канал, который закроется по истечении срока.
func (d *deadline) wait() <-chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cancel
}

// expired сообщает, истёк ли срок прямо сейчас.
func (d *deadline) expired() bool {
	select {
	case <-d.wait():
		return true
	default:
		return false
	}
}

func (d *deadline) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
