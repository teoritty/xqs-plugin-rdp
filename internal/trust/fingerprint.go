// Package trust решает единственный вопрос: тот ли это сервер.
//
// Вопрос не косметический. Привязка к открытому ключу в CredSSP защищает
// разговор с тем, с кем мы фактически говорим. Если не проверять, кто это,
// привязка честно защитит разговор с атакующим, и вся конструкция NLA станет
// украшением. Поэтому пакет отказывает по умолчанию.
package trust

import (
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
)

// fingerprintPrefix называет алгоритм прямо в значении.
//
// Голый base64 через год невозможно интерпретировать: неизвестно, чего это
// хеш и чем посчитан. Префикс делает значение самоописывающим и оставляет путь
// к смене алгоритма без молчаливой поломки старых пинов.
const fingerprintPrefix = "SHA256:"

// Fingerprint считает отпечаток по SubjectPublicKeyInfo сертификата.
//
// Считается именно ключ, а не сертификат целиком: сервер вправе перевыпустить
// сертификат с тем же ключом (продление срока), и требовать в этом случае
// нового подтверждения от пользователя значило бы приучать его подтверждать
// смену ключа не глядя — ровно то, от чего вся эта проверка защищает.
func Fingerprint(certDER []byte) (string, error) {
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return "", fmt.Errorf("trust: сертификат не разбирается: %w", err)
	}
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return fingerprintPrefix + base64.RawStdEncoding.EncodeToString(sum[:]), nil
}

// SubjectPublicKey возвращает содержимое BIT STRING из SubjectPublicKeyInfo —
// то есть сам открытый ключ, без обёртки с идентификатором алгоритма.
//
// Это НЕ то же самое, что материал отпечатка выше, и разница здесь
// намеренная, а не небрежность:
//
//   - Fingerprint считается от SubjectPublicKeyInfo целиком. Так принято при
//     пиннинге (RFC 6698, HPKP): включённый идентификатор алгоритма исключает
//     совпадение ключей разных алгоритмов.
//   - CredSSP привязывается именно к содержимому BIT STRING. Это не наш выбор,
//     а то, что делает сервер: так берут ключ и IronRDP
//     (subject_public_key.as_bytes()), и FreeRDP (i2d_PublicKey). Посчитать
//     привязку от другого материала — значит не сойтись с сервером.
//
// Функция живёт здесь, а не в credssp, чтобы разбор сертификатов остался в
// одном пакете: credssp получает готовые байты и про x509 не знает.
func SubjectPublicKey(certDER []byte) ([]byte, error) {
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("trust: сертификат не разбирается: %w", err)
	}
	var spki struct {
		Algorithm pkix.AlgorithmIdentifier
		PublicKey asn1.BitString
	}
	rest, err := asn1.Unmarshal(cert.RawSubjectPublicKeyInfo, &spki)
	if err != nil {
		return nil, fmt.Errorf("trust: SubjectPublicKeyInfo не разбирается: %w", err)
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("trust: после SubjectPublicKeyInfo осталось %d байт", len(rest))
	}
	if len(spki.PublicKey.Bytes) == 0 {
		return nil, errors.New("trust: пустой открытый ключ в сертификате")
	}
	return spki.PublicKey.Bytes, nil
}
