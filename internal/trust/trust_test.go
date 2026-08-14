package trust_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
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

// certWithKey выпускает сертификат на ЗАДАННОМ ключе — иначе перевыпуск не
// отличить от смены сервера.
func certWithKey(t *testing.T, key *ecdsa.PrivateKey, cn string, validFor time.Duration) []byte {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(validFor),
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

// Главное свойство пакета после переезда решения в ядро: материал, который мы
// присылаем, и отпечаток, который ядро по нему считает, обязаны сходиться.
//
// Ядро считает SHA-256 от присланных байтов и кодирует base64 без паддинга
// (domain.PeerFingerprint). Если наша Fingerprint однажды начнёт считать от
// другого материала, пользователь увидит в диалоге одно значение, а в журнале
// плагина — другое, и сверить их будет нечем.
func TestFingerprintMatchesWhatTheCoreComputesOverPeerMaterial(t *testing.T) {
	der := certDER(t, "host")
	material, err := trust.PeerMaterial(der)
	if err != nil {
		t.Fatalf("PeerMaterial: %v", err)
	}
	sum := sha256.Sum256(material)
	want := "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])

	got, err := trust.Fingerprint(der)
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}
	if got != want {
		t.Fatalf("отпечаток плагина %q, ядро посчитает %q — пользователю нечего будет сверить", got, want)
	}
}

// Материал доверия — SubjectPublicKeyInfo, а НЕ содержимое BIT STRING. Это
// разные значения, и путаница между ними уже однажды стоила живой отладки:
// привязка CredSSP берёт второе, доверие — первое.
func TestPeerMaterialIsTheWholeSPKINotTheBareKey(t *testing.T) {
	der := certDER(t, "host")
	material, err := trust.PeerMaterial(der)
	if err != nil {
		t.Fatalf("PeerMaterial: %v", err)
	}
	bare, err := trust.SubjectPublicKey(der)
	if err != nil {
		t.Fatalf("SubjectPublicKey: %v", err)
	}
	if bytes.Equal(material, bare) {
		t.Fatal("материал доверия совпал с материалом привязки — это разные значения")
	}
	// SPKI содержит ключ внутри себя вместе с идентификатором алгоритма.
	if !bytes.Contains(material, bare) {
		t.Fatal("SPKI не содержит открытого ключа — это не SubjectPublicKeyInfo")
	}
}

// Перевыпуск сертификата с тем же ключом обязан давать тот же материал.
// Иначе продление срока выглядело бы как смена сервера, и пользователь
// привык бы подтверждать «смену» не глядя — ровно то, от чего защита нужна.
func TestPeerMaterialSurvivesCertificateRenewalWithTheSameKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ключ: %v", err)
	}
	first := certWithKey(t, key, "host", time.Hour)
	renewed := certWithKey(t, key, "host", 10*time.Hour)
	if bytes.Equal(first, renewed) {
		t.Fatal("сертификаты совпали — тест ничего не проверяет")
	}

	a, err := trust.PeerMaterial(first)
	if err != nil {
		t.Fatalf("PeerMaterial: %v", err)
	}
	b, err := trust.PeerMaterial(renewed)
	if err != nil {
		t.Fatalf("PeerMaterial: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("перевыпуск с тем же ключом сменил материал доверия")
	}
}

// Смена ключа обязана менять материал: иначе доверие к одному серверу
// молча распространилось бы на другой.
func TestPeerMaterialDiffersWhenTheKeyDiffers(t *testing.T) {
	a, err := trust.PeerMaterial(certDER(t, "one"))
	if err != nil {
		t.Fatalf("PeerMaterial: %v", err)
	}
	b, err := trust.PeerMaterial(certDER(t, "two"))
	if err != nil {
		t.Fatalf("PeerMaterial: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("разные ключи дали одинаковый материал доверия")
	}
}

func TestPeerMaterialRejectsGarbage(t *testing.T) {
	if _, err := trust.PeerMaterial([]byte{0xFF, 0xFF}); err == nil {
		t.Fatal("мусор вместо сертификата обязан быть ошибкой")
	}
	if _, err := trust.PeerMaterial(nil); err == nil {
		t.Fatal("пустой сертификат обязан быть ошибкой")
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
