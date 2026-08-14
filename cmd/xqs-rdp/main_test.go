package main

import (
	"encoding/json"
	"os"
	"testing"
)

func loadManifest(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}

func TestManifestDeclaresOnlyWhatWeNeed(t *testing.T) {
	m := loadManifest(t, "../../plugin.json")

	engine := m["engine"].(map[string]any)
	if engine["type"] != "go-binary" {
		t.Fatalf("engine.type = %v; ядро отвергает всё кроме go-binary", engine["type"])
	}
	if m["isolation"] != "per-session" {
		t.Fatalf("isolation = %v; embed требует per-session", m["isolation"])
	}

	caps := m["capabilities"].(map[string]any)

	ch := caps["channel"].(map[string]any)
	// Не 8: виртуальные каналы RDP мультиплексируются внутри одного соединения,
	// а WASM-клиент открывает ровно одну WebSocket (находка R-2).
	if n := ch["maxConcurrent"].(float64); n != 2 {
		t.Fatalf("maxConcurrent = %v, ожидалось 2", n)
	}
	purposes := ch["purposes"].([]any)
	if len(purposes) != 2 {
		t.Fatalf("purposes = %v, ожидались ровно embed-stream и tcp-relay", purposes)
	}
	want := map[string]bool{"embed-stream": true, "tcp-relay": true}
	for _, p := range purposes {
		s := p.(string)
		if !want[s] {
			t.Fatalf("лишний purpose %q — каждый из них расширяет то, на что соглашается пользователь", s)
		}
	}

	// exec требует отдельного согласия при установке и нам не нужен;
	// filesystem не нужен, потому что хранить нечего.
	for _, unwanted := range []string{"fs", "filesystem", "exec", "events"} {
		if _, ok := caps[unwanted]; ok {
			t.Fatalf("манифест запрашивает %q, которого мы не используем", unwanted)
		}
	}

	sess := caps["session"].(map[string]any)
	if sess["embed"] != true || sess["terminal"] != false {
		t.Fatalf("session = %v; embed и terminal взаимоисключающи", sess)
	}
	protos := sess["connectProtocols"].([]any)
	if len(protos) != 1 || protos[0] != "rdp" {
		t.Fatalf("connectProtocols = %v", protos)
	}
}

func TestManifestFieldsCoverAuthAndTrust(t *testing.T) {
	m := loadManifest(t, "../../plugin.json")
	protos := m["contributions"].(map[string]any)["connectionProtocols"].([]any)
	proto := protos[0].(map[string]any)
	if proto["id"] != "rdp" {
		t.Fatalf("protocol id = %v", proto["id"])
	}
	if port := proto["defaultPort"].(float64); port != 3389 {
		t.Fatalf("defaultPort = %v", port)
	}
	if proto["embedEntry"] != "ui/rdp.html" {
		t.Fatalf("embedEntry = %v", proto["embedEntry"])
	}

	seen := map[string]map[string]any{}
	for _, g := range proto["fields"].([]any) {
		for _, f := range g.(map[string]any)["fields"].([]any) {
			fm := f.(map[string]any)
			seen[fm["id"].(string)] = fm
		}
	}
	for _, id := range []string{"username", "password", "domain"} {
		if _, ok := seen[id]; !ok {
			t.Fatalf("поле %q отсутствует", id)
		}
	}
	if seen["password"]["secret"] != true {
		t.Fatal("password обязан быть secret: тип password этого требует")
	}
	if seen["password"]["type"] != "password" {
		t.Fatalf("password type = %v", seen["password"]["type"])
	}
	// Доверие к серверу больше не поле формы: его помнит ядро, а спрашивает —
	// диалогом. Поле, оставшееся в манифесте, снова просило бы у пользователя
	// то, чего у него больше не спрашивают.
	if _, ok := seen["serverFingerprint"]; ok {
		t.Fatal("в манифесте осталось поле serverFingerprint — доверие переехало в ядро")
	}
}

// connectProtocols и contributions обязаны сходиться: ядро отвергает
// подключение по протоколу, не объявленному в capabilities.
func TestContributedProtocolIsDeclaredInCapabilities(t *testing.T) {
	m := loadManifest(t, "../../plugin.json")
	declared := map[string]bool{}
	for _, p := range m["capabilities"].(map[string]any)["session"].(map[string]any)["connectProtocols"].([]any) {
		declared[p.(string)] = true
	}
	for _, p := range m["contributions"].(map[string]any)["connectionProtocols"].([]any) {
		id := p.(map[string]any)["id"].(string)
		if !declared[id] {
			t.Fatalf("протокол %q объявлен в contributions, но не в capabilities.session.connectProtocols", id)
		}
	}
}

func TestActivationEventMatchesProtocol(t *testing.T) {
	m := loadManifest(t, "../../plugin.json")
	events := m["activationEvents"].([]any)
	if len(events) != 1 || events[0] != "onProtocol:rdp" {
		t.Fatalf("activationEvents = %v", events)
	}
}

// plugin.json и xqsp.json обязаны совпадать во всём, что описывает поведение:
// расхождение означает, что установленный плагин ведёт себя не так, как обещал
// каталог, из которого его поставили.
func TestManifestsAgree(t *testing.T) {
	p := loadManifest(t, "../../plugin.json")
	x := loadManifest(t, "../../xqsp.json")
	for _, key := range []string{"id", "version", "isolation", "engine", "capabilities", "contributions", "activationEvents", "requires"} {
		a, err := json.Marshal(p[key])
		if err != nil {
			t.Fatalf("marshal plugin.json[%s]: %v", key, err)
		}
		b, err := json.Marshal(x[key])
		if err != nil {
			t.Fatalf("marshal xqsp.json[%s]: %v", key, err)
		}
		if string(a) != string(b) {
			t.Fatalf("%s расходится:\nplugin.json: %s\nxqsp.json:   %s", key, a, b)
		}
	}
}

// Версия API в манифесте и в коде — одно и то же число в двух местах. Тест
// существует, чтобы они не разъехались молча.
func TestManifestAPIVersionMatchesCode(t *testing.T) {
	m := loadManifest(t, "../../plugin.json")
	got := m["requires"].(map[string]any)["pluginApi"]
	if got != declaredPluginAPI() {
		t.Fatalf("манифест требует %v, код сообщает %v", got, declaredPluginAPI())
	}
}
