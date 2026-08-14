package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// FrameSender отправляет один фрейм данных канала.
//
// Порт, а не прямая зависимость от ipc: он позволяет проверять Conn на фейке,
// без шины и без хоста, — а поверх Conn работает crypto/tls, которому нужен
// именно настоящий net.Conn, а не заглушка.
//
// КОНТРАКТ: payload действителен только на время вызова. Отправитель обязан
// записать его синхронно либо скопировать сам. Это записано здесь, потому что
// на другой стороне контракта — горячий путь: копия на каждый кадр стоила
// 32 КБ мусора на кадр и заметной просадки при всплеске.
type FrameSender interface {
	SendChannelFrame(channelID uint32, payload []byte) error
}

// Conn — полноценный net.Conn поверх бинарного канала ядра.
//
// Полный интерфейс взят не для галочки: tls.Client принимает net.Conn целиком,
// вместе с дедлайнами и адресами, и сузить порт до Read/Write/Close значило бы
// гарантированно переделывать его снизу вверх на этапе TLS.
type Conn struct {
	channelID uint32
	addr      Addr
	send      FrameSender
	sendWin   *SendWindow
	recvWin   *RecvWindow
	maxFrame  int

	inbox  chan []byte
	remain []byte // непрочитанный хвост последнего фрейма

	readDL  *deadline
	writeDL *deadline

	closeOnce sync.Once
	closed    chan struct{}

	// cancelWrites прерывает ожидание кредита при закрытии.
	cancelWrites context.CancelFunc
	writeCtx     context.Context
}

var _ net.Conn = (*Conn)(nil)

// NewConn создаёт соединение поверх канала.
//
// maxFrame — потолок ОДНОГО фрейма для purpose этого канала: 64 KiB для
// embed-stream, 1 MiB для tcp-relay. Резать обязаны мы: превышение — не
// backpressure, а протокольное нарушение, за которое ядро убивает процесс.
func NewConn(channelID uint32, addr Addr, send FrameSender, sendWin *SendWindow, recvWin *RecvWindow, maxFrame, inboxDepth int) *Conn {
	ctx, cancel := context.WithCancel(context.Background())
	return &Conn{
		channelID:    channelID,
		addr:         addr,
		send:         send,
		sendWin:      sendWin,
		recvWin:      recvWin,
		maxFrame:     maxFrame,
		inbox:        make(chan []byte, inboxDepth),
		readDL:       newDeadline(),
		writeDL:      newDeadline(),
		closed:       make(chan struct{}),
		writeCtx:     ctx,
		cancelWrites: cancel,
	}
}

// Deliver кладёт пришедший от хоста фрейм в очередь чтения.
//
// Вызывается из цикла диспетчера. Блокируется, если читатель отстал: это и есть
// backpressure. Буферизовать «на всякий случай» нельзя — неограниченная очередь
// ровно то, ради устранения чего существует кредитное окно.
func (c *Conn) Deliver(payload []byte) error {
	select {
	case <-c.closed:
		return ErrClosed
	case c.inbox <- payload:
		return nil
	}
}

func (c *Conn) Read(b []byte) (int, error) {
	if len(c.remain) > 0 {
		n := copy(b, c.remain)
		c.remain = c.remain[n:]
		if len(c.remain) == 0 {
			// Фрейм дочитан — место в окне освободилось, можно подтверждать.
			c.recvWin.Consumed(1)
		}
		return n, nil
	}

	if c.readDL.expired() {
		return 0, c.opError("read", os.ErrDeadlineExceeded)
	}

	select {
	case <-c.closed:
		return 0, c.opError("read", net.ErrClosed)
	case <-c.readDL.wait():
		return 0, c.opError("read", os.ErrDeadlineExceeded)
	case frame, ok := <-c.inbox:
		if !ok {
			return 0, io.EOF
		}
		n := copy(b, frame)
		if n < len(frame) {
			c.remain = frame[n:]
			return n, nil
		}
		c.recvWin.Consumed(1)
		return n, nil
	}
}

func (c *Conn) Write(b []byte) (int, error) {
	if c.writeDL.expired() {
		return 0, c.opError("write", os.ErrDeadlineExceeded)
	}
	select {
	case <-c.closed:
		return 0, c.opError("write", net.ErrClosed)
	default:
	}

	written := 0
	for written < len(b) {
		end := written + c.maxFrame
		if end > len(b) {
			end = len(b)
		}
		chunk := b[written:end]

		if err := c.acquireWriteCredit(); err != nil {
			return written, err
		}
		// Без копии: по контракту FrameSender фрейм уходит синхронно, а до
		// возврата из Write наш вызывающий свой буфер не трогает.
		if err := c.send.SendChannelFrame(c.channelID, chunk); err != nil {
			return written, c.opError("write", err)
		}
		written = end
	}
	return written, nil
}

// acquireWriteCredit ждёт кредита, соблюдая дедлайн записи и закрытие.
//
// Быстрый путь отдельно от медленного намеренно. При живой сессии кредит есть
// почти всегда, а медленный путь на каждый кадр поднимает контекст, сторожевую
// горутину и канал. Дедлайн при этом не теряется: не дождавшись кредита сразу,
// мы уходим в полный путь, где он соблюдается, — а Write уже проверил
// истёкший дедлайн на входе.
func (c *Conn) acquireWriteCredit() error {
	if c.sendWin.TryAcquire() {
		return nil
	}

	ctx, cancel := context.WithCancel(c.writeCtx)
	defer cancel()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-c.writeDL.wait():
			cancel()
		case <-done:
		}
	}()

	if err := c.sendWin.Acquire(ctx); err != nil {
		switch {
		case c.writeDL.expired():
			return c.opError("write", os.ErrDeadlineExceeded)
		case errors.Is(err, ErrClosed) || errors.Is(err, context.Canceled):
			return c.opError("write", net.ErrClosed)
		default:
			return c.opError("write", err)
		}
	}
	return nil
}

func (c *Conn) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
		c.cancelWrites()
		c.sendWin.Close()
		c.readDL.stop()
		c.writeDL.stop()
	})
	return nil
}

func (c *Conn) LocalAddr() net.Addr  { return c.addr }
func (c *Conn) RemoteAddr() net.Addr { return c.addr }

func (c *Conn) SetDeadline(t time.Time) error {
	c.readDL.set(t)
	c.writeDL.set(t)
	return nil
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	c.readDL.set(t)
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.writeDL.set(t)
	return nil
}

// opError оборачивает ошибку в *net.OpError — то, что вызывающие ждут от
// net.Conn. crypto/tls и обычный код проверяют таймаут через
// errors.Is(err, os.ErrDeadlineExceeded) и net.Error.Timeout(), и голая ошибка
// эти проверки бы провалила.
func (c *Conn) opError(op string, err error) error {
	return &net.OpError{Op: op, Net: c.addr.Network(), Addr: c.addr, Err: err}
}

// RecvWindow отдаёт приёмное окно канала.
//
// Нужен связке кредитных окон: чтобы притормозить RDP-сервер, она удерживает
// гранты именно этого окна. Возвращается сам объект, а не копия состояния —
// удержание должно действовать на то окно, которое реально выдаёт кредиты.
func (c *Conn) RecvWindow() *RecvWindow { return c.recvWin }
