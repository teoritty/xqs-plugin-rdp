package ipc_test

import (
	"strings"
	"testing"

	"xqs-plugin-rdp/internal/ipc"
)

func TestRedactHidesSecretFields(t *testing.T) {
	in := map[string]any{
		"username":          "admin",
		"password":          "hunter2",
		"domain":            "CORP",
		"serverFingerprint": "SHA256:abc",
	}
	out := ipc.Redact(in)
	if out["password"] == "hunter2" {
		t.Fatal("пароль попал в лог")
	}
	if out["username"] != "admin" {
		t.Fatal("username не секрет, его резать не надо")
	}
	// Отпечаток обязан оставаться видимым: он и существует, чтобы человек его
	// прочитал и сверил.
	if out["serverFingerprint"] != "SHA256:abc" {
		t.Fatalf("serverFingerprint = %v, он не секрет", out["serverFingerprint"])
	}
}

// Длина пароля тоже не логируется: она сужает перебор.
func TestRedactDoesNotLeakSecretLength(t *testing.T) {
	short := ipc.Redact(map[string]any{"password": "a"})
	long := ipc.Redact(map[string]any{"password": strings.Repeat("a", 64)})
	if short["password"] != long["password"] {
		t.Fatalf("маска зависит от длины: %v vs %v", short["password"], long["password"])
	}
}

func TestRedactIsCaseInsensitive(t *testing.T) {
	out := ipc.Redact(map[string]any{"Password": "hunter2", "PASSWORD": "hunter2", "pAsSwOrD": "hunter2"})
	for k, v := range out {
		if v == "hunter2" {
			t.Fatalf("ключ %q не срезан", k)
		}
	}
}

func TestRedactDoesNotMutateInput(t *testing.T) {
	in := map[string]any{"password": "hunter2"}
	_ = ipc.Redact(in)
	if in["password"] != "hunter2" {
		t.Fatal("Redact изменил исходную карту — вызывающий код мог рассчитывать на неё")
	}
}

// Секрет, спрятанный в вложенной карте (например, в fields из session.connect),
// обязан срезаться так же: плоский обход поймал бы только верхний уровень, а
// именно там пароля как раз и не будет.
func TestRedactDescendsIntoNestedMaps(t *testing.T) {
	in := map[string]any{
		"sessionId": "s-1",
		"fields": map[string]any{
			"username": "admin",
			"password": "hunter2",
		},
	}
	out := ipc.Redact(in)
	nested, ok := out["fields"].(map[string]any)
	if !ok {
		t.Fatalf("fields имеет тип %T, ожидалась карта", out["fields"])
	}
	if nested["password"] == "hunter2" {
		t.Fatal("пароль во вложенной карте не срезан")
	}
	if nested["username"] != "admin" {
		t.Fatal("username во вложенной карте срезан напрасно")
	}
	// И вход по-прежнему цел.
	if in["fields"].(map[string]any)["password"] != "hunter2" {
		t.Fatal("Redact изменил вложенную карту входа")
	}
}

func TestRedactHandlesNil(t *testing.T) {
	if got := ipc.Redact(nil); len(got) != 0 {
		t.Fatalf("Redact(nil) = %v", got)
	}
}
