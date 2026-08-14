package trust_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"xqs-plugin-rdp/internal/trust"
)

func certDER(t *testing.T, cn string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ключ: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("сертификат: %v", err)
	}
	return der
}

func TestFingerprintIsStableAndCanonical(t *testing.T) {
	der := certDER(t, "host")
	a, err := trust.Fingerprint(der)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	b, err := trust.Fingerprint(der)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if a != b {
		t.Fatalf("отпечаток не стабилен: %q vs %q", a, b)
	}
	if !strings.HasPrefix(a, "SHA256:") {
		t.Fatalf("отпечаток %q без префикса алгоритма — через год никто не вспомнит, чем он посчитан", a)
	}
	// 32 байта в base64 без паддинга — 43 символа.
	if got := len(a) - len("SHA256:"); got != 43 {
		t.Fatalf("тело отпечатка %d символов, ожидалось 43", got)
	}
}

func TestFingerprintDiffersBetweenCerts(t *testing.T) {
	a, err := trust.Fingerprint(certDER(t, "one"))
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	b, err := trust.Fingerprint(certDER(t, "two"))
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if a == b {
		t.Fatal("разные сертификаты дали одинаковый отпечаток")
	}
}

func TestFingerprintRejectsGarbage(t *testing.T) {
	if _, err := trust.Fingerprint([]byte{0xFF, 0xFF}); err == nil {
		t.Fatal("мусор вместо сертификата обязан быть ошибкой")
	}
	if _, err := trust.Fingerprint(nil); err == nil {
		t.Fatal("пустой сертификат обязан быть ошибкой")
	}
}

// Главное свойство пакета: пустой пин — отказ, а не «доверяем».
// Fail-open здесь означал бы, что вся конструкция CredSSP защищает разговор
// неизвестно с кем.
func TestEmptyPinIsRefused(t *testing.T) {
	der := certDER(t, "host")
	for _, pin := range []string{"", "   ", "\t\n"} {
		err := trust.Verify(der, pin)
		if err == nil {
			t.Fatalf("пустой пин %q принят — политика обязана отказывать по умолчанию", pin)
		}
		var unk *trust.UnknownServerError
		if !errors.As(err, &unk) {
			t.Fatalf("err типа %T, ожидался *trust.UnknownServerError", err)
		}
		// Пользователь обязан увидеть отпечаток: иначе ему нечего вписать в поле.
		if !strings.Contains(unk.Error(), unk.Fingerprint) {
			t.Fatalf("текст ошибки не содержит отпечатка: %q", unk.Error())
		}
		if !strings.HasPrefix(unk.Fingerprint, "SHA256:") {
			t.Fatalf("отпечаток в ошибке без префикса: %q", unk.Fingerprint)
		}
	}
}

func TestMatchingPinIsAccepted(t *testing.T) {
	der := certDER(t, "host")
	fp, err := trust.Fingerprint(der)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if err := trust.Verify(der, fp); err != nil {
		t.Fatalf("совпавший пин отвергнут: %v", err)
	}
}

// Пробелы вокруг вставленного значения — норма при копировании из текста
// ошибки, и отвергать из-за них было бы придиркой, а не строгостью.
func TestPinIsTrimmedButNotOtherwiseNormalised(t *testing.T) {
	der := certDER(t, "host")
	fp, err := trust.Fingerprint(der)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if err := trust.Verify(der, "  "+fp+"\n"); err != nil {
		t.Fatalf("пин с пробелами отвергнут: %v", err)
	}
	// А вот регистр менять нельзя: base64 регистрозависим, и «починить»
	// отличающийся регистр значило бы принять другой отпечаток.
	if err := trust.Verify(der, strings.ToUpper(fp)); err == nil {
		t.Fatal("пин в другом регистре принят — base64 регистрозависим")
	}
}

// Смена ключа сервера обязана быть громкой и отличимой от «пина ещё нет»:
// первое подозрительно, второе — обычное первое подключение.
func TestChangedKeyIsDistinctFromMissingPin(t *testing.T) {
	oldFP, err := trust.Fingerprint(certDER(t, "one"))
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	newDER := certDER(t, "two")

	err = trust.Verify(newDER, oldFP)
	if err == nil {
		t.Fatal("несовпадение обязано быть отказом")
	}
	var mism *trust.MismatchError
	if !errors.As(err, &mism) {
		t.Fatalf("err типа %T, ожидался *trust.MismatchError", err)
	}
	var unk *trust.UnknownServerError
	if errors.As(err, &unk) {
		t.Fatal("несовпадение не должно выглядеть как отсутствие пина: это разные события")
	}
	if !strings.Contains(mism.Error(), mism.Expected) || !strings.Contains(mism.Error(), mism.Got) {
		t.Fatalf("текст не показывает оба отпечатка: %q", mism.Error())
	}
}

// Сверяется первый сертификат цепочки — это сертификат сервера. Пустая цепочка
// не «доверяем по умолчанию», а ошибка.
func TestVerifyChainUsesLeafAndRejectsEmpty(t *testing.T) {
	leaf := certDER(t, "leaf")
	fp, err := trust.Fingerprint(leaf)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	chain := [][]byte{leaf, certDER(t, "issuer")}
	if err := trust.VerifyChain(chain, fp); err != nil {
		t.Fatalf("цепочка с верным листом отвергнута: %v", err)
	}
	if err := trust.VerifyChain(nil, fp); err == nil {
		t.Fatal("пустая цепочка обязана быть ошибкой")
	}
	// Пин от второго сертификата не должен подходить: доверяем листу, а не
	// «чему-нибудь из цепочки».
	issuerFP, err := trust.Fingerprint(chain[1])
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if err := trust.VerifyChain(chain, issuerFP); err == nil {
		t.Fatal("пин промежуточного сертификата принят вместо листа")
	}
}

// SubjectPublicKey обязан отдавать содержимое BIT STRING, а не SPKI целиком:
// именно к нему привязывается CredSSP, и подмена материала на «похожий»
// проявится как обрыв TLS сервером без объяснений.
func TestSubjectPublicKeyIsBitStringContentsNotWholeSPKI(t *testing.T) {
	der := certDER(t, "host")
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	got, err := trust.SubjectPublicKey(der)
	if err != nil {
		t.Fatalf("SubjectPublicKey: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("пустой ключ")
	}
	if bytes.Equal(got, cert.RawSubjectPublicKeyInfo) {
		t.Fatal("вернулся SubjectPublicKeyInfo целиком — это другой материал")
	}
	if len(got) >= len(cert.RawSubjectPublicKeyInfo) {
		t.Fatalf("ключ (%d) не короче SPKI (%d)", len(got), len(cert.RawSubjectPublicKeyInfo))
	}
	// Он обязан лежать внутри SPKI как подпоследовательность.
	if !bytes.Contains(cert.RawSubjectPublicKeyInfo, got) {
		t.Fatal("возвращённые байты не найдены внутри SubjectPublicKeyInfo")
	}
	// Для P-256 это несжатая точка: 0x04 и 64 байта координат.
	if got[0] != 0x04 || len(got) != 65 {
		t.Fatalf("для P-256 ожидалась несжатая точка 65 байт, получено %d байт с префиксом %#x", len(got), got[0])
	}
}

func TestSubjectPublicKeyRejectsGarbage(t *testing.T) {
	if _, err := trust.SubjectPublicKey([]byte{0xFF, 0xFF}); err == nil {
		t.Fatal("мусор обязан быть ошибкой")
	}
}
