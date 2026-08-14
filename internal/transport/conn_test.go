package transport_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"xqs-plugin-rdp/internal/transport"
)

// fakeBus — другой конец канала. Он и есть то, ради чего FrameSender сделан
// портом: настоящая шина для этих проверок не нужна.
type fakeBus struct {
	mu     sync.Mutex
	frames [][]byte
	peer   *transport.Conn // куда доставлять отправленное, если сведены в пару
	fail   error
}

func (b *fakeBus) SendChannelFrame(_ uint32, payload []byte) error {
	b.mu.Lock()
	if b.fail != nil {
		err := b.fail
		b.mu.Unlock()
		return err
	}
	b.frames = append(b.frames, append([]byte(nil), payload...))
	peer := b.peer
	b.mu.Unlock()

	if peer != nil {
		return peer.Deliver(append([]byte(nil), payload...))
	}
	return nil
}

func (b *fakeBus) sent() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([][]byte, len(b.frames))
	copy(out, b.frames)
	return out
}

func newTestConn(bus transport.FrameSender, credit, maxFrame int) *transport.Conn {
	return transport.NewConn(
		7,
		transport.Addr{Purpose: "tcp-relay", ChannelID: 7},
		bus,
		transport.NewSendWindow(credit),
		transport.NewRecvWindow(8, nil),
		maxFrame,
		16,
	)
}

func TestConnSatisfiesNetConn(t *testing.T) {
	var c any = newTestConn(&fakeBus{}, 4, 64)
	if _, ok := c.(net.Conn); !ok {
		t.Fatal("Conn не реализует net.Conn — поверх него не заработает crypto/tls")
	}
}

func TestConnReadReturnsDeliveredFrames(t *testing.T) {
	c := newTestConn(&fakeBus{}, 4, 64)
	if err := c.Deliver([]byte("hello")); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	buf := make([]byte, 16)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf[:n]) != "hello" {
		t.Fatalf("прочитано %q", buf[:n])
	}
}

// Короткий буфер не должен терять хвост фрейма: поток байтовый, и «остаток
// выбросили» проявился бы как повреждённые данные далеко от места ошибки.
func TestConnReadKeepsRemainderOfFrame(t *testing.T) {
	c := newTestConn(&fakeBus{}, 4, 64)
	if err := c.Deliver([]byte("abcdef")); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	small := make([]byte, 2)
	var got []byte
	for i := 0; i < 3; i++ {
		n, err := c.Read(small)
		if err != nil {
			t.Fatalf("Read #%d: %v", i, err)
		}
		got = append(got, small[:n]...)
	}
	if string(got) != "abcdef" {
		t.Fatalf("собрано %q, ожидалось abcdef", got)
	}
}

// Кредит подтверждается только когда фрейм ДОЧИТАН, а не когда он получен:
// подтвердить раньше значит разрешить источнику прислать ещё, пока предыдущее
// лежит непрочитанным.
func TestConnAcknowledgesOnlyFullyReadFrames(t *testing.T) {
	var granted int
	var mu sync.Mutex
	c := transport.NewConn(1, transport.Addr{ChannelID: 1}, &fakeBus{},
		transport.NewSendWindow(4),
		transport.NewRecvWindow(8, func(n int) { mu.Lock(); granted += n; mu.Unlock() }),
		64, 4)

	if err := c.Deliver([]byte("abcdef")); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	small := make([]byte, 2)
	if _, err := c.Read(small); err != nil {
		t.Fatalf("Read: %v", err)
	}
	mu.Lock()
	early := granted
	mu.Unlock()
	if early != 0 {
		t.Fatalf("кредит подтверждён на недочитанном фрейме: %d", early)
	}
	for i := 0; i < 2; i++ {
		if _, err := c.Read(small); err != nil {
			t.Fatalf("Read: %v", err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if granted != 1 {
		t.Fatalf("подтверждено %d фреймов, ожидался 1", granted)
	}
}

// Запись режется по потолку purpose. Превышение — не backpressure, а смерть
// процесса, поэтому резать обязаны мы, а не надеяться на вызывающего.
func TestConnWriteChunksToMaxFrame(t *testing.T) {
	bus := &fakeBus{}
	c := newTestConn(bus, 100, 4)
	payload := []byte("0123456789")
	n, err := c.Write(payload)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("записано %d из %d", n, len(payload))
	}
	frames := bus.sent()
	if len(frames) != 3 {
		t.Fatalf("фреймов %d, ожидалось 3 (4+4+2)", len(frames))
	}
	for i, f := range frames {
		if len(f) > 4 {
			t.Fatalf("фрейм #%d длиной %d сверх потолка 4", i, len(f))
		}
	}
	if !bytes.Equal(bytes.Join(frames, nil), payload) {
		t.Fatalf("склейка фреймов не равна исходным данным")
	}
}

func TestConnWriteCopiesCallerBuffer(t *testing.T) {
	bus := &fakeBus{}
	c := newTestConn(bus, 10, 64)
	buf := []byte("secret")
	if _, err := c.Write(buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	copy(buf, "XXXXXX") // вызывающий переиспользовал буфер
	if got := string(bus.sent()[0]); got != "secret" {
		t.Fatalf("отправлено %q — буфер вызывающего не скопирован", got)
	}
}

// Дедлайн чтения обязан будить заблокированного читателя и, по семантике
// net.Conn, продолжать заваливать операции после истечения.
func TestConnReadDeadline(t *testing.T) {
	c := newTestConn(&fakeBus{}, 4, 64)
	if err := c.SetReadDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	start := time.Now()
	_, err := c.Read(make([]byte, 8))
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("err = %v, ожидался os.ErrDeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Read висел %v — дедлайн не сработал", elapsed)
	}
	// Ошибка обязана распознаваться как таймаут через net.Error.
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("err %v не распознаётся как таймаут net.Error", err)
	}
	// И повторный вызов обязан падать так же, а не «отвиснуть».
	if _, err := c.Read(make([]byte, 8)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("после истечения дедлайна Read вернул %v", err)
	}
}

func TestConnDeadlineCanBeReset(t *testing.T) {
	c := newTestConn(&fakeBus{}, 4, 64)
	if err := c.SetReadDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if _, err := c.Read(make([]byte, 4)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("истёкший дедлайн не сработал: %v", err)
	}
	// Снятие дедлайна возвращает соединение в рабочее состояние.
	if err := c.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("SetReadDeadline(zero): %v", err)
	}
	if err := c.Deliver([]byte("ok")); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	buf := make([]byte, 4)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("Read после снятия дедлайна: %v", err)
	}
	if string(buf[:n]) != "ok" {
		t.Fatalf("прочитано %q", buf[:n])
	}
}

// Запись на исчерпанном кредите обязана падать по дедлайну, а не висеть вечно.
func TestConnWriteDeadlineWhileWaitingForCredit(t *testing.T) {
	c := newTestConn(&fakeBus{}, 0, 64)
	if err := c.SetWriteDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	_, err := c.Write([]byte("x"))
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("err = %v, ожидался os.ErrDeadlineExceeded", err)
	}
}

func TestConnCloseUnblocksReaderAndWriter(t *testing.T) {
	c := newTestConn(&fakeBus{}, 0, 64)

	readErr := make(chan error, 1)
	go func() { _, err := c.Read(make([]byte, 4)); readErr <- err }()
	writeErr := make(chan error, 1)
	go func() { _, err := c.Write([]byte("x")); writeErr <- err }()
	time.Sleep(20 * time.Millisecond)

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for _, ch := range []chan error{readErr, writeErr} {
		select {
		case err := <-ch:
			if !errors.Is(err, net.ErrClosed) {
				t.Fatalf("err = %v, ожидался net.ErrClosed", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Close не разбудил заблокированную операцию")
		}
	}
	// Close идемпотентен: ядро закрывает каналы само, и второй вызов — норма.
	if err := c.Close(); err != nil {
		t.Fatalf("повторный Close: %v", err)
	}
}

func TestConnAddrsAreHonest(t *testing.T) {
	c := newTestConn(&fakeBus{}, 4, 64)
	if got := c.LocalAddr().Network(); got != "xqs-channel" {
		t.Fatalf("Network = %q — адрес не должен притворяться tcp", got)
	}
	if got := c.RemoteAddr().String(); got != "tcp-relay/7" {
		t.Fatalf("String = %q", got)
	}
}

func TestConnWriteSurfacesBusFailure(t *testing.T) {
	bus := &fakeBus{fail: errors.New("шина сломалась")}
	c := newTestConn(bus, 4, 64)
	if _, err := c.Write([]byte("x")); err == nil {
		t.Fatal("ошибка шины проглочена")
	}
}

// Главная проверка всего пакета: поверх пары таких соединений должен
// подниматься настоящий TLS. Ради этого Conn и реализует net.Conn целиком —
// если рукопожатие пройдёт, значит и дедлайны, и адреса, и потоковая семантика
// достаточны для crypto/tls, а не только для наших собственных тестов.
func TestTLSHandshakeOverPairedConns(t *testing.T) {
	clientBus, serverBus := &fakeBus{}, &fakeBus{}
	client := transport.NewConn(1, transport.Addr{Purpose: "tcp-relay", ChannelID: 1}, clientBus,
		transport.NewSendWindow(1<<20), transport.NewRecvWindow(8, nil), 4096, 256)
	server := transport.NewConn(2, transport.Addr{Purpose: "tcp-relay", ChannelID: 2}, serverBus,
		transport.NewSendWindow(1<<20), transport.NewRecvWindow(8, nil), 4096, 256)
	clientBus.peer = server
	serverBus.peer = client
	defer client.Close()
	defer server.Close()

	cert := selfSignedTLSCert(t)

	errCh := make(chan error, 1)
	go func() {
		s := tls.Server(server, &tls.Config{Certificates: []tls.Certificate{cert}})
		if err := s.Handshake(); err != nil {
			errCh <- err
			return
		}
		buf := make([]byte, 5)
		if _, err := io.ReadFull(s, buf); err != nil {
			errCh <- err
			return
		}
		_, err := s.Write(buf)
		errCh <- err
	}()

	c := tls.Client(client, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // самоподписанный сертификат теста
	if err := c.Handshake(); err != nil {
		t.Fatalf("TLS-рукопожатие поверх канала не прошло: %v", err)
	}
	if _, err := c.Write([]byte("ping!")); err != nil {
		t.Fatalf("запись через TLS: %v", err)
	}
	echo := make([]byte, 5)
	if _, err := io.ReadFull(c, echo); err != nil {
		t.Fatalf("чтение через TLS: %v", err)
	}
	if string(echo) != "ping!" {
		t.Fatalf("эхо %q", echo)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("серверная сторона: %v", err)
	}
}

func selfSignedTLSCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ключ: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("сертификат: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
