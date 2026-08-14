package proxy

import "encoding/binary"

// Правка одного поля в первом сообщении браузера после точки отсечения.
//
// Это единственное место, где плагин разбирает байты ПОСЛЕ рукопожатия, и
// исключение здесь вынужденное, а не удобное. Две стороны требуют
// несовместимого, и обе — обоснованно:
//
//   - Клиент (ironrdp-web с enableCredssp(false)) объявляет только SSL и
//     отвергает ответ, называющий протокол, которого он не просил:
//     «client advertised SSL, but server selected HYBRID» —
//     ironrdp-connector/src/lib.rs:409.
//   - Сервер по MS-RDPBCGR 3.3.5.3.1 обязан оборвать соединение, если
//     clientCoreData.serverSelectedProtocol не совпадает с тем, что он выбрал
//     в RDP Negotiation Response.
//
// Одним значением не угодить обоим. Поэтому браузеру говорится SSL, а здесь
// сказанное ему исправляется на правду по дороге к серверу. Оба поведения
// воспроизведены на стенде, а не выведены из документации.

const (
	// csCoreType — TS_UD_CS_CORE, тип блока пользовательских данных GCC.
	// В потоке лежит little-endian, то есть байтами 0x01 0xC0.
	csCoreTypeLo = 0x01
	csCoreTypeHi = 0xC0

	// csCoreSelectedOffset — смещение serverSelectedProtocol от начала блока
	// (MS-RDPBCGR 2.2.1.3.2: заголовок 4 байта, дальше фиксированные поля).
	csCoreSelectedOffset = 212

	// Границы длины блока. Нижняя — поле должно поместиться целиком; верхняя
	// отсекает случайное совпадение двух байт в чужих данных.
	csCoreMinLen = csCoreSelectedOffset + 4
	csCoreMaxLen = 512
)

// patchServerSelectedProtocol правит serverSelectedProtocol в CS_CORE.
//
// Возвращает, нашёлся ли блок. Ненайденный блок — не ошибка: поле
// необязательное, и старый клиент вправе его не прислать. Тогда и правит
// нечего, а сервер такому клиенту ничего не предъявит.
func patchServerSelectedProtocol(pdu []byte, selected uint32) bool {
	for i := 0; i+4 <= len(pdu); i++ {
		if pdu[i] != csCoreTypeLo || pdu[i+1] != csCoreTypeHi {
			continue
		}
		length := int(binary.LittleEndian.Uint16(pdu[i+2 : i+4]))
		if length < csCoreMinLen || length > csCoreMaxLen || i+length > len(pdu) {
			continue
		}
		binary.LittleEndian.PutUint32(pdu[i+csCoreSelectedOffset:i+csCoreSelectedOffset+4], selected)
		return true
	}
	return false
}
