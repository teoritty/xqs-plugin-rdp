package credssp

import (
	"encoding/asn1"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

// credTypePassword — TSPasswordCreds (MS-CSSP 2.2.1.2.1).
const credTypePassword = 1

// TSCredentials (MS-CSSP 2.2.1.2):
//
//	TSCredentials ::= SEQUENCE {
//	    credType    [0] INTEGER,
//	    credentials [1] OCTET STRING   -- DER TSPasswordCreds
//	}
type tsCredentialsDER struct {
	CredType    int    `asn1:"explicit,tag:0"`
	Credentials []byte `asn1:"explicit,tag:1"`
}

// TSPasswordCreds (MS-CSSP 2.2.1.2.1). Строки внутри — UTF-16LE.
type tsPasswordCredsDER struct {
	DomainName []byte `asn1:"explicit,tag:0"`
	UserName   []byte `asn1:"explicit,tag:1"`
	Password   []byte `asn1:"explicit,tag:2"`
}

// EncodeCredentials упаковывает учётные данные в TSCredentials.
//
// Пароль принимается как []byte и НЕ копируется в string: строку в Go нельзя
// занулить, и любая копия пароля в string пережила бы нас в куче до сборки
// мусора. Промежуточное UTF-16-представление зануляется здесь же.
//
// Вызывающий остаётся владельцем password и обязан занулить его сам: этот
// пакет не знает, нужен ли пароль дальше.
func EncodeCredentials(domain, username string, password []byte) ([]byte, error) {
	pwUTF16 := utf16LEFromBytes(password)
	// Промежуточная копия пароля живёт ровно до конца функции.
	defer clearBytes(pwUTF16)

	inner, err := asn1.Marshal(tsPasswordCredsDER{
		DomainName: utf16LEFromString(domain),
		UserName:   utf16LEFromString(username),
		Password:   pwUTF16,
	})
	if err != nil {
		return nil, fmt.Errorf("credssp: сборка TSPasswordCreds: %w", err)
	}
	// inner несёт пароль в открытом виде; он будет зашифрован вызывающим и
	// должен быть занулён им же после этого. См. handshake.go.
	out, err := asn1.Marshal(tsCredentialsDER{
		CredType:    credTypePassword,
		Credentials: inner,
	})
	if err != nil {
		clearBytes(inner)
		return nil, fmt.Errorf("credssp: сборка TSCredentials: %w", err)
	}
	clearBytes(inner)
	return out, nil
}

// utf16LEFromString кодирует строку в UTF-16LE без BOM и без завершающего нуля —
// именно так, как этого ждёт MS-CSSP.
func utf16LEFromString(s string) []byte {
	return encodeUTF16LE(utf16.Encode([]rune(s)))
}

// utf16LEFromBytes делает то же для пароля, не создавая его копии в string.
//
// Декодирование идёт прямо по байтам через utf8.DecodeRune. Соблазнительное
// `[]rune(string(b))` короче, но материализует пароль в неизменяемой строке,
// которую невозможно занулить: она пролежит в куче до сборки мусора, то есть
// ровно столько, сколько мы не контролируем.
func utf16LEFromBytes(b []byte) []byte {
	runes := make([]rune, 0, len(b))
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		runes = append(runes, r)
		i += size
	}
	u := utf16.Encode(runes)
	out := encodeUTF16LE(u)
	// Промежуточные представления пароля живут не дольше вызова.
	for i := range runes {
		runes[i] = 0
	}
	for i := range u {
		u[i] = 0
	}
	return out
}

func encodeUTF16LE(u []uint16) []byte {
	out := make([]byte, 0, len(u)*2)
	for _, c := range u {
		out = append(out, byte(c), byte(c>>8))
	}
	return out
}

// clearBytes зануляет срез. Отдельная функция, чтобы намерение читалось в
// вызове и чтобы её нельзя было «оптимизировать» удалением при чтении кода.
func clearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
