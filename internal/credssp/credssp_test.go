package credssp_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/asn1"
	"errors"
	"testing"

	"xqs-plugin-rdp/internal/credssp"
)

func TestTSRequestRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		req  credssp.TSRequest
	}{
		{"только версия", credssp.TSRequest{Version: credssp.Version}},
		{"с negoToken", credssp.TSRequest{Version: 6, NegoToken: []byte("NTLMSSP\x00\x01")}},
		{"с pubKeyAuth и nonce", credssp.TSRequest{Version: 6, PubKeyAuth: []byte{1, 2, 3}, ClientNonce: bytes.Repeat([]byte{7}, credssp.NonceLen)}},
		{"с authInfo", credssp.TSRequest{Version: 6, AuthInfo: []byte{9, 9, 9}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := tc.req.Marshal()
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if raw[0] != 0x30 {
				t.Fatalf("TSRequest обязан быть DER SEQUENCE, первый байт %#x", raw[0])
			}
			got, err := credssp.UnmarshalTSRequest(raw)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if got.Version != tc.req.Version ||
				!bytes.Equal(got.NegoToken, tc.req.NegoToken) ||
				!bytes.Equal(got.PubKeyAuth, tc.req.PubKeyAuth) ||
				!bytes.Equal(got.AuthInfo, tc.req.AuthInfo) ||
				!bytes.Equal(got.ClientNonce, tc.req.ClientNonce) {
				t.Fatalf("round-trip:\n got %+v\nwant %+v", got, tc.req)
			}
		})
	}
}

// negoToken обязан лежать внутри вложенного SEQUENCE OF, а не прямо в [1].
// Это ровно то место, где реализации расходятся, и расхождение выглядит как
// «сервер молча закрыл соединение».
func TestNegoTokenIsWrappedInSequenceOf(t *testing.T) {
	raw, err := credssp.TSRequest{Version: 6, NegoToken: []byte{0xAA, 0xBB}}.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var probe struct {
		Version    int `asn1:"explicit,tag:0"`
		NegoTokens []struct {
			Token []byte `asn1:"explicit,tag:0"`
		} `asn1:"explicit,tag:1,omitempty"`
	}
	if _, err := asn1.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("структура не соответствует NegoData: %v", err)
	}
	if len(probe.NegoTokens) != 1 || !bytes.Equal(probe.NegoTokens[0].Token, []byte{0xAA, 0xBB}) {
		t.Fatalf("negoTokens = %+v", probe.NegoTokens)
	}
}

func TestUnmarshalRejectsTrailingBytes(t *testing.T) {
	raw, err := credssp.TSRequest{Version: 6}.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if _, err := credssp.UnmarshalTSRequest(append(raw, 0x00)); err == nil {
		t.Fatal("хвост после TSRequest обязан быть ошибкой: он означает, что границу сообщения мы определили неверно")
	}
}

func TestUnmarshalRejectsGarbage(t *testing.T) {
	for _, b := range [][]byte{nil, {0xFF}, {0x30, 0x80}} {
		if _, err := credssp.UnmarshalTSRequest(b); err == nil {
			t.Fatalf("мусор % x принят", b)
		}
	}
}

// Версия ниже 5 — сервер без исправления CVE-2018-0886.
func TestServerVersionBelowFiveIsRefused(t *testing.T) {
	for _, v := range []int{0, 2, 3, 4} {
		err := credssp.CheckServerVersion(v)
		if err == nil {
			t.Fatalf("версия %d принята", v)
		}
		if !errors.Is(err, credssp.ErrServerTooOld) {
			t.Fatalf("версия %d: err = %v, ожидалась ErrServerTooOld", v, err)
		}
	}
	for _, v := range []int{5, 6, 7} {
		if err := credssp.CheckServerVersion(v); err != nil {
			t.Fatalf("версия %d отвергнута: %v", v, err)
		}
	}
}

// Магические строки и порядок конкатенации проверяются против независимо
// посчитанного значения. Это единственная часть привязки, которую можно
// проверить без сервера, и ошибка в ней не проявилась бы как поломка — она
// проявилась бы как «сервер не принимает пароль».
func TestBindingHashesMatchIndependentComputation(t *testing.T) {
	nonce := bytes.Repeat([]byte{0x42}, credssp.NonceLen)
	pubKey := []byte{0x30, 0x59, 0x30, 0x13}

	wantClient := sha256.Sum256(append(append([]byte("CredSSP Client-To-Server Binding Hash\x00"), nonce...), pubKey...))
	if got := credssp.ClientServerHash(nonce, pubKey); !bytes.Equal(got, wantClient[:]) {
		t.Fatalf("ClientServerHash\n got % x\nwant % x", got, wantClient)
	}

	wantServer := sha256.Sum256(append(append([]byte("CredSSP Server-To-Client Binding Hash\x00"), nonce...), pubKey...))
	if got := credssp.ServerClientHash(nonce, pubKey); !bytes.Equal(got, wantServer[:]) {
		t.Fatalf("ServerClientHash\n got % x\nwant % x", got, wantServer)
	}

	// Направления обязаны различаться: если бы они совпадали, перехватчик мог
	// бы вернуть клиенту его же значение.
	if bytes.Equal(wantClient[:], wantServer[:]) {
		t.Fatal("хеши направлений совпали")
	}
}

// Завершающий нулевой байт входит в хешируемые данные. Без него сойдётся всё,
// кроме результата, — и отладить это будет нечем.
func TestBindingMagicIncludesTrailingNUL(t *testing.T) {
	nonce := bytes.Repeat([]byte{1}, credssp.NonceLen)
	pubKey := []byte{2, 3}
	withoutNUL := sha256.Sum256(append(append([]byte("CredSSP Client-To-Server Binding Hash"), nonce...), pubKey...))
	if bytes.Equal(credssp.ClientServerHash(nonce, pubKey), withoutNUL[:]) {
		t.Fatal("нулевой байт не входит в хеш")
	}
}

func TestVerifyServerHash(t *testing.T) {
	nonce := bytes.Repeat([]byte{5}, credssp.NonceLen)
	pubKey := []byte{0xAA}
	good := credssp.ServerClientHash(nonce, pubKey)
	if err := credssp.VerifyServerHash(good, nonce, pubKey); err != nil {
		t.Fatalf("верный хеш отвергнут: %v", err)
	}
	// Подсунутый клиентский хеш не должен подойти.
	if err := credssp.VerifyServerHash(credssp.ClientServerHash(nonce, pubKey), nonce, pubKey); !errors.Is(err, credssp.ErrPubKeyMismatch) {
		t.Fatalf("клиентский хеш принят за серверный: %v", err)
	}
	bad := append([]byte(nil), good...)
	bad[0] ^= 1
	if err := credssp.VerifyServerHash(bad, nonce, pubKey); !errors.Is(err, credssp.ErrPubKeyMismatch) {
		t.Fatalf("изменённый хеш принят: %v", err)
	}
	if err := credssp.VerifyServerHash(nil, nonce, pubKey); !errors.Is(err, credssp.ErrPubKeyMismatch) {
		t.Fatal("пустой ответ принят")
	}
}

func TestNonceIsRandomAndCorrectLength(t *testing.T) {
	a, err := credssp.NewNonce()
	if err != nil {
		t.Fatalf("NewNonce: %v", err)
	}
	if len(a) != credssp.NonceLen {
		t.Fatalf("nonce %d байт, ожидалось %d", len(a), credssp.NonceLen)
	}
	b, err := credssp.NewNonce()
	if err != nil {
		t.Fatalf("NewNonce: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("два вызова дали одинаковый nonce")
	}
}

// TSCredentials: структура и UTF-16LE. Это то, что сервер разбирает после
// расшифровки, и ошибка кодировки выглядит как неверный пароль.
func TestEncodeCredentialsStructureAndEncoding(t *testing.T) {
	out, err := credssp.EncodeCredentials("CORP", "admin", []byte("pa$$"))
	if err != nil {
		t.Fatalf("EncodeCredentials: %v", err)
	}
	var outer struct {
		CredType    int    `asn1:"explicit,tag:0"`
		Credentials []byte `asn1:"explicit,tag:1"`
	}
	if _, err := asn1.Unmarshal(out, &outer); err != nil {
		t.Fatalf("TSCredentials не разбирается: %v", err)
	}
	if outer.CredType != 1 {
		t.Fatalf("credType = %d, ожидался 1 (TSPasswordCreds)", outer.CredType)
	}
	var inner struct {
		DomainName []byte `asn1:"explicit,tag:0"`
		UserName   []byte `asn1:"explicit,tag:1"`
		Password   []byte `asn1:"explicit,tag:2"`
	}
	if _, err := asn1.Unmarshal(outer.Credentials, &inner); err != nil {
		t.Fatalf("TSPasswordCreds не разбирается: %v", err)
	}
	want := map[string][]byte{
		"domain":   {'C', 0, 'O', 0, 'R', 0, 'P', 0},
		"user":     {'a', 0, 'd', 0, 'm', 0, 'i', 0, 'n', 0},
		"password": {'p', 0, 'a', 0, '$', 0, '$', 0},
	}
	if !bytes.Equal(inner.DomainName, want["domain"]) {
		t.Fatalf("domain = % x, ожидалось % x", inner.DomainName, want["domain"])
	}
	if !bytes.Equal(inner.UserName, want["user"]) {
		t.Fatalf("user = % x", inner.UserName)
	}
	if !bytes.Equal(inner.Password, want["password"]) {
		t.Fatalf("password = % x, ожидалось % x", inner.Password, want["password"])
	}
}

// Кириллица и не-BMP символы в пароле обязаны кодироваться корректно: пароль
// с суррогатной парой, закодированный неверно, даёт «неверный пароль» без
// единого намёка на причину.
func TestEncodeCredentialsHandlesNonASCII(t *testing.T) {
	out, err := credssp.EncodeCredentials("", "пользователь", []byte("па😀роль"))
	if err != nil {
		t.Fatalf("EncodeCredentials: %v", err)
	}
	var outer struct {
		CredType    int    `asn1:"explicit,tag:0"`
		Credentials []byte `asn1:"explicit,tag:1"`
	}
	if _, err := asn1.Unmarshal(out, &outer); err != nil {
		t.Fatalf("TSCredentials не разбирается: %v", err)
	}
	var inner struct {
		DomainName []byte `asn1:"explicit,tag:0"`
		UserName   []byte `asn1:"explicit,tag:1"`
		Password   []byte `asn1:"explicit,tag:2"`
	}
	if _, err := asn1.Unmarshal(outer.Credentials, &inner); err != nil {
		t.Fatalf("TSPasswordCreds не разбирается: %v", err)
	}
	// "па😀роль": п а <суррогатная пара> р о л ь = 6 BMP + 2 = 8 кодовых единиц.
	if len(inner.Password) != 8*2 {
		t.Fatalf("пароль %d байт, ожидалось 16 (8 кодовых единиц UTF-16)", len(inner.Password))
	}
	// Суррогатная пара для U+1F600.
	if !bytes.Contains(inner.Password, []byte{0x3D, 0xD8, 0x00, 0xDE}) {
		t.Fatalf("суррогатная пара закодирована неверно: % x", inner.Password)
	}
	if !bytes.Equal(inner.DomainName, []byte{}) && len(inner.DomainName) != 0 {
		t.Fatalf("пустой домен закодирован как % x", inner.DomainName)
	}
}

// EncodeCredentials не должен изменять переданный пароль: владелец зануляет его
// сам, и порча буфера сломала бы это молча.
func TestEncodeCredentialsDoesNotMutatePassword(t *testing.T) {
	pw := []byte("secret")
	orig := append([]byte(nil), pw...)
	if _, err := credssp.EncodeCredentials("", "u", pw); err != nil {
		t.Fatalf("EncodeCredentials: %v", err)
	}
	if !bytes.Equal(pw, orig) {
		t.Fatalf("пароль изменён: % x -> % x", orig, pw)
	}
}
