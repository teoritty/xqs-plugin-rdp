package cleanpath

import (
	"errors"
	"fmt"
	"io"
)

// MaxMessage — потолок одного RDCleanPath PDU.
//
// Его пишет страница в iframe — недоверенная сторона. Без потолка она заказала
// бы нам аллокацию любого размера одним полем длины.
const MaxMessage = 64 << 10

// ReadMessage читает ровно один DER SEQUENCE.
//
// Границу сообщения даёт сам DER, а не транспорт. Полагаться на то, что
// сообщение придёт одним фреймом, нельзя: WebSocket вправе его разрезать, а
// сразу за RDCleanPath-запросом по тому же потоку идут байты RDP — прочитать
// лишнее значит потерять их начало.
func ReadMessage(r io.Reader) ([]byte, error) {
	var head [2]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, fmt.Errorf("cleanpath: чтение заголовка: %w", err)
	}
	if head[0] != 0x30 {
		return nil, fmt.Errorf("cleanpath: ожидался DER SEQUENCE, получен тег %#x", head[0])
	}

	header := []byte{head[0], head[1]}
	bodyLen := int(head[1])
	if head[1] >= 0x80 {
		n := int(head[1] & 0x7F)
		if n == 0 || n > 4 {
			return nil, fmt.Errorf("cleanpath: неподдерживаемая форма длины DER (%d байт)", n)
		}
		lenBytes := make([]byte, n)
		if _, err := io.ReadFull(r, lenBytes); err != nil {
			return nil, fmt.Errorf("cleanpath: чтение длины: %w", err)
		}
		header = append(header, lenBytes...)
		bodyLen = 0
		for _, b := range lenBytes {
			bodyLen = bodyLen<<8 | int(b)
		}
	}
	if bodyLen < 0 {
		return nil, errors.New("cleanpath: отрицательная длина")
	}
	if bodyLen > MaxMessage {
		return nil, fmt.Errorf("cleanpath: сообщение %d байт сверх потолка %d", bodyLen, MaxMessage)
	}

	out := make([]byte, len(header)+bodyLen)
	copy(out, header)
	if _, err := io.ReadFull(r, out[len(header):]); err != nil {
		return nil, fmt.Errorf("cleanpath: чтение тела: %w", err)
	}
	return out, nil
}
