package trust

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
)

// UnknownServerError — пина нет, сервер видим впервые.
//
// Это НЕ то же самое, что несовпадение, и разделение здесь принципиально.
// Первое подключение к новому серверу — обычное дело; сменившийся ключ —
// повод остановиться. Один общий текст на оба случая приучил бы пользователя
// проходить мимо второго так же, как он проходит мимо первого.
type UnknownServerError struct {
	// Fingerprint — то, что пользователь должен сверить и вписать в поле
	// подключения. Без него ошибка бесполезна: вписывать нечего.
	Fingerprint string
}

func (e *UnknownServerError) Error() string {
	return fmt.Sprintf(
		"trust: отпечаток сервера не задан. Сверьте отпечаток и, если он верен, впишите его в поле «Server key fingerprint»: %s",
		e.Fingerprint,
	)
}

// MismatchError — ключ сервера не тот, что был подтверждён.
type MismatchError struct {
	Expected string
	Got      string
}

func (e *MismatchError) Error() string {
	return fmt.Sprintf(
		"trust: ключ сервера изменился. Ожидался %s, получен %s. Так выглядит и обычная переустановка сервера, и перехват соединения — пока причина не выяснена, подключение не выполняется",
		e.Expected, e.Got,
	)
}

// ErrEmptyChain — сервер не прислал ни одного сертификата.
var ErrEmptyChain = errors.New("trust: сервер не прислал сертификат")

// Verify сверяет сертификат сервера с подтверждённым отпечатком.
//
// Пустой пин — отказ. Это поведение StrictHostKeyChecking=yes, а не
// trust-on-first-use: плагину негде запомнить увиденное (при isolation
// per-session каталог данных живёт ровно одну сессию), поэтому «запомнить и
// больше не спрашивать» было бы обещанием, которого мы не можем сдержать.
func Verify(certDER []byte, pin string) error {
	got, err := Fingerprint(certDER)
	if err != nil {
		return err
	}
	// Пробелы вокруг вставленного значения — норма при копировании из текста
	// ошибки. А регистр не трогаем: base64 регистрозависим, и «исправить»
	// регистр значило бы принять другой отпечаток за этот.
	pin = strings.TrimSpace(pin)
	if pin == "" {
		return &UnknownServerError{Fingerprint: got}
	}
	// Сравнение постоянного времени: отпечаток не секрет, но сравнение
	// секретов и не-секретов в одном коде лучше держать одинаковым, чтобы
	// «быстрое» сравнение не переехало однажды туда, где оно опасно.
	if subtle.ConstantTimeCompare([]byte(pin), []byte(got)) != 1 {
		return &MismatchError{Expected: pin, Got: got}
	}
	return nil
}

// VerifyChain сверяет лист цепочки — сертификат самого сервера.
//
// Именно лист, а не «что-нибудь из цепочки»: доверие к промежуточному
// сертификату означало бы доверие ко всему, что им подписано.
func VerifyChain(chain [][]byte, pin string) error {
	if len(chain) == 0 {
		return ErrEmptyChain
	}
	return Verify(chain[0], pin)
}
