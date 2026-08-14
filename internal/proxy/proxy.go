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
	"encoding/binary"
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
	// VerifyPeer спрашивает у ядра, доверяем ли мы тому, кто на том конце.
	//
	// Функция, а не отпечаток в поле: решение принимает пользователь, а помнит
	// ядро. Плагину при isolation per-session помнить негде — каталог данных
	// живёт ровно одну сессию, — и хранить доверие тут значило бы обещать то,
	// чего мы сдержать не можем.
	//
	// nil — отказ подключаться. Проверка, которую забыли подключить, не должна
	// молча превращаться в её отсутствие.
	VerifyPeer  func(ctx context.Context, material []byte) (bool, error)
	Credentials credssp.Credentials
}

// ErrPeerNotTrusted — ядро не подтвердило сервер.
//
// Отдельная ошибка, а не текст: вызывающий обязан отличить её от сбоя. Ядро в
// этот момент уже спросило пользователя и перевело сессию в ожидание решения,
// поэтому сообщать поверх этого об ошибке значило бы затереть заданный вопрос
// ответом «не удалось».
var ErrPeerNotTrusted = errors.New("proxy: сервер не подтверждён пользователем")

// ErrNoPeerVerifier — цель собрана без проверки доверия.
var ErrNoPeerVerifier = errors.New("proxy: не задана проверка доверия к серверу")

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

	leg, err := connectServer(ctx, server, req.X224, target)
	if err != nil {
		return nil, err
	}
	serverTLS := leg.tls

	resp, err := browserResponse(leg, target.Address)
	if err != nil {
		return nil, err
	}
	if _, err := browser.Write(resp); err != nil {
		return nil, fmt.Errorf("proxy: отправка RDCleanPath-ответа: %w", err)
	}

	// Последний шаг рукопожатия: первое сообщение браузера правится и уходит
	// серверу. Дальше поток не разбирается совсем.
	if err := forwardConnectInitial(browser, serverTLS, leg.selected); err != nil {
		return nil, err
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

// serverLeg — серверная половина, доведённая до готовности к релею.
//
// Структура, а не три возврата подряд: connectConfirm и chain — это разные
// вещи одинакового вида ([]byte и [][]byte легко перепутать местами глазами,
// хоть и не компилятором), и обе уезжают браузеру в одном ответе.
type serverLeg struct {
	tls   net.Conn
	chain [][]byte
	// selected — протокол, который выбрал сервер. Браузеру он не сообщается
	// (тот его отвергнет), но именно его сервер ждёт обратно в MCS.
	selected uint32
}

// connectServer доводит серверную половину до готового к релею TLS-потока.
func connectServer(ctx context.Context, server net.Conn, browserCR []byte, target Target) (serverLeg, error) {
	// 1. X.224: просим у сервера NLA, что бы ни просил браузер.
	cr, err := x224.RewriteRequestForServer(browserCR)
	if err != nil {
		return serverLeg{}, fmt.Errorf("proxy: перезапись Connection Request: %w", err)
	}
	if _, err := server.Write(cr); err != nil {
		return serverLeg{}, fmt.Errorf("proxy: отправка Connection Request: %w", err)
	}
	raw, err := x224.ReadPDU(server)
	if err != nil {
		return serverLeg{}, fmt.Errorf("proxy: чтение Connection Confirm: %w", err)
	}
	cc, err := x224.ParseConnectConfirm(raw)
	if err != nil {
		// NegotiationFailure долетает сюда как есть: у него внутри код, по
		// которому пользователь поймёт, что делать.
		return serverLeg{}, fmt.Errorf("proxy: %w", err)
	}
	if err := x224.RequireHybrid(cc.Selected); err != nil {
		return serverLeg{}, fmt.Errorf("proxy: %w", err)
	}

	// 2. TLS. Проверку цепочки средствами crypto/tls отключаем осознанно:
	// RDP-серверы самоподписаны, цепочки доверия у них нет, и подлинность
	// устанавливается по отпечатку — строкой ниже, а не «потом когда-нибудь».
	tlsConn := tls.Client(server, &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // подлинность проверяется по отпечатку, см. ниже
		MinVersion:         tls.VersionTLS12,
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return serverLeg{}, fmt.Errorf("proxy: TLS с сервером: %w", err)
	}
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return serverLeg{}, errors.New("proxy: сервер не прислал сертификат")
	}

	// 3. Кто на том конце. До CredSSP, а не после.
	if err := verifyPeer(ctx, certs[0].Raw, target); err != nil {
		return serverLeg{}, err
	}

	// 4. NLA. Пароль дальше этой функции не уходит.
	pubKey, err := trust.SubjectPublicKey(certs[0].Raw)
	if err != nil {
		return serverLeg{}, err
	}
	if err := credssp.Authenticate(tlsConn, pubKey, target.Credentials); err != nil {
		return serverLeg{}, err
	}

	chain := make([][]byte, 0, len(certs))
	for _, c := range certs {
		chain = append(chain, c.Raw)
	}
	return serverLeg{tls: tlsConn, chain: chain, selected: cc.Selected}, nil
}

// browserResponse собирает RDCleanPath-ответ браузеру.
//
// Connect Confirm называет SSL, а не то, что выбрал сервер. Это не небрежность
// и не упрощение: клиент объявляет только SSL (страница отключает CredSSP) и
// отвергает ответ с любым другим протоколом — «client advertised SSL, but
// server selected HYBRID», ironrdp-connector/src/lib.rs:409. Проверено на
// стенде: с HYBRID клиент закрывает сокет, не отправив ни байта.
//
// Расплата за это — правка одного поля в первом MCS-сообщении, см. mcs.go.
//
// Флаги нулевые намеренно: они относятся к возможностям, о которых с браузером
// договариваться не о чем.
func browserResponse(leg serverLeg, addr string) ([]byte, error) {
	return cleanpath.BuildResponse(connectConfirmForBrowser(), leg.chain, addr)
}

// connectConfirmForBrowser собирает Connection Confirm с selected = SSL.
func connectConfirmForBrowser() []byte {
	out := []byte{0x03, 0x00, 0x00, 0x13, 0x0E, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x08, 0x00}
	return binary.LittleEndian.AppendUint32(out, x224.ProtocolSSL)
}

// forwardConnectInitial принимает от браузера первое сообщение после точки
// отсечения, исправляет в нём заявленный протокол и отправляет серверу.
//
// Читается ровно один TPKT-пакет: за ним сразу идёт поток, который мы больше не
// разбираем, и лишний прочитанный байт увёл бы его у релея.
func forwardConnectInitial(browser io.Reader, server io.Writer, selected uint32) error {
	pdu, err := x224.ReadPDU(browser)
	if err != nil {
		return fmt.Errorf("proxy: чтение MCS Connect Initial: %w", err)
	}
	if !patchServerSelectedProtocol(pdu, selected) {
		// Поле необязательное. Не найдя его, идём дальше: сервер такому
		// клиенту ничего не предъявит, а обрывать подключение из-за
		// отсутствия того, что и не обязано быть, — хуже.
		slog.Warn("proxy: в MCS Connect Initial нет CS_CORE — протокол не исправлен")
	}
	if _, err := server.Write(pdu); err != nil {
		return fmt.Errorf("proxy: отправка MCS Connect Initial: %w", err)
	}
	return nil
}

// verifyPeer спрашивает ядро, тот ли это сервер.
//
// Материал уходит целиком, отпечаток считает ядро. Прислать готовый отпечаток
// значило бы позволить показать пользователю одно значение, а запомнить
// другое; здесь мы отпечаток считаем только чтобы назвать сервер в журнале.
func verifyPeer(ctx context.Context, certDER []byte, target Target) error {
	if target.VerifyPeer == nil {
		return ErrNoPeerVerifier
	}
	material, err := trust.PeerMaterial(certDER)
	if err != nil {
		return err
	}
	trusted, err := target.VerifyPeer(ctx, material)
	if err != nil {
		return fmt.Errorf("proxy: проверка доверия: %w", err)
	}
	if trusted {
		return nil
	}
	if fp, err := trust.Fingerprint(certDER); err == nil {
		slog.Info("proxy: сервер не подтверждён — ждём решения пользователя",
			"address", target.Address, "fingerprint", fp)
	}
	return ErrPeerNotTrusted
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
