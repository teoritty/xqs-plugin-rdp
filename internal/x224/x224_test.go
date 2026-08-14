package x224_test

import (
	"bytes"
	"errors"
	"testing"

	"xqs-plugin-rdp/internal/x224"
)

// Байты сняты с живого сервера (192.168.31.38) и с ironrdp-web, а не выведены
// из спеки: спека допускает варианты, а эти два и есть то, между чем плагин
// сидит.
var (
	// Ответ настоящего Windows-сервера на запрос SSL|HYBRID.
	liveConfirmHybrid = []byte{
		0x03, 0x00, 0x00, 0x13,
		0x0e, 0xd0, 0x00, 0x00, 0x12, 0x34, 0x00,
		0x02, 0x1f, 0x08, 0x00, 0x02, 0x00, 0x00, 0x00,
	}
	// Он же на запрос голого SSL: NEG_FAILURE 0x5 = HYBRID_REQUIRED_BY_SERVER.
	liveFailureHybridRequired = []byte{
		0x03, 0x00, 0x00, 0x13,
		0x0e, 0xd0, 0x00, 0x00, 0x12, 0x34, 0x00,
		0x03, 0x00, 0x08, 0x00, 0x05, 0x00, 0x00, 0x00,
	}
)

func TestParseConfirmReadsLiveServerResponse(t *testing.T) {
	cc, err := x224.ParseConnectConfirm(liveConfirmHybrid)
	if err != nil {
		t.Fatalf("ParseConnectConfirm: %v", err)
	}
	if cc.Selected != x224.ProtocolHybrid {
		t.Fatalf("Selected = %#x, ожидался HYBRID", cc.Selected)
	}
	if cc.Flags != 0x1f {
		t.Fatalf("Flags = %#x, ожидался 0x1f", cc.Flags)
	}
}

// NEG_FAILURE — не «ошибка разбора», а разобранный отказ сервера с кодом.
// Пользователю нужен код, а не «connection failed».
func TestParseConfirmSurfacesNegotiationFailure(t *testing.T) {
	_, err := x224.ParseConnectConfirm(liveFailureHybridRequired)
	if err == nil {
		t.Fatal("NEG_FAILURE обязан быть ошибкой")
	}
	var nf *x224.NegotiationFailure
	if !errors.As(err, &nf) {
		t.Fatalf("err типа %T, ожидался *x224.NegotiationFailure", err)
	}
	if nf.Code != x224.FailureHybridRequiredByServer {
		t.Fatalf("Code = %#x, ожидался HYBRID_REQUIRED_BY_SERVER", nf.Code)
	}
	if !bytes.Contains([]byte(nf.Error()), []byte("NLA")) {
		t.Fatalf("текст ошибки не объясняет причину: %q", nf.Error())
	}
}

func TestParseRequestReadsBrowserRequest(t *testing.T) {
	// Запрос ironrdp-web с enableCredssp(false): cookie + NEG_REQ с SSL.
	cookie := []byte("Cookie: mstshash=spike\r\n")
	body := append([]byte{0xE0, 0x00, 0x00, 0x00, 0x00, 0x00}, cookie...)
	body = append(body, 0x01, 0x00, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00)
	total := 4 + 1 + len(body)
	cr := append([]byte{0x03, 0x00, byte(total >> 8), byte(total), byte(len(body))}, body...)

	got, err := x224.ParseConnectRequest(cr)
	if err != nil {
		t.Fatalf("ParseConnectRequest: %v", err)
	}
	if got.Requested != x224.ProtocolSSL {
		t.Fatalf("Requested = %#x", got.Requested)
	}
	if !bytes.Contains(got.Variable, cookie) {
		t.Fatal("cookie потерян: он обязан доехать до сервера как есть")
	}
}

// Перезапись в сторону сервера: ровно HYBRID, без HYBRID_EX.
//
// Живой сервер выбирает HYBRID_EX, если его предложить, а HYBRID_EX добавляет
// после CredSSP лишний Early User Authorization Result PDU — состояние, которое
// нам нечем обслуживать и незачем заводить.
func TestRewriteRequestAsksForHybridOnly(t *testing.T) {
	cookie := []byte("Cookie: mstshash=spike\r\n")
	body := append([]byte{0xE0, 0x00, 0x00, 0x00, 0x00, 0x00}, cookie...)
	body = append(body, 0x01, 0x00, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00)
	total := 4 + 1 + len(body)
	cr := append([]byte{0x03, 0x00, byte(total >> 8), byte(total), byte(len(body))}, body...)

	out, err := x224.RewriteRequestForServer(cr)
	if err != nil {
		t.Fatalf("RewriteRequestForServer: %v", err)
	}
	got, err := x224.ParseConnectRequest(out)
	if err != nil {
		t.Fatalf("перезаписанный CR не разбирается: %v", err)
	}
	if got.Requested != x224.ProtocolHybrid {
		t.Fatalf("Requested = %#x, ожидался ровно HYBRID (0x2)", got.Requested)
	}
	if got.Requested&x224.ProtocolHybridEx != 0 {
		t.Fatal("HYBRID_EX не должен запрашиваться")
	}
	if !bytes.Contains(out, cookie) {
		t.Fatal("cookie обязан пережить перезапись: сервер по нему маршрутизирует")
	}
}

// CR без NEG_REQ означает standard RDP security. Дописать туда NEG_REQ нельзя
// молча: изменится длина, и это уже не «перезапись поля».
func TestRewriteRequestRejectsRequestWithoutNegReq(t *testing.T) {
	cr := []byte{0x03, 0x00, 0x00, 0x0B, 0x06, 0xE0, 0x00, 0x00, 0x00, 0x00, 0x00}
	if _, err := x224.RewriteRequestForServer(cr); err == nil {
		t.Fatal("CR без NEG_REQ обязан быть отказом, а не молчаливой дописью")
	}
}

// Ответ браузеру: selected = SSL. Это и есть точка отсечения.
func TestConfirmForBrowserSelectsSSL(t *testing.T) {
	out := x224.ConnectConfirmForBrowser()
	cc, err := x224.ParseConnectConfirm(out)
	if err != nil {
		t.Fatalf("ParseConnectConfirm: %v", err)
	}
	if cc.Selected != x224.ProtocolSSL {
		t.Fatalf("Selected = %#x, ожидался SSL", cc.Selected)
	}
	want := []byte{
		0x03, 0x00, 0x00, 0x13,
		0x0E, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00,
	}
	if !bytes.Equal(out, want) {
		t.Fatalf("\n got % x\nwant % x", out, want)
	}
}

func TestParseConfirmRejectsMalformed(t *testing.T) {
	cases := map[string][]byte{
		"пусто":            {},
		"обрыв в TPKT":     {0x03, 0x00},
		"не TPKT":          {0x04, 0x00, 0x00, 0x0B, 0x06, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00},
		"length > буфера":  {0x03, 0x00, 0xFF, 0xFF, 0x0E, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00},
		"это CR, а не CC":  {0x03, 0x00, 0x00, 0x0B, 0x06, 0xE0, 0x00, 0x00, 0x00, 0x00, 0x00},
		"NEG_RSP обрезан":  {0x03, 0x00, 0x00, 0x11, 0x0C, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x08, 0x00, 0x01, 0x00},
		"NEG_RSP длина!=8": {0x03, 0x00, 0x00, 0x13, 0x0E, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x09, 0x00, 0x01, 0x00, 0x00, 0x00},
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := x224.ParseConnectConfirm(b); err == nil {
				t.Fatal("обязана быть ошибка")
			}
		})
	}
}

// Сервер, ответивший без NEG_RSP, выбрал standard RDP security. Это downgrade:
// пользователь просил NLA, и продолжать нельзя.
func TestParseConfirmWithoutNegRspIsStandardSecurity(t *testing.T) {
	cc, err := x224.ParseConnectConfirm([]byte{0x03, 0x00, 0x00, 0x0B, 0x06, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err != nil {
		t.Fatalf("ParseConnectConfirm: %v", err)
	}
	if cc.Selected != x224.ProtocolRDP {
		t.Fatalf("Selected = %#x, ожидался ProtocolRDP", cc.Selected)
	}
}

// Единственная функция, которой поручено решать, приемлем ли выбор сервера.
// Она отдельная и с говорящим именем, чтобы downgrade нельзя было пропустить
// по невнимательности в длинной цепочке if-ов.
func TestRequireHybrid(t *testing.T) {
	cases := []struct {
		selected uint32
		ok       bool
	}{
		{x224.ProtocolHybrid, true},
		{x224.ProtocolRDP, false},
		{x224.ProtocolSSL, false},
		{x224.ProtocolHybridEx, false},
	}
	for _, tc := range cases {
		err := x224.RequireHybrid(tc.selected)
		if tc.ok && err != nil {
			t.Fatalf("selected=%#x: %v", tc.selected, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("selected=%#x обязан быть отказом: пользователь просил NLA", tc.selected)
		}
	}
}

// Каждый код отказа обязан иметь человеческий текст. Тип NegotiationFailure
// существует ровно затем, чтобы пользователь увидел причину вместо
// «connection failed», и молчаливый пропуск кода сводит его на нет.
func TestEveryFailureCodeHasText(t *testing.T) {
	codes := []uint32{
		x224.FailureSSLRequiredByServer,
		x224.FailureSSLNotAllowedByServer,
		x224.FailureSSLCertNotOnServer,
		x224.FailureInconsistentFlags,
		x224.FailureHybridRequiredByServer,
		x224.FailureSSLWithUserAuthRequired,
	}
	for _, c := range codes {
		msg := (&x224.NegotiationFailure{Code: c}).Error()
		if bytes.Contains([]byte(msg), []byte("неизвестная причина")) {
			t.Fatalf("код %#x не описан: %q", c, msg)
		}
	}
	// А неизвестный код обязан честно называться неизвестным, а не молчать.
	if msg := (&x224.NegotiationFailure{Code: 0xFF}).Error(); !bytes.Contains([]byte(msg), []byte("неизвестная")) {
		t.Fatalf("неизвестный код описан как %q", msg)
	}
}

// RequireHybrid объясняет, что именно выбрал сервер: «не NLA» без подробностей
// не даёт пользователю ничего.
func TestRequireHybridNamesWhatServerChose(t *testing.T) {
	cases := map[uint32]string{
		x224.ProtocolRDP:      "standard RDP security",
		x224.ProtocolSSL:      "TLS без NLA",
		x224.ProtocolHybridEx: "NLA-EX",
		0x40:                  "неизвестный протокол",
	}
	for sel, want := range cases {
		err := x224.RequireHybrid(sel)
		if err == nil {
			t.Fatalf("selected=%#x обязан быть отказом", sel)
		}
		if !bytes.Contains([]byte(err.Error()), []byte(want)) {
			t.Fatalf("selected=%#x: %q не содержит %q", sel, err.Error(), want)
		}
	}
}
