// Package proxy — единственное место, где сходятся x224, cleanpath, trust и
// credssp.
//
// Здесь живёт точка отсечения из §1 спеки целиком: браузеру возвращается
// Connect Confirm с selected = SSL, а весь NLA выполняется тут, между двумя
// недоверенными сторонами. После Handshake плагин байты не разбирает — начинается
// сырой релей.
package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"xqs-plugin-rdp/internal/cleanpath"
	"xqs-plugin-rdp/internal/credssp"
	"xqs-plugin-rdp/internal/trust"
	"xqs-plugin-rdp/internal/x224"
)

// handshakeTimeout ограничивает рукопожатие целиком.
//
// Оно складывается из дозвона, TLS и трёх сообщений NTLM. Отдельные таймауты на
// каждый шаг дали бы сумму, которую невозможно назвать пользователю; один общий
// отвечает на его единственный вопрос — сколько ждать.
const handshakeTimeout = 30 * time.Second

// Target — то, к чему подключаемся. Всё это приходит из session.connect.fields,
// то есть от хоста, а не из браузера.
type Target struct {
	// Address — «хост:порт». Берётся ТОЛЬКО отсюда. Адрес из RDCleanPath-запроса
	// игнорируется: его пишет iframe, и доверие к нему означало бы, что
	// скомпрометированная страница может заставить плагин дозвониться куда
	// угодно в локальной сети под нашим allowArbitraryOutbound.
	Address string
	// Fingerprint — подтверждённый пользователем отпечаток ключа сервера.
	// Пустой — отказ подключаться (см. internal/trust).
	Fingerprint string
	Credentials credssp.Credentials
}

// Handshake проводит рукопожатие между браузером и RDP-сервером.
//
// browser — открытый поток к embed-iframe, server — сырой TCP к RDP-серверу.
// Возвращает поток к серверу ВНУТРИ TLS: именно между ним и browser дальше идёт
// сырой релей.
//
// Порядок шагов не переставляется. В частности, отпечаток проверяется ДО
// CredSSP: привязка к ключу защищает разговор с тем, с кем мы фактически
// говорим, и запускать аутентификацию, ещё не зная, кто на том конце, значит
// предъявлять учётные данные неизвестно кому.
func Handshake(ctx context.Context, browser, server net.Conn, target Target) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	if deadline, ok := ctx.Deadline(); ok {
		_ = browser.SetDeadline(deadline)
		_ = server.SetDeadline(deadline)
	}

	req, err := readRequest(browser)
	if err != nil {
		return nil, err
	}

	// Адрес из запроса не используется, но расхождение с тем, что дал хост,
	// стоит увидеть: это либо подменённая страница, либо наше недопонимание.
	if req.Destination != "" && req.Destination != target.Address {
		slog.Warn("proxy: браузер запросил другой адрес — игнорируется",
			"browserDestination", req.Destination, "hostDestination", target.Address)
	}

	serverTLS, chain, err := connectServer(ctx, server, req.X224, target)
	if err != nil {
		return nil, err
	}

	// Ответ браузеру: Connect Confirm с selected = SSL. С этого момента он
	// считает, что NLA не выбран, и уходит прямо в MCS.
	resp, err := cleanpath.BuildResponse(x224.ConnectConfirmForBrowser(), chain, target.Address)
	if err != nil {
		return nil, err
	}
	if _, err := browser.Write(resp); err != nil {
		return nil, fmt.Errorf("proxy: отправка RDCleanPath-ответа: %w", err)
	}

	// Дедлайны рукопожатия сняты: дальше поток живёт без общего срока.
	_ = browser.SetDeadline(time.Time{})
	_ = server.SetDeadline(time.Time{})
	return serverTLS, nil
}

func readRequest(browser net.Conn) (cleanpath.Request, error) {
	raw, err := cleanpath.ReadMessage(browser)
	if err != nil {
		return cleanpath.Request{}, fmt.Errorf("proxy: чтение RDCleanPath-запроса: %w", err)
	}
	req, err := cleanpath.ParseRequest(raw)
	if err != nil {
		return cleanpath.Request{}, fmt.Errorf("proxy: разбор RDCleanPath-запроса: %w", err)
	}
	return req, nil
}

// connectServer доводит серверную половину до готового к релею TLS-потока.
func connectServer(ctx context.Context, server net.Conn, browserCR []byte, target Target) (net.Conn, [][]byte, error) {
	// 1. X.224: просим у сервера NLA, что бы ни просил браузер.
	cr, err := x224.RewriteRequestForServer(browserCR)
	if err != nil {
		return nil, nil, fmt.Errorf("proxy: перезапись Connection Request: %w", err)
	}
	if _, err := server.Write(cr); err != nil {
		return nil, nil, fmt.Errorf("proxy: отправка Connection Request: %w", err)
	}
	raw, err := x224.ReadPDU(server)
	if err != nil {
		return nil, nil, fmt.Errorf("proxy: чтение Connection Confirm: %w", err)
	}
	cc, err := x224.ParseConnectConfirm(raw)
	if err != nil {
		// NegotiationFailure долетает сюда как есть: у него внутри код, по
		// которому пользователь поймёт, что делать.
		return nil, nil, fmt.Errorf("proxy: %w", err)
	}
	if err := x224.RequireHybrid(cc.Selected); err != nil {
		return nil, nil, fmt.Errorf("proxy: %w", err)
	}

	// 2. TLS. Проверку цепочки средствами crypto/tls отключаем осознанно:
	// RDP-серверы самоподписаны, цепочки доверия у них нет, и подлинность
	// устанавливается по отпечатку — строкой ниже, а не «потом когда-нибудь».
	tlsConn := tls.Client(server, &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // подлинность проверяется по отпечатку, см. ниже
		MinVersion:         tls.VersionTLS12,
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return nil, nil, fmt.Errorf("proxy: TLS с сервером: %w", err)
	}
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, nil, errors.New("proxy: сервер не прислал сертификат")
	}

	// 3. Кто на том конце. До CredSSP, а не после.
	if err := trust.Verify(certs[0].Raw, target.Fingerprint); err != nil {
		return nil, nil, err
	}

	// 4. NLA. Пароль дальше этой функции не уходит.
	pubKey, err := trust.SubjectPublicKey(certs[0].Raw)
	if err != nil {
		return nil, nil, err
	}
	if err := credssp.Authenticate(tlsConn, pubKey, target.Credentials); err != nil {
		return nil, nil, err
	}

	chain := make([][]byte, 0, len(certs))
	for _, c := range certs {
		chain = append(chain, c.Raw)
	}
	return tlsConn, chain, nil
}

// ReportError сообщает браузеру, что подключение не состоялось.
//
// Без этого он показывает таймаут: страница ждёт RDCleanPath-ответа, которого
// уже не будет, и пользователь видит пустой экран вместо причины. Текст ошибки
// сюда не передаётся намеренно — в RDCleanPathErr для него нет места, а
// придумывать канал ради этого значило бы расширять формат; настоящая причина
// уходит в updateState и в лог.
func ReportError(browser io.Writer) {
	msg, err := cleanpath.BuildError(cleanpath.ErrCodeGeneral)
	if err != nil {
		slog.Error("proxy: не удалось собрать сообщение об ошибке", "err", err.Error())
		return
	}
	if _, err := browser.Write(msg); err != nil {
		slog.Debug("proxy: не удалось отправить ошибку браузеру", "err", err.Error())
	}
}
