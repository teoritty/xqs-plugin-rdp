package ipc

// Потолки и виды фреймов канальной шины (docs/plugin-api.md, «Frame layer»).
//
// Превышение потолка — не backpressure и не повод для повтора: ядро закрывает
// stdio и убивает процесс, а затем запускает crash-recovery. Поэтому все
// проверки здесь односторонние и жёсткие.
const (
	// MaxJSONRPC — потолок управляющей плоскости, kind 0x01.
	MaxJSONRPC = 256 << 10
	// MaxChannelData — общий потолок бинарного канала, kind 0x02.
	MaxChannelData = 1 << 20
	// MaxEmbedStream — отдельный, более низкий потолок для purpose
	// embed-stream. На уровне фрейма он не виден: kind один и тот же, а purpose
	// знает только тот, кто открывал канал. Резать обязан он же.
	MaxEmbedStream = 64 << 10
	// creditPayloadLen — кредитный фрейм фиксирован: [4B channelId][4B credit].
	creditPayloadLen = 8
)

const (
	KindJSONRPC     byte = 0x01
	KindChannelData byte = 0x02
	KindCredit      byte = 0x03
)

// maxPayload возвращает потолок для kind. Второе значение — false для всего
// незнакомого: 0x04–0x0F зарезервированы и запрещены в v1, а 0x00 и всё
// остальное не определены вовсе.
func maxPayload(kind byte) (int, bool) {
	switch kind {
	case KindJSONRPC:
		return MaxJSONRPC, true
	case KindChannelData:
		return MaxChannelData, true
	case KindCredit:
		return creditPayloadLen, true
	default:
		return 0, false
	}
}

// exactPayload сообщает, обязан ли kind иметь ровно maxPayload байт. Для
// кредита длина фиксирована, и «меньше» так же неверно, как «больше».
func exactPayload(kind byte) bool { return kind == KindCredit }
