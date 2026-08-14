package ipc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ErrProtocol — нарушение, после которого продолжать нельзя.
//
// Ядро на протокольное нарушение не отвечает ошибкой: оно закрывает stdio и
// убивает процесс, а затем перезапускает нас (до трёх раз). Поэтому у нас нет
// права на снисходительный парсер — любая неоднозначность на нашей стороне
// превращается в цикл перезапусков, который пользователь видит как мигающий
// плагин.
//
// Обрыв потока на границе фрейма к этому классу НЕ относится: это обычный
// конец жизни процесса, и путать их нельзя.
var ErrProtocol = errors.New("ipc: protocol violation")

const headerLen = 9

// Frame — одно сообщение на шине. ChannelID == 0 означает управляющую
// плоскость JSON-RPC.
type Frame struct {
	Kind      byte
	ChannelID uint32
	Payload   []byte
}

// ReadFrame читает ровно один фрейм.
//
// Возвращает io.EOF на чистой границе, io.ErrUnexpectedEOF при обрыве внутри
// фрейма и ErrProtocol на нарушении.
func ReadFrame(r io.Reader) (Frame, error) {
	var h [headerLen]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		// ReadFull отдаёт ErrUnexpectedEOF при частичном заголовке и EOF при
		// пустом потоке. Оба — конец потока, не нарушение.
		return Frame{}, err
	}
	n := binary.BigEndian.Uint32(h[0:4])
	kind := h[4]
	channelID := binary.BigEndian.Uint32(h[5:9])

	limit, ok := maxPayload(kind)
	if !ok {
		return Frame{}, fmt.Errorf("%w: неизвестный kind %#x", ErrProtocol, kind)
	}
	// Проверка ДО аллокации: длина пришла от недоверенной стороны, и доверчивый
	// make() здесь — это OOM по чужой команде.
	if n > uint32(limit) {
		return Frame{}, fmt.Errorf("%w: длина %d сверх потолка %d для kind %#x", ErrProtocol, n, limit, kind)
	}
	if exactPayload(kind) && int(n) != limit {
		return Frame{}, fmt.Errorf("%w: kind %#x требует ровно %d байт, получено %d", ErrProtocol, kind, limit, n)
	}

	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		if errors.Is(err, io.EOF) {
			// Заголовок обещал n байт, поток кончился раньше.
			return Frame{}, io.ErrUnexpectedEOF
		}
		return Frame{}, err
	}
	return Frame{Kind: kind, ChannelID: channelID, Payload: payload}, nil
}

// WriteFrame пишет ровно один фрейм, отказываясь от того, что ядро сочло бы
// нарушением. Отказ здесь дешевле, чем убитый процесс там.
func WriteFrame(w io.Writer, f Frame) error {
	limit, ok := maxPayload(f.Kind)
	if !ok {
		return fmt.Errorf("%w: неизвестный kind %#x", ErrProtocol, f.Kind)
	}
	if len(f.Payload) > limit {
		return fmt.Errorf("%w: payload %d сверх потолка %d для kind %#x", ErrProtocol, len(f.Payload), limit, f.Kind)
	}
	if exactPayload(f.Kind) && len(f.Payload) != limit {
		return fmt.Errorf("%w: kind %#x требует ровно %d байт, получено %d", ErrProtocol, f.Kind, limit, len(f.Payload))
	}

	var h [headerLen]byte
	binary.BigEndian.PutUint32(h[0:4], uint32(len(f.Payload)))
	h[4] = f.Kind
	binary.BigEndian.PutUint32(h[5:9], f.ChannelID)
	if _, err := w.Write(h[:]); err != nil {
		return err
	}
	_, err := w.Write(f.Payload)
	return err
}
