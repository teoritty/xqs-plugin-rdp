// Package cleanpath кодирует и декодирует RDCleanPath PDU — протокол, по
// которому ironrdp-web разговаривает с прокси.
//
// Схема взята из ironrdp-rdcleanpath (crates/ironrdp-rdcleanpath/src/lib.rs):
// SEQUENCE с context-specific EXPLICIT тегами.
//
//	[0] version            INTEGER
//	[1] error              RDCleanPathErr   OPTIONAL
//	[2] destination        UTF8String       OPTIONAL
//	[3] proxy_auth         UTF8String       OPTIONAL
//	[4] server_auth        UTF8String       OPTIONAL
//	[5] preconnection_blob UTF8String       OPTIONAL
//	[6] x224_connection_pdu OCTET STRING    OPTIONAL
//	[7] server_cert_chain  SEQUENCE OF OCTET STRING OPTIONAL
//	[9] server_addr        UTF8String       OPTIONAL
//
// Пакет чистый: ни net, ни io — только байты.
package cleanpath

// Version — RDCleanPath VERSION_1 (BASE_VERSION 3389 + 1).
//
// Клиент сверяет её, и расхождение он покажет как RDCleanPath-ошибку без
// подробностей — то есть отладить это будет нечем. Поэтому версия проверяется
// и на приёме тоже.
const Version = 3390

// Коды ошибок RDCleanPathErr, которые мы отправляем клиенту.
const (
	ErrCodeGeneral = 0
)

// requestPDU и responsePDU — две проекции одной схемы.
//
// Разделение намеренное: запрос и ответ несут разные подмножества полей, и
// одна общая структура с десятком optional-полей позволяла бы собрать ответ с
// полем destination, которого в ответе быть не должно. Тип, которым нельзя
// выразить неверное сообщение, дешевле проверки, что сообщение верное.
type requestPDU struct {
	Version           int    `asn1:"explicit,tag:0"`
	Destination       string `asn1:"explicit,tag:2,utf8,optional"`
	ProxyAuth         string `asn1:"explicit,tag:3,utf8,optional"`
	ServerAuth        string `asn1:"explicit,tag:4,utf8,optional"`
	PreconnectionBlob string `asn1:"explicit,tag:5,utf8,optional"`
	X224              []byte `asn1:"explicit,tag:6,optional"`
}

type responsePDU struct {
	Version    int      `asn1:"explicit,tag:0"`
	X224       []byte   `asn1:"explicit,tag:6,omitempty"`
	CertChain  [][]byte `asn1:"explicit,tag:7,omitempty"`
	ServerAddr string   `asn1:"explicit,tag:9,utf8,omitempty"`
}

type errPDU struct {
	Version int      `asn1:"explicit,tag:0"`
	Err     errorSeq `asn1:"explicit,tag:1"`
}

// errorSeq несёт только код. Необязательные поля (httpStatusCode,
// wsaLastError, tlsAlertCode) в схеме есть, но encoding/asn1 закодировал бы их
// нулём вместо пропуска, а ноль в них — не «нет значения», а конкретный
// неверный статус. Отправлять то, чего мы не знаем, хуже, чем не отправлять.
type errorSeq struct {
	ErrorCode int `asn1:"explicit,tag:0"`
}
