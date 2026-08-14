// Package x224 разбирает и собирает X.224 Connection Request/Confirm вместе с
// вложенной в них структурой RDP Negotiation (MS-RDPBCGR 2.2.1.1–2.2.1.2).
//
// Пакет чистый: он не импортирует net и работает только с байтовыми срезами.
// Именно это позволяет проверять его табличными тестами на снятых с проводов
// байтах, без сети и без сервера.
package x224

import (
	"encoding/binary"
	"fmt"
)

// Протоколы безопасности (MS-RDPBCGR 2.2.1.1.1).
const (
	ProtocolRDP      uint32 = 0x00000000 // standard RDP security
	ProtocolSSL      uint32 = 0x00000001
	ProtocolHybrid   uint32 = 0x00000002 // CredSSP
	ProtocolHybridEx uint32 = 0x00000008 // CredSSP + Early User Authorization Result
)

// Коды отказа в согласовании (MS-RDPBCGR 2.2.1.2.2).
const (
	FailureSSLRequiredByServer     uint32 = 0x00000001
	FailureSSLNotAllowedByServer   uint32 = 0x00000002
	FailureSSLCertNotOnServer      uint32 = 0x00000003
	FailureInconsistentFlags       uint32 = 0x00000004
	FailureHybridRequiredByServer  uint32 = 0x00000005
	FailureSSLWithUserAuthRequired uint32 = 0x00000006
)

const (
	tpktHeaderLen = 4
	tpktVersion   = 0x03
	tpduCR        = 0xE0
	tpduCC        = 0xD0
	negTypeReq    = 0x01
	negTypeRsp    = 0x02
	negTypeFail   = 0x03
	negLen        = 8 // NEG_REQ, NEG_RSP и NEG_FAILURE — все ровно 8 байт
	fixedTPDULen  = 6 // код + DST-REF + SRC-REF + class
)

// ConnectRequest — разобранный X.224 Connection Request.
type ConnectRequest struct {
	// Requested — запрошенные протоколы. Ноль означает standard RDP security
	// (структуры согласования в запросе не было).
	Requested uint32
	// HasNegReq различает «запрошен RDP(0)» и «структуры согласования не было».
	HasNegReq bool
	// Flags из RDP_NEG_REQ.
	Flags byte
	// Variable — всё, что стоит до NEG_REQ: routingToken или Cookie. Сервер по
	// нему маршрутизирует, поэтому оно обязано доехать без изменений.
	Variable []byte
}

// ConnectConfirm — разобранный X.224 Connection Confirm.
type ConnectConfirm struct {
	// Selected — выбранный сервером протокол. ProtocolRDP означает, что
	// структуры согласования в ответе не было, то есть standard RDP security.
	Selected uint32
	Flags    byte
}

// NegotiationFailure — сервер отказал в согласовании и назвал причину.
//
// Это отдельный тип, а не строка: код нужен вызывающему, чтобы отличить
// «сервер требует NLA» от «у сервера нет сертификата», а пользователю нужен
// внятный текст вместо «connection failed».
type NegotiationFailure struct {
	Code uint32
}

func (e *NegotiationFailure) Error() string {
	return fmt.Sprintf("x224: сервер отклонил согласование (код %#x): %s", e.Code, failureText(e.Code))
}

func failureText(code uint32) string {
	switch code {
	case FailureSSLRequiredByServer:
		return "сервер требует TLS"
	case FailureSSLNotAllowedByServer:
		return "сервер не разрешает TLS"
	case FailureSSLCertNotOnServer:
		return "на сервере нет сертификата для TLS"
	case FailureInconsistentFlags:
		return "несовместимые флаги в запросе"
	case FailureHybridRequiredByServer:
		return "сервер требует NLA (CredSSP)"
	case FailureSSLWithUserAuthRequired:
		return "сервер требует TLS с авторизацией пользователя"
	default:
		return "неизвестная причина"
	}
}

// tpdu вычленяет TPDU из TPKT, проверяя обе длины. Возвращает также полную
// длину TPKT, чтобы вызывающий мог убедиться, что за пакетом ничего не висит.
func tpdu(b []byte) ([]byte, error) {
	if len(b) < tpktHeaderLen+1 {
		return nil, fmt.Errorf("x224: усечённый TPKT (%d байт)", len(b))
	}
	if b[0] != tpktVersion {
		return nil, fmt.Errorf("x224: TPKT версии %d, ожидалась 3", b[0])
	}
	total := int(binary.BigEndian.Uint16(b[2:4]))
	if total < tpktHeaderLen+1 || total > len(b) {
		return nil, fmt.Errorf("x224: TPKT length %d при буфере %d", total, len(b))
	}
	li := int(b[tpktHeaderLen])
	end := tpktHeaderLen + 1 + li
	if end > total {
		return nil, fmt.Errorf("x224: LI %d выходит за TPKT length %d", li, total)
	}
	t := b[tpktHeaderLen+1 : end]
	if len(t) < fixedTPDULen {
		return nil, fmt.Errorf("x224: TPDU короче фиксированной части (%d байт)", len(t))
	}
	return t, nil
}

// splitNegotiation отделяет переменную часть (cookie) от структуры согласования.
// Структура, если она есть, всегда последние 8 байт TPDU.
func splitNegotiation(t []byte) (variable, neg []byte) {
	rest := t[fixedTPDULen:]
	if len(rest) < negLen {
		return rest, nil
	}
	candidate := rest[len(rest)-negLen:]
	switch candidate[0] {
	case negTypeReq, negTypeRsp, negTypeFail:
		return rest[:len(rest)-negLen], candidate
	default:
		return rest, nil
	}
}

// ParseConnectRequest разбирает X.224 Connection Request.
func ParseConnectRequest(b []byte) (ConnectRequest, error) {
	t, err := tpdu(b)
	if err != nil {
		return ConnectRequest{}, err
	}
	if t[0] != tpduCR {
		return ConnectRequest{}, fmt.Errorf("x224: TPDU-код %#x, ожидался Connection Request", t[0])
	}
	variable, neg := splitNegotiation(t)
	if neg == nil {
		return ConnectRequest{Variable: variable}, nil
	}
	if neg[0] != negTypeReq {
		return ConnectRequest{}, fmt.Errorf("x224: в Connection Request структура типа %#x", neg[0])
	}
	if got := binary.LittleEndian.Uint16(neg[2:4]); got != negLen {
		return ConnectRequest{}, fmt.Errorf("x224: NEG_REQ length %d, ожидалось %d", got, negLen)
	}
	return ConnectRequest{
		Requested: binary.LittleEndian.Uint32(neg[4:8]),
		HasNegReq: true,
		Flags:     neg[1],
		Variable:  variable,
	}, nil
}

// ParseConnectConfirm разбирает X.224 Connection Confirm.
//
// NEG_FAILURE возвращается как *NegotiationFailure: это разобранный отказ с
// кодом, а не сбой разбора.
func ParseConnectConfirm(b []byte) (ConnectConfirm, error) {
	t, err := tpdu(b)
	if err != nil {
		return ConnectConfirm{}, err
	}
	if t[0] != tpduCC {
		return ConnectConfirm{}, fmt.Errorf("x224: TPDU-код %#x, ожидался Connection Confirm", t[0])
	}
	variable, neg := splitNegotiation(t)
	if neg == nil {
		// Пустой хвост — законный ответ: сервер выбрал standard RDP security.
		//
		// Непустой хвост, из которого не получилась структура согласования, —
		// это мусор, и назвать его «standard RDP security» нельзя. Отказ
		// случится в обоих случаях, но пользователю нужен верный диагноз:
		// «сервер выбрал не NLA» и «сервер прислал испорченный ответ» — разные
		// поводы, и лечатся они по-разному.
		if len(variable) != 0 {
			return ConnectConfirm{}, fmt.Errorf("x224: в Connection Confirm %d байт, не образующих структуру согласования", len(variable))
		}
		return ConnectConfirm{Selected: ProtocolRDP}, nil
	}
	if got := binary.LittleEndian.Uint16(neg[2:4]); got != negLen {
		return ConnectConfirm{}, fmt.Errorf("x224: структура согласования length %d, ожидалось %d", got, negLen)
	}
	switch neg[0] {
	case negTypeRsp:
		return ConnectConfirm{Selected: binary.LittleEndian.Uint32(neg[4:8]), Flags: neg[1]}, nil
	case negTypeFail:
		return ConnectConfirm{}, &NegotiationFailure{Code: binary.LittleEndian.Uint32(neg[4:8])}
	default:
		return ConnectConfirm{}, fmt.Errorf("x224: в Connection Confirm структура типа %#x", neg[0])
	}
}
