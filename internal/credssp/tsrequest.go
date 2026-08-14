// Package credssp реализует клиентскую сторону MS-CSSP — того самого NLA,
// который RDP-сервер требует до всякой графики.
//
// Пакет чистый: он не открывает соединений и не знает про TLS. Обмен идёт
// через io.ReadWriter, который ему дают снаружи. Это то, что позволяет
// проверять структуру на векторах, а живую цепочку — на стенде.
//
// Класс дефекта здесь особый: ошибка в привязке к ключу или в упаковке
// учётных данных даёт не «не работает», а молча ослабленную аутентификацию.
// Поэтому всё, что можно проверить без сервера, проверяется без сервера.
package credssp

import (
	"encoding/asn1"
	"fmt"
)

// TSRequest (MS-CSSP 2.2.1):
//
//	TSRequest ::= SEQUENCE {
//	    version     [0] INTEGER,
//	    negoTokens  [1] NegoData      OPTIONAL,
//	    authInfo    [2] OCTET STRING  OPTIONAL,
//	    pubKeyAuth  [3] OCTET STRING  OPTIONAL,
//	    errorCode   [4] INTEGER       OPTIONAL,
//	    clientNonce [5] OCTET STRING  OPTIONAL
//	}
//
//	NegoData ::= SEQUENCE OF SEQUENCE { negoToken [0] OCTET STRING }
type TSRequest struct {
	Version     int
	NegoToken   []byte
	AuthInfo    []byte
	PubKeyAuth  []byte
	ErrorCode   int
	HasError    bool
	ClientNonce []byte
}

type negoItem struct {
	Token []byte `asn1:"explicit,tag:0"`
}

// Две проекции на теги вместо одной — по той же причине, что в
// internal/cleanpath: клиент и сервер отправляют разные подмножества полей.
//
// Конкретно: errorCode отправляет только сервер. В общей структуре Go-ASN.1
// кодировал бы его нулём в каждом нашем сообщении (omitempty на сериализации
// работает лишь для срезов), то есть мы бы в каждом запросе сообщали серверу
// несуществующий статус.
//
// На разборе, наоборот, все поля обязаны быть optional: omitempty на приёме не
// действует, и без optional декодер требует поле, которого в сообщении нет.
type tsRequestClientDER struct {
	Version     int        `asn1:"explicit,tag:0"`
	NegoTokens  []negoItem `asn1:"explicit,tag:1,omitempty"`
	AuthInfo    []byte     `asn1:"explicit,tag:2,omitempty"`
	PubKeyAuth  []byte     `asn1:"explicit,tag:3,omitempty"`
	ClientNonce []byte     `asn1:"explicit,tag:5,omitempty"`
}

type tsRequestWireDER struct {
	Version     int        `asn1:"explicit,tag:0"`
	NegoTokens  []negoItem `asn1:"explicit,tag:1,optional,omitempty"`
	AuthInfo    []byte     `asn1:"explicit,tag:2,optional,omitempty"`
	PubKeyAuth  []byte     `asn1:"explicit,tag:3,optional,omitempty"`
	ErrorCode   int        `asn1:"explicit,tag:4,optional"`
	ClientNonce []byte     `asn1:"explicit,tag:5,optional,omitempty"`
}

// Marshal кодирует TSRequest в DER.
func (r TSRequest) Marshal() ([]byte, error) {
	d := tsRequestClientDER{
		Version:     r.Version,
		AuthInfo:    r.AuthInfo,
		PubKeyAuth:  r.PubKeyAuth,
		ClientNonce: r.ClientNonce,
	}
	if len(r.NegoToken) > 0 {
		d.NegoTokens = []negoItem{{Token: r.NegoToken}}
	}
	out, err := asn1.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("credssp: сборка TSRequest: %w", err)
	}
	return out, nil
}

// UnmarshalTSRequest разбирает TSRequest.
//
// Хвост после SEQUENCE — ошибка: TSRequest идёт по потоку внутри TLS вплотную
// к следующим данным, и лишние байты означают, что мы неверно определили
// границу сообщения.
func UnmarshalTSRequest(b []byte) (TSRequest, error) {
	var d tsRequestWireDER
	rest, err := asn1.Unmarshal(b, &d)
	if err != nil {
		return TSRequest{}, fmt.Errorf("credssp: разбор TSRequest: %w", err)
	}
	if len(rest) != 0 {
		return TSRequest{}, fmt.Errorf("credssp: после TSRequest осталось %d байт", len(rest))
	}
	r := TSRequest{
		Version:     d.Version,
		AuthInfo:    d.AuthInfo,
		PubKeyAuth:  d.PubKeyAuth,
		ClientNonce: d.ClientNonce,
		ErrorCode:   d.ErrorCode,
		// Отсутствующий errorCode и errorCode = 0 неразличимы после разбора.
		// На практике это не мешает: NTSTATUS 0 — это успех, и сервер шлёт
		// поле только при отказе. Но полагаться на «поле есть» нельзя, поэтому
		// признаком ошибки считается именно ненулевое значение.
		HasError: d.ErrorCode != 0,
	}
	// negoTokens — SEQUENCE OF, но в NLA всегда ровно один элемент. Больше
	// одного означает, что мы читаем не то, что думаем.
	switch len(d.NegoTokens) {
	case 0:
	case 1:
		r.NegoToken = d.NegoTokens[0].Token
	default:
		return TSRequest{}, fmt.Errorf("credssp: в negoTokens %d элементов, ожидался один", len(d.NegoTokens))
	}
	return r, nil
}
