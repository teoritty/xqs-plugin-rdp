package x224

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MaxPDU — потолок одного X.224 PDU.
//
// Длина приходит от стороны, с которой мы ещё не аутентифицированы. TPKT
// позволяет до 65535 байт, но рукопожатие RDP укладывается в единицы сотен, и
// незачем разрешать больше того, что бывает.
const MaxPDU = 4096

// ReadPDU читает ровно один TPKT-пакет.
//
// Читать «сколько дадут» нельзя: за Connection Confirm в том же потоке сразу
// идёт TLS ClientHello, и лишний прочитанный байт увёл бы его из рук
// crypto/tls — а проявилось бы это как необъяснимый сбой рукопожатия.
func ReadPDU(r io.Reader) ([]byte, error) {
	var header [tpktHeaderLen]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, fmt.Errorf("x224: чтение заголовка TPKT: %w", err)
	}
	if header[0] != tpktVersion {
		return nil, fmt.Errorf("x224: TPKT версии %d, ожидалась 3", header[0])
	}
	total := int(binary.BigEndian.Uint16(header[2:4]))
	if total < tpktHeaderLen {
		return nil, fmt.Errorf("x224: TPKT length %d меньше заголовка", total)
	}
	if total > MaxPDU {
		return nil, fmt.Errorf("x224: TPKT length %d сверх потолка %d", total, MaxPDU)
	}
	out := make([]byte, total)
	copy(out, header[:])
	if _, err := io.ReadFull(r, out[tpktHeaderLen:]); err != nil {
		return nil, fmt.Errorf("x224: чтение тела TPKT: %w", err)
	}
	return out, nil
}
