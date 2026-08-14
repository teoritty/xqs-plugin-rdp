package ipc

import "strings"

// redacted — маска постоянной длины. Именно постоянной: длина пароля сама по
// себе сужает перебор, поэтому «***(7)» было бы хуже, чем ничего.
const redacted = "***"

// secretKeys — ключи, значения которых не попадают в логи ни при каких
// обстоятельствах.
//
// Ядро режет чувствительные ключи в log.write самостоятельно, но это последний
// рубеж, а не первый: полагаться на чужую санитизацию значит зависеть от того,
// совпадает ли её список с нашим.
var secretKeys = map[string]bool{
	"password":    true,
	"secret":      true,
	"token":       true,
	"authtoken":   true,
	"credentials": true,
	"pubkeyauth":  true,
}

// Redact возвращает копию карты с замаскированными секретами, не трогая вход.
//
// Обход рекурсивный, потому что интересные секреты лежат не на верхнем уровне:
// в session.connect пароль приходит внутри fields, и плоский обход прошёл бы
// ровно мимо него.
func Redact(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}
	out := make(map[string]any, len(params))
	for k, v := range params {
		if secretKeys[strings.ToLower(k)] {
			out[k] = redacted
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			out[k] = Redact(nested)
			continue
		}
		out[k] = v
	}
	return out
}
