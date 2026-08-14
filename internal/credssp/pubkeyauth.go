package credssp

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
)

// Версии CredSSP.
//
// Начиная с версии 5 привязка к открытому ключу считается через хеш с
// клиентским nonce — это исправление CVE-2018-0886, из-за которого прежняя
// схема позволяла переиспользовать перехваченный обмен. Мы объявляем 6 и
// требуем от сервера не меньше 5.
const (
	// Version — что объявляем мы.
	Version = 6
	// MinServerVersion — ниже этого сервер считается непропатченным.
	//
	// Отказ здесь сознательный. Поддержать старую схему технически несложно, но
	// это означало бы молча соглашаться на известную уязвимость ради серверов,
	// не обновлявшихся с 2018 года. Пользователь просил защищённое соединение.
	MinServerVersion = 5
	// NonceLen — длина клиентского nonce (MS-CSSP 3.1.5).
	NonceLen = 32
)

// Магические строки привязки (MS-CSSP 3.1.5). Завершающий нулевой байт входит
// в хешируемые данные — без него сойдётся всё, кроме результата.
var (
	clientToServerMagic = []byte("CredSSP Client-To-Server Binding Hash\x00")
	serverToClientMagic = []byte("CredSSP Server-To-Client Binding Hash\x00")
)

// ErrServerTooOld — сервер объявил версию CredSSP без исправления CVE-2018-0886.
var ErrServerTooOld = errors.New("credssp: сервер объявил версию CredSSP ниже 5 (без исправления CVE-2018-0886)")

// ErrPubKeyMismatch — сервер не подтвердил владение ключом, к которому мы
// привязались. Это либо перехват, либо расхождение реализаций; в обоих случаях
// продолжать нельзя.
var ErrPubKeyMismatch = errors.New("credssp: сервер не подтвердил привязку к своему открытому ключу")

// NewNonce возвращает клиентский nonce.
func NewNonce() ([]byte, error) {
	n := make([]byte, NonceLen)
	if _, err := rand.Read(n); err != nil {
		return nil, fmt.Errorf("credssp: nonce: %w", err)
	}
	return n, nil
}

// CheckServerVersion проверяет объявленную сервером версию.
func CheckServerVersion(v int) error {
	if v < MinServerVersion {
		return fmt.Errorf("%w: объявлена версия %d", ErrServerTooOld, v)
	}
	return nil
}

// ClientServerHash считает значение, которое клиент кладёт в pubKeyAuth.
//
// pubKey — содержимое BIT STRING из SubjectPublicKeyInfo сертификата сервера
// (trust.SubjectPublicKey). Не SPKI целиком: материал задаёт сервер, а не мы.
func ClientServerHash(nonce, pubKey []byte) []byte {
	return bindingHash(clientToServerMagic, nonce, pubKey)
}

// ServerClientHash считает значение, которое обязан вернуть сервер.
func ServerClientHash(nonce, pubKey []byte) []byte {
	return bindingHash(serverToClientMagic, nonce, pubKey)
}

func bindingHash(magic, nonce, pubKey []byte) []byte {
	h := sha256.New()
	h.Write(magic)
	h.Write(nonce)
	h.Write(pubKey)
	return h.Sum(nil)
}

// VerifyServerHash сверяет то, что вернул сервер, с тем, что он обязан был
// вернуть.
//
// Сравнение постоянного времени: значение выводится из секрета сессии NTLM, и
// утечка времени сравнения — это утечка о секрете.
func VerifyServerHash(got, nonce, pubKey []byte) error {
	want := ServerClientHash(nonce, pubKey)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrPubKeyMismatch
	}
	return nil
}
