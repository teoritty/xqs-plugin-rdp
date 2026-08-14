package cleanpath_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"xqs-plugin-rdp/internal/cleanpath"
)

// Настоящий запрос от ironrdp-web 0.7.0, снятый со стенда фазы 0a. Это лучший
// из возможных векторов: он описывает не то, что мы прочитали в спеке, а то,
// что реально приходит.
const liveRequestHex = "3050a00402020d3ea2100c0e3132372e302e302e313a33333839a3070c057370696b65" +
	"a62d042b0300002b26e00000000000436f6f6b69653a206d737473686173683d7370696b650d0a010008000100" +
	"0000"

func liveRequest(t *testing.T) []byte {
	t.Helper()
	b, err := hex.DecodeString(liveRequestHex)
	if err != nil {
		t.Fatalf("вектор не декодируется: %v", err)
	}
	return b
}

func TestParseRequestReadsLiveBrowserRequest(t *testing.T) {
	req, err := cleanpath.ParseRequest(liveRequest(t))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.Destination != "127.0.0.1:3389" {
		t.Fatalf("Destination = %q", req.Destination)
	}
	if req.ProxyAuth != "spike" {
		t.Fatalf("ProxyAuth = %q", req.ProxyAuth)
	}
	if len(req.X224) != 43 {
		t.Fatalf("X224 = %d байт, ожидалось 43", len(req.X224))
	}
	if req.X224[0] != 0x03 || req.X224[5] != 0xE0 {
		t.Fatalf("X224 не похож на Connection Request: % x", req.X224[:6])
	}
}

func TestResponseRoundTrip(t *testing.T) {
	x224 := []byte{0x03, 0x00, 0x00, 0x13, 0x0E, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x08, 0x00, 0x01, 0x00, 0x00, 0x00}
	chain := [][]byte{{0x30, 0x03, 0x02, 0x01, 0x07}, {0x30, 0x03, 0x02, 0x01, 0x08}}

	out, err := cleanpath.BuildResponse(x224, chain, "192.168.31.38")
	if err != nil {
		t.Fatalf("BuildResponse: %v", err)
	}
	if len(out) == 0 || out[0] != 0x30 {
		t.Fatalf("ответ обязан быть DER SEQUENCE, первый байт %#x", out[0])
	}
	if !bytes.Contains(out, x224) {
		t.Fatal("x224_connection_pdu не найден в ответе")
	}

	got, err := cleanpath.ParseResponseForTest(out)
	if err != nil {
		t.Fatalf("ParseResponseForTest: %v", err)
	}
	if got.Version != cleanpath.Version {
		t.Fatalf("Version = %d, ожидалось %d", got.Version, cleanpath.Version)
	}
	if !bytes.Equal(got.X224, x224) {
		t.Fatalf("X224 не совпал")
	}
	if len(got.CertChain) != 2 {
		t.Fatalf("цепочка из %d сертификатов, ожидалось 2", len(got.CertChain))
	}
	if got.ServerAddr != "192.168.31.38" {
		t.Fatalf("ServerAddr = %q", got.ServerAddr)
	}
}

// Версия обязана быть 3390. Клиент её сверяет, и по несовпадению закроет
// соединение с RDCleanPath-ошибкой вместо внятной диагностики.
func TestResponseCarriesVersion3390(t *testing.T) {
	if cleanpath.Version != 3390 {
		t.Fatalf("Version = %d", cleanpath.Version)
	}
	out, err := cleanpath.BuildResponse([]byte{0x03, 0x00}, [][]byte{{0x30, 0x00}}, "h")
	if err != nil {
		t.Fatalf("BuildResponse: %v", err)
	}
	if !bytes.Contains(out, []byte{0x02, 0x02, 0x0D, 0x3E}) { // INTEGER 3390
		t.Fatalf("INTEGER 3390 не найден в % x", out)
	}
}

func TestParseRequestRejectsTrailingGarbage(t *testing.T) {
	raw := append(liveRequest(t), 0xAA)
	_, err := cleanpath.ParseRequest(raw)
	if err == nil {
		t.Fatal("мусор после SEQUENCE обязан быть ошибкой: дальше по этому потоку пойдут байты RDP, и рассинхрон здесь стоит сессии")
	}
}

func TestParseRequestRejectsGarbage(t *testing.T) {
	cases := map[string][]byte{
		"пусто":       {},
		"не DER":      {0xFF, 0xFF, 0xFF},
		"обрыв":       {0x30, 0x50, 0xa0},
		"пустой SEQ":  {0x30, 0x00},
		"чужая схема": {0x30, 0x03, 0x02, 0x01, 0x07},
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := cleanpath.ParseRequest(b); err == nil {
				t.Fatal("обязана быть ошибка")
			}
		})
	}
}

// Версия запроса, отличная от нашей, — отказ. Продолжить значило бы разбирать
// структуру по схеме, о которой мы не договаривались.
func TestParseRequestRejectsForeignVersion(t *testing.T) {
	raw := liveRequest(t)
	// Версия лежит как INTEGER 0x0D3E внутри explicit [0] в самом начале.
	i := bytes.Index(raw, []byte{0x02, 0x02, 0x0D, 0x3E})
	if i < 0 {
		t.Fatal("версия не найдена в векторе")
	}
	bad := append([]byte(nil), raw...)
	bad[i+3] = 0x3F // 3391
	if _, err := cleanpath.ParseRequest(bad); err == nil {
		t.Fatal("чужая версия обязана быть отказом")
	}
}

// Запрос без x224_connection_pdu — это VMConnect-режим, которого мы не
// поддерживаем. Отказ обязан быть распознаваемым, чтобы вызывающий мог
// объяснить пользователю, что именно не поддержано.
func TestParseRequestRejectsMissingX224(t *testing.T) {
	raw, err := cleanpath.BuildRequestForTest(nil, "h:3389", "tok")
	if err != nil {
		t.Fatalf("BuildRequestForTest: %v", err)
	}
	_, err = cleanpath.ParseRequest(raw)
	if !errors.Is(err, cleanpath.ErrNoX224) {
		t.Fatalf("err = %v, ожидалась ErrNoX224", err)
	}
}

// Ошибка клиенту — единственный способ сказать браузеру, почему не вышло.
func TestBuildErrorResponse(t *testing.T) {
	out, err := cleanpath.BuildError(cleanpath.ErrCodeGeneral)
	if err != nil {
		t.Fatalf("BuildError: %v", err)
	}
	if len(out) == 0 || out[0] != 0x30 {
		t.Fatalf("ошибка обязана быть DER SEQUENCE: % x", out)
	}
	if bytes.Contains(out, []byte{0x02, 0x02, 0x0D, 0x3E}) == false {
		t.Fatal("ответ с ошибкой обязан нести версию")
	}
}
