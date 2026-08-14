package x224

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrNoNegotiation — в Connection Request не было структуры согласования.
var ErrNoNegotiation = errors.New("x224: в Connection Request нет RDP_NEG_REQ")

// RewriteRequestForServer переписывает запрошенные протоколы браузера на
// HYBRID и только на него.
//
// Это половина точки отсечения: браузер просит SSL и NLA не делает, а мы просим
// у сервера NLA и делаем его сами. Вторая половина — ConnectConfirmForBrowser.
//
// HYBRID_EX сознательно не запрашивается, хотя сервер его поддерживает и
// выбрал бы (проверено на 192.168.31.38): HYBRID_EX добавляет после CredSSP
// Early User Authorization Result PDU, то есть ещё одно состояние в машине,
// которая и так терминирует безопасность. Мы просим ровно то, что умеем
// довести до конца.
//
// Переменная часть (routingToken, Cookie) переносится байт в байт: сервер по
// ней маршрутизирует, и её потеря выглядела бы как «иногда не подключается».
func RewriteRequestForServer(b []byte) ([]byte, error) {
	req, err := ParseConnectRequest(b)
	if err != nil {
		return nil, err
	}
	// Дописать NEG_REQ туда, где его не было, значило бы изменить длину и
	// смысл запроса, а не переписать поле. Такой запрос браузера означает
	// standard RDP security, и это отдельный разговор, а не наш случай.
	if !req.HasNegReq {
		return nil, ErrNoNegotiation
	}
	return buildConnectRequest(req.Variable, req.Flags, ProtocolHybrid)
}

func buildConnectRequest(variable []byte, flags byte, requested uint32) ([]byte, error) {
	bodyLen := fixedTPDULen + len(variable) + negLen
	total := tpktHeaderLen + 1 + bodyLen
	if total > 0xFFFF {
		return nil, fmt.Errorf("x224: Connection Request %d байт не помещается в TPKT", total)
	}

	out := make([]byte, 0, total)
	out = append(out, tpktVersion, 0x00)
	out = binary.BigEndian.AppendUint16(out, uint16(total))
	out = append(out, byte(bodyLen)) // LI
	out = append(out, tpduCR, 0x00, 0x00, 0x00, 0x00, 0x00)
	out = append(out, variable...)
	out = append(out, negTypeReq, flags)
	out = binary.LittleEndian.AppendUint16(out, negLen)
	out = binary.LittleEndian.AppendUint32(out, requested)
	return out, nil
}

// ConnectConfirmForBrowser собирает Connection Confirm с selected = SSL.
//
// Это вторая половина точки отсечения. Увидев SSL, коннектор ironrdp-web
// пропускает NLA («CredSSP is disabled, skipping NLA») и уходит прямо в MCS —
// проверено вживую в фазе 0a, в том числе когда браузер запрашивал HYBRID.
//
// Флаги нулевые намеренно: они относятся к возможностям, о которых с браузером
// договариваться не о чем, а транслировать флаги настоящего сервера значило бы
// обещать от его имени то, чего мы для этого плеча не поддерживаем.
func ConnectConfirmForBrowser() []byte {
	bodyLen := fixedTPDULen + negLen
	total := tpktHeaderLen + 1 + bodyLen

	out := make([]byte, 0, total)
	out = append(out, tpktVersion, 0x00)
	out = binary.BigEndian.AppendUint16(out, uint16(total))
	out = append(out, byte(bodyLen)) // LI
	out = append(out, tpduCC, 0x00, 0x00, 0x00, 0x00, 0x00)
	out = append(out, negTypeRsp, 0x00)
	out = binary.LittleEndian.AppendUint16(out, negLen)
	out = binary.LittleEndian.AppendUint32(out, ProtocolSSL)
	return out
}

// RequireHybrid проверяет, что сервер выбрал именно CredSSP.
//
// Функция отдельная и с говорящим именем нарочно: это защита от downgrade, и
// она не должна раствориться в цепочке if-ов, где её легко не заметить при
// правке. Всё, кроме HYBRID, — отказ: пользователь просил NLA, и продолжить
// без него значит выдать за защищённое соединение то, чем оно не является.
func RequireHybrid(selected uint32) error {
	if selected == ProtocolHybrid {
		return nil
	}
	return fmt.Errorf("x224: сервер выбрал %s вместо NLA (CredSSP) — соединение не будет установлено", protocolName(selected))
}

func protocolName(p uint32) string {
	switch p {
	case ProtocolRDP:
		return "standard RDP security"
	case ProtocolSSL:
		return "TLS без NLA"
	case ProtocolHybrid:
		return "NLA (CredSSP)"
	case ProtocolHybridEx:
		return "NLA-EX (CredSSP + Early User Authorization)"
	default:
		return fmt.Sprintf("неизвестный протокол %#x", p)
	}
}
