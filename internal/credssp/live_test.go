package credssp_test

import (
	"crypto/tls"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"xqs-plugin-rdp/internal/credssp"
	"xqs-plugin-rdp/internal/trust"
	"xqs-plugin-rdp/internal/x224"
)

// Живая проверка CredSSP против настоящего NLA-сервера.
//
// Она выключена по умолчанию и включается переменными окружения:
//
//	XQS_RDP_TEST_HOST=192.168.31.38:3389
//	XQS_RDP_TEST_USER=...
//	XQS_RDP_TEST_PASS=...
//	XQS_RDP_TEST_DOMAIN=...        (необязательно)
//
// Почему она вообще нужна. Всё остальное в этом пакете проверяется структурно:
// DER round-trip, хеши против независимого счёта, кодировка UTF-16. Но сойдётся
// ли ЦЕПОЧКА — порядок сообщений, порядок «подпись перед данными» в GSS-обёртке,
// момент отправки nonce — структурными тестами не проверяется никак. Публичных
// векторов MS-CSSP нет. Единственный судья здесь — сервер.
//
// Почему она не в CI по умолчанию: ей нужны реальные учётные данные, а класть
// их в репозиторий нельзя. См. README раздела «Тестирование».
func TestLiveCredSSP(t *testing.T) {
	host := os.Getenv("XQS_RDP_TEST_HOST")
	user := os.Getenv("XQS_RDP_TEST_USER")
	pass := os.Getenv("XQS_RDP_TEST_PASS")
	if host == "" || user == "" {
		t.Skip("XQS_RDP_TEST_HOST / XQS_RDP_TEST_USER не заданы — живая проверка пропущена")
	}

	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		t.Fatalf("подключение к %s: %v", host, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatalf("дедлайн: %v", err)
	}

	// 1. X.224: просим HYBRID, как это будет делать плагин.
	cr, err := x224.RewriteRequestForServer(browserStyleRequest())
	if err != nil {
		t.Fatalf("RewriteRequestForServer: %v", err)
	}
	if _, err := conn.Write(cr); err != nil {
		t.Fatalf("отправка X.224 CR: %v", err)
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("чтение X.224 CC: %v", err)
	}
	cc, err := x224.ParseConnectConfirm(buf[:n])
	if err != nil {
		t.Fatalf("ParseConnectConfirm: %v", err)
	}
	if err := x224.RequireHybrid(cc.Selected); err != nil {
		t.Fatalf("сервер не выбрал NLA: %v", err)
	}
	t.Logf("X.224: сервер выбрал HYBRID, флаги %#x", cc.Flags)

	// 2. TLS. Проверку сертификата средствами crypto/tls отключаем намеренно:
	// RDP-серверы самоподписаны, и подлинность устанавливает internal/trust по
	// отпечатку, а не цепочка доверия, которой у них нет.
	tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // подлинность проверяется по отпечатку ниже
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("TLS: %v", err)
	}
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		t.Fatal("сервер не прислал сертификат")
	}
	fp, err := trust.Fingerprint(certs[0].Raw)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	t.Logf("TLS: %s, отпечаток сервера %s", tls.VersionName(tlsConn.ConnectionState().Version), fp)

	// 3. CredSSP поверх TLS.
	// Привязка считается от содержимого BIT STRING, а не от SPKI целиком — см.
	// trust.SubjectPublicKey.
	pubKey, err := trust.SubjectPublicKey(certs[0].Raw)
	if err != nil {
		t.Fatalf("SubjectPublicKey: %v", err)
	}
	err = credssp.Authenticate(tlsConn, pubKey, credssp.Credentials{
		Domain:   os.Getenv("XQS_RDP_TEST_DOMAIN"),
		Username: user,
		Password: []byte(pass),
	})
	if err == nil {
		t.Log("CredSSP: аутентификация ПРОШЛА — цепочка сошлась целиком")
		return
	}

	// Отказ по учётным данным — это тоже результат, и хороший: он означает, что
	// сервер разобрал наши TSRequest, довёл NTLM до конца и ответил по существу.
	// Провалом является только то, что до этого места не дошло.
	var rejected *credssp.ErrRejected
	if errors.As(err, &rejected) {
		t.Fatalf("CredSSP: %v. Дойдя до этого места, сервер уже разобрал наши TSRequest "+
			"и довёл NTLM до конца — формат сообщений верен. "+
			"Проверьте XQS_RDP_TEST_USER / XQS_RDP_TEST_PASS.", rejected)
	}
	t.Fatalf("CredSSP: %v", err)
}

// browserStyleRequest собирает X.224 Connection Request такой же формы, какую
// шлёт ironrdp-web: cookie + NEG_REQ с одним SSL.
func browserStyleRequest() []byte {
	cookie := []byte("Cookie: mstshash=xqs\r\n")
	body := append([]byte{0xE0, 0x00, 0x00, 0x00, 0x00, 0x00}, cookie...)
	body = append(body, 0x01, 0x00, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00)
	total := 4 + 1 + len(body)
	return append([]byte{0x03, 0x00, byte(total >> 8), byte(total), byte(len(body))}, body...)
}
