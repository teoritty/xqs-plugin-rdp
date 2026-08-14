package proxy_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"errors"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"xqs-plugin-rdp/internal/cleanpath"
	"xqs-plugin-rdp/internal/credssp"
	"xqs-plugin-rdp/internal/proxy"
	"xqs-plugin-rdp/internal/trust"
	"xqs-plugin-rdp/internal/x224"
)

func testCert(t *testing.T) (tls.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ключ: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "rdp-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("сертификат: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, der
}

// browserRequest собирает RDCleanPath-запрос такой же формы, какую шлёт
// ironrdp-web с enableCredssp(false).
func browserRequest(t *testing.T, destination string) []byte {
	t.Helper()
	cookie := []byte("Cookie: mstshash=xqs\r\n")
	body := append([]byte{0xE0, 0x00, 0x00, 0x00, 0x00, 0x00}, cookie...)
	body = append(body, 0x01, 0x00, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00)
	total := 4 + 1 + len(body)
	cr := append([]byte{0x03, 0x00, byte(total >> 8), byte(total), byte(len(body))}, body...)

	raw, err := cleanpath.BuildRequestForTest(cr, destination, "token")
	if err != nil {
		t.Fatalf("BuildRequestForTest: %v", err)
	}
	return raw
}

// connectConfirm собирает ответ сервера с заданным выбранным протоколом.
func connectConfirm(selected uint32) []byte {
	out := []byte{0x03, 0x00, 0x00, 0x13, 0x0E, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x08, 0x00}
	return binary.LittleEndian.AppendUint32(out, selected)
}

func negotiationFailure(code uint32) []byte {
	out := []byte{0x03, 0x00, 0x00, 0x13, 0x0E, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x08, 0x00}
	return binary.LittleEndian.AppendUint32(out, code)
}

// fakeServer — RDP-сервер ровно настолько, насколько нужно тестам: X.224,
// потом при желании TLS. Он же свидетель: запоминает, дошли ли до него
// прикладные байты после TLS.
type fakeServer struct {
	conn net.Conn

	mu       sync.Mutex
	gotCR    []byte
	appBytes int
}

func (s *fakeServer) run(t *testing.T, confirm []byte, cert *tls.Certificate) {
	t.Helper()
	cr, err := x224.ReadPDU(s.conn)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.gotCR = cr
	s.mu.Unlock()

	if _, err := s.conn.Write(confirm); err != nil {
		return
	}
	if cert == nil {
		return
	}
	tlsConn := tls.Server(s.conn, &tls.Config{Certificates: []tls.Certificate{*cert}})
	if err := tlsConn.Handshake(); err != nil {
		return
	}
	// После рукопожатия сервер молчит и только считает, что ему прислали.
	buf := make([]byte, 4096)
	for {
		n, err := tlsConn.Read(buf)
		s.mu.Lock()
		s.appBytes += n
		s.mu.Unlock()
		if err != nil {
			return
		}
	}
}

func (s *fakeServer) requestedProtocols(t *testing.T) uint32 {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gotCR == nil {
		t.Fatal("сервер не получил Connection Request")
	}
	req, err := x224.ParseConnectRequest(s.gotCR)
	if err != nil {
		t.Fatalf("ParseConnectRequest: %v", err)
	}
	return req.Requested
}

func (s *fakeServer) applicationBytes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appBytes
}

type harness struct {
	browser   net.Conn // наш конец потока к странице
	page      net.Conn // конец «страницы»
	serverOur net.Conn // наш конец потока к серверу
	server    *fakeServer
}

func newHarness(t *testing.T, confirm []byte, cert *tls.Certificate) *harness {
	t.Helper()
	page, browser := net.Pipe()
	srvOur, srvTheir := net.Pipe()
	h := &harness{browser: browser, page: page, server: &fakeServer{conn: srvTheir}}
	go h.server.run(t, confirm, cert)
	t.Cleanup(func() {
		_ = page.Close()
		_ = browser.Close()
		_ = srvOur.Close()
		_ = srvTheir.Close()
	})
	h.serverOur = srvOur
	return h
}

func (h *harness) writeRequest(t *testing.T, raw []byte) {
	t.Helper()
	go func() {
		_, _ = h.page.Write(raw)
	}()
}

// trustAll и denyAll — ответы ядра, а не политика плагина: решение переехало
// в ядро, и здесь мы проверяем, что плагин с обоими ответами обращается верно.
func trustAll(context.Context, []byte) (bool, error) { return true, nil }
func denyAll(context.Context, []byte) (bool, error)  { return false, nil }

// Главная проверка пакета: пока ядро не подтвердило сервер, учётные данные
// никуда не уходят. Подключение обязано оборваться ДО CredSSP, а не «на всякий
// случай попробовать».
func TestHandshakeRefusesUntrustedServerBeforeSendingCredentials(t *testing.T) {
	cert, _ := testCert(t)
	h := newHarness(t, connectConfirm(x224.ProtocolHybrid), &cert)
	h.writeRequest(t, browserRequest(t, "10.0.0.9:3389"))

	_, err := proxy.Handshake(context.Background(), h.browser, h.serverOur, proxy.Target{
		Address:     "192.168.31.38:3389",
		VerifyPeer:  denyAll, // ядро задало вопрос пользователю
		Credentials: credssp.Credentials{Username: "admin", Password: []byte("hunter2")},
	})
	if !errors.Is(err, proxy.ErrPeerNotTrusted) {
		t.Fatalf("err = %v, ожидалась ErrPeerNotTrusted", err)
	}
	// И, что важнее формулировки ошибки: серверу не отправлено ни байта
	// прикладных данных — то есть ни NTLM, ни тем более учётных данных.
	if n := h.server.applicationBytes(); n != 0 {
		t.Fatalf("серверу отправлено %d прикладных байт до проверки подлинности", n)
	}
}

// Цель без проверки доверия — отказ, а не подключение без проверки. Забытая
// проводка не должна молча превращаться в отсутствие защиты.
func TestHandshakeRefusesTargetWithoutVerifier(t *testing.T) {
	cert, _ := testCert(t)
	h := newHarness(t, connectConfirm(x224.ProtocolHybrid), &cert)
	h.writeRequest(t, browserRequest(t, "x"))

	_, err := proxy.Handshake(context.Background(), h.browser, h.serverOur, proxy.Target{
		Address:     "192.168.31.38:3389",
		Credentials: credssp.Credentials{Username: "admin", Password: []byte("hunter2")},
	})
	if !errors.Is(err, proxy.ErrNoPeerVerifier) {
		t.Fatalf("err = %v, ожидалась ErrNoPeerVerifier", err)
	}
	if n := h.server.applicationBytes(); n != 0 {
		t.Fatalf("серверу отправлено %d прикладных байт без всякой проверки", n)
	}
}

// Недоступное ядро — тоже отказ. Сбой связи с тем, кто принимает решение, не
// повод принять решение самому.
func TestHandshakeRefusesWhenTheCoreCannotAnswer(t *testing.T) {
	cert, _ := testCert(t)
	h := newHarness(t, connectConfirm(x224.ProtocolHybrid), &cert)
	h.writeRequest(t, browserRequest(t, "x"))

	boom := errors.New("шина недоступна")
	_, err := proxy.Handshake(context.Background(), h.browser, h.serverOur, proxy.Target{
		Address: "192.168.31.38:3389",
		VerifyPeer: func(context.Context, []byte) (bool, error) {
			return false, boom
		},
		Credentials: credssp.Credentials{Username: "admin", Password: []byte("hunter2")},
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидалась причина от ядра", err)
	}
	if errors.Is(err, proxy.ErrPeerNotTrusted) {
		t.Fatal("сбой связи выдан за отказ пользователя — это разные события")
	}
	if n := h.server.applicationBytes(); n != 0 {
		t.Fatalf("серверу отправлено %d прикладных байт при недоступном ядре", n)
	}
}

// Ядру уходит SubjectPublicKeyInfo сервера — ровно то, по чему оно считает
// отпечаток. Прислать другие байты значило бы записать доверие не к тому, с кем
// мы говорим, и потом никогда с ним не совпасть.
func TestHandshakeHandsTheCoreTheServersSPKI(t *testing.T) {
	cert, _ := testCert(t)
	h := newHarness(t, connectConfirm(x224.ProtocolHybrid), &cert)
	h.writeRequest(t, browserRequest(t, "x"))

	var seen []byte
	_, _ = proxy.Handshake(context.Background(), h.browser, h.serverOur, proxy.Target{
		Address: "192.168.31.38:3389",
		VerifyPeer: func(_ context.Context, material []byte) (bool, error) {
			seen = append([]byte(nil), material...)
			return false, nil
		},
		Credentials: credssp.Credentials{Username: "u", Password: []byte("p")},
	})

	want, err := trust.PeerMaterial(cert.Certificate[0])
	if err != nil {
		t.Fatalf("PeerMaterial: %v", err)
	}
	if !bytes.Equal(seen, want) {
		t.Fatalf("ядру ушло %d байт, ожидался SPKI сервера (%d байт)", len(seen), len(want))
	}
}

// Что бы ни просил браузер, серверу уходит HYBRID — и только он.
func TestHandshakeAsksServerForHybridOnly(t *testing.T) {
	cert, _ := testCert(t)
	h := newHarness(t, connectConfirm(x224.ProtocolHybrid), &cert)
	h.writeRequest(t, browserRequest(t, "x"))

	// denyAll, а не trustAll: запрошенные протоколы уходят серверу ДО проверки
	// доверия, а подтверждённый сервер увёл бы тест в CredSSP и заставил его
	// ждать полный таймаут рукопожатия ради значения, известного раньше.
	_, _ = proxy.Handshake(context.Background(), h.browser, h.serverOur, proxy.Target{
		Address:     "192.168.31.38:3389",
		VerifyPeer:  denyAll,
		Credentials: credssp.Credentials{Username: "u", Password: []byte("p")},
	})

	got := h.server.requestedProtocols(t)
	if got != x224.ProtocolHybrid {
		t.Fatalf("серверу запрошено %#x, ожидался ровно HYBRID (0x2)", got)
	}
}

// Сервер согласился на TLS без NLA — это downgrade. Пользователь просил NLA.
func TestHandshakeRefusesDowngradeToSSL(t *testing.T) {
	h := newHarness(t, connectConfirm(x224.ProtocolSSL), nil)
	h.writeRequest(t, browserRequest(t, "x"))

	_, err := proxy.Handshake(context.Background(), h.browser, h.serverOur, proxy.Target{
		Address:     "192.168.31.38:3389",
		VerifyPeer:  trustAll,
		Credentials: credssp.Credentials{Username: "u", Password: []byte("p")},
	})
	if err == nil {
		t.Fatal("downgrade принят")
	}
	if !strings.Contains(err.Error(), "NLA") {
		t.Fatalf("ошибка не объясняет причину: %v", err)
	}
}

// NEG_FAILURE обязан доходить до вызывающего с кодом, а не превращаться в
// «connection failed».
func TestHandshakeSurfacesNegotiationFailure(t *testing.T) {
	h := newHarness(t, negotiationFailure(x224.FailureSSLCertNotOnServer), nil)
	h.writeRequest(t, browserRequest(t, "x"))

	_, err := proxy.Handshake(context.Background(), h.browser, h.serverOur, proxy.Target{
		Address:     "192.168.31.38:3389",
		VerifyPeer:  trustAll,
		Credentials: credssp.Credentials{Username: "u", Password: []byte("p")},
	})
	var failure *x224.NegotiationFailure
	if !errors.As(err, &failure) {
		t.Fatalf("err типа %T (%v), ожидался *x224.NegotiationFailure", err, err)
	}
	if failure.Code != x224.FailureSSLCertNotOnServer {
		t.Fatalf("код %#x", failure.Code)
	}
}

// Мусор вместо RDCleanPath-запроса — недоверенный вход, и он не должен ни
// падать паникой, ни зависать.
func TestHandshakeRejectsGarbageFromBrowser(t *testing.T) {
	h := newHarness(t, connectConfirm(x224.ProtocolHybrid), nil)
	h.writeRequest(t, []byte{0xFF, 0xFF, 0xFF, 0xFF})

	_, err := proxy.Handshake(context.Background(), h.browser, h.serverOur, proxy.Target{
		Address:     "192.168.31.38:3389",
		VerifyPeer:  trustAll,
		Credentials: credssp.Credentials{Username: "u", Password: []byte("p")},
	})
	if err == nil {
		t.Fatal("мусор от браузера принят")
	}
}

// ReportError — единственный способ сказать браузеру «не вышло». Без него
// страница ждёт ответа, которого не будет, и пользователь видит пустой экран.
func TestReportErrorWritesParseableMessage(t *testing.T) {
	page, ours := net.Pipe()
	defer page.Close()
	defer ours.Close()

	go proxy.ReportError(ours)

	if err := page.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	raw, err := cleanpath.ReadMessage(page)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if len(raw) == 0 || raw[0] != 0x30 {
		t.Fatalf("сообщение об ошибке не DER: % x", raw)
	}
}
