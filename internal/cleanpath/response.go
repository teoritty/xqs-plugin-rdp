package cleanpath

import (
	"encoding/asn1"
	"fmt"
)

// BuildResponse собирает ответ прокси: Connection Confirm, цепочка
// сертификатов сервера и его разрешённый адрес.
//
// Цепочка отдаётся для диагностики, а не для защиты. При selected = SSL клиент
// её не использует: привязка к открытому ключу нужна только для CredSSP,
// которого на плече «браузер ↔ плагин» нет (проверено в фазе 0a — стаб отдавал
// сертификат-пустышку, и клиент не возражал). Проверка подлинности сервера
// целиком лежит на плагине, см. internal/trust.
func BuildResponse(x224 []byte, certChain [][]byte, serverAddr string) ([]byte, error) {
	out, err := asn1.Marshal(responsePDU{
		Version:    Version,
		X224:       x224,
		CertChain:  certChain,
		ServerAddr: serverAddr,
	})
	if err != nil {
		return nil, fmt.Errorf("cleanpath: сборка ответа: %w", err)
	}
	return out, nil
}

// BuildError собирает ответ с ошибкой — единственный способ объяснить браузеру,
// почему подключение не состоялось. Без него он покажет таймаут.
func BuildError(code int) ([]byte, error) {
	out, err := asn1.Marshal(errPDU{
		Version: Version,
		Err:     errorSeq{ErrorCode: code},
	})
	if err != nil {
		return nil, fmt.Errorf("cleanpath: сборка ошибки: %w", err)
	}
	return out, nil
}

// Response — разобранный ответ. Плагин ответы не принимает (он их только
// отправляет), поэтому разбор существует ради тестов: без него нечем проверить,
// что собранное нами вообще читается.
type Response struct {
	Version    int
	X224       []byte
	CertChain  [][]byte
	ServerAddr string
}

// ParseResponseForTest разбирает собранный нами ответ.
func ParseResponseForTest(b []byte) (Response, error) {
	var p responsePDU
	rest, err := asn1.Unmarshal(b, &p)
	if err != nil {
		return Response{}, fmt.Errorf("cleanpath: разбор ответа: %w", err)
	}
	if len(rest) != 0 {
		return Response{}, ErrTrailing
	}
	return Response{
		Version:    p.Version,
		X224:       p.X224,
		CertChain:  p.CertChain,
		ServerAddr: p.ServerAddr,
	}, nil
}
