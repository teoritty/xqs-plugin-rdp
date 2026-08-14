package cleanpath

import (
	"encoding/asn1"
	"errors"
	"fmt"
)

var (
	// ErrTrailing — после SEQUENCE остались байты.
	ErrTrailing = errors.New("cleanpath: мусор после SEQUENCE")
	// ErrNoX224 — в запросе нет x224_connection_pdu. Так выглядит режим
	// VMConnect (Hyper-V), которого мы не поддерживаем.
	ErrNoX224 = errors.New("cleanpath: в запросе нет x224_connection_pdu (режим VMConnect не поддерживается)")
	// ErrVersion — версия PDU не наша.
	ErrVersion = errors.New("cleanpath: неподдерживаемая версия PDU")
)

// Request — разобранный запрос браузера.
type Request struct {
	// X224 — Connection Request, который браузер просит передать серверу.
	X224 []byte

	// Destination и ProxyAuth разбираются, но им НЕЛЬЗЯ ДОВЕРЯТЬ.
	//
	// Их пишет страница в iframe — недоверенная граница. Адрес подключения
	// берётся из session.connect.fields, где его задал пользователь через
	// хост; поле отсюда годится только на то, чтобы сверить с ним и разорвать
	// сессию при расхождении. Авторизацию уже сделал токен ядра, поэтому
	// ProxyAuth не используется вовсе.
	//
	// В эталонной реализации Devolutions адрес приходит именно отсюда — им это
	// подходит, потому что их прокси обслуживает произвольных клиентов. Нам
	// нет: скомпрометированный iframe заставил бы плагин дозвониться куда
	// угодно в локальной сети под нашим allowArbitraryOutbound.
	Destination string
	ProxyAuth   string
}

// ParseRequest разбирает запрос браузера.
//
// Хвост после SEQUENCE — ошибка: дальше по этому же потоку пойдут байты RDP, и
// снисходительность здесь означала бы рассинхрон, который проявится далеко от
// места, где возник.
func ParseRequest(b []byte) (Request, error) {
	var p requestPDU
	rest, err := asn1.Unmarshal(b, &p)
	if err != nil {
		return Request{}, fmt.Errorf("cleanpath: разбор запроса: %w", err)
	}
	if len(rest) != 0 {
		return Request{}, ErrTrailing
	}
	if p.Version != Version {
		return Request{}, fmt.Errorf("%w: %d, ожидалась %d", ErrVersion, p.Version, Version)
	}
	if len(p.X224) == 0 {
		return Request{}, ErrNoX224
	}
	return Request{
		X224:        p.X224,
		Destination: p.Destination,
		ProxyAuth:   p.ProxyAuth,
	}, nil
}

// BuildRequestForTest собирает запрос той же формы, что шлёт браузер.
// Существует только ради тестов: без него round-trip проверить нечем.
func BuildRequestForTest(x224 []byte, destination, proxyAuth string) ([]byte, error) {
	return asn1.Marshal(requestPDU{
		Version:     Version,
		Destination: destination,
		ProxyAuth:   proxyAuth,
		X224:        x224,
	})
}
