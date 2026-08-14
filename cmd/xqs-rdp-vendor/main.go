// Command xqs-rdp-vendor раскладывает веб-клиент IronRDP в ui/vendor/ironrdp.
//
// Он существует из-за одной правки, которую приходится вносить в чужой код, и
// именно поэтому он инструмент, а не инструкция в README: правка должна
// воспроизводиться при каждом обновлении одинаково, а не по памяти.
//
// # Правка и её причина
//
// В npm-пакете модуль WebAssembly зашит в JS одной строкой
// `data:application/wasm;base64,…`, которую загрузчик тянет через fetch().
// CSP embed-роутов ядра — `connect-src 'self'`, и `data:` в неё не входит:
// запрос блокируется, init() падает с «Failed to fetch», страница не доходит
// даже до открытия туннеля. Проверено вживую.
//
// Поэтому base64 распаковывается в отдельный .wasm рядом с модулем, а строка
// заменяется на `new URL("./…wasm", import.meta.url)`. Запрос становится
// same-origin и проходит; загрузчик wasm-bindgen принимает URL там же, где
// принимал строку, так что поведение не меняется. Go отдаёт .wasm как
// application/wasm, поэтому работает быстрый путь instantiateStreaming.
//
// Побочно бандл худеет: base64 весит на треть больше двоичных данных.
package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	dataURIPrefix = "data:application/wasm;base64,"
	wasmFileName  = "iron-remote-desktop-rdp.wasm"
	replacement   = `new URL("./` + wasmFileName + `", import.meta.url)`
)

type source struct {
	pkg  string // каталог пакета в node_modules
	js   string // имя JS-файла
	dts  string // как назвать .d.ts на выходе
	wasm bool   // распаковывать ли встроенный WebAssembly
}

var sources = []source{
	{pkg: "iron-remote-desktop", js: "iron-remote-desktop.js", dts: "iron-remote-desktop.d.ts"},
	{pkg: "iron-remote-desktop-rdp", js: "iron-remote-desktop-rdp.js", dts: "iron-remote-desktop-rdp.d.ts", wasm: true},
}

func main() {
	modules := flag.String("modules", "spike/web/node_modules/@devolutions", "каталог с распакованными npm-пакетами")
	out := flag.String("out", "ui/vendor/ironrdp", "куда положить бандл")
	flag.Parse()

	if err := run(*modules, *out); err != nil {
		log.Fatal(err)
	}
}

func run(modules, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, s := range sources {
		if err := vendorOne(modules, out, s); err != nil {
			return fmt.Errorf("%s: %w", s.pkg, err)
		}
	}
	return writeChecksums(out)
}

func vendorOne(modules, out string, s source) error {
	src := filepath.Join(modules, s.pkg)

	if err := copyFile(filepath.Join(src, "index.d.ts"), filepath.Join(out, s.dts)); err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(src, s.js))
	if err != nil {
		return err
	}
	if !s.wasm {
		return os.WriteFile(filepath.Join(out, s.js), raw, 0o644)
	}

	patched, wasm, err := extractWasm(string(raw))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, wasmFileName), wasm, 0o644); err != nil {
		return err
	}
	fmt.Printf("%-36s %6.1f МБ WebAssembly вынесено отдельно\n", wasmFileName, float64(len(wasm))/(1<<20))
	return os.WriteFile(filepath.Join(out, s.js), []byte(patched), 0o644)
}

// extractWasm находит строковый литерал с data-URI, декодирует его и заменяет
// сам литерал ссылкой на файл.
//
// Ищется именно литерал, а не «const kA = …»: имя переменной минифицировано и
// меняется от сборки к сборке, а префикс data-URI — нет.
func extractWasm(js string) (string, []byte, error) {
	start := strings.Index(js, dataURIPrefix)
	if start < 0 {
		return "", nil, fmt.Errorf("data-URI с WebAssembly не найден — возможно, апстрим перестал его встраивать; тогда эта правка больше не нужна")
	}
	// Литерал начинается кавычкой непосредственно перед префиксом.
	quote := start - 1
	if quote < 0 || (js[quote] != '"' && js[quote] != '\'') {
		return "", nil, fmt.Errorf("перед data-URI нет открывающей кавычки — форма модуля изменилась, правку надо пересмотреть")
	}
	end := strings.IndexByte(js[start:], js[quote])
	if end < 0 {
		return "", nil, fmt.Errorf("литерал с data-URI не закрыт")
	}
	end += start

	b64 := js[start+len(dataURIPrefix) : end]
	wasm, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", nil, fmt.Errorf("декодирование base64: %w", err)
	}
	// Проверяем, что получили именно модуль WebAssembly, а не что-то похожее:
	// молча записать мусор в .wasm значит отложить ошибку до браузера.
	if len(wasm) < 4 || string(wasm[:4]) != "\x00asm" {
		return "", nil, fmt.Errorf("декодированные данные не начинаются с сигнатуры WebAssembly")
	}

	patched := js[:quote] + replacement + js[end+1:]
	if strings.Contains(patched, dataURIPrefix) {
		return "", nil, fmt.Errorf("в модуле остался ещё один data-URI — заменена не та строка")
	}
	return patched, wasm, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	outF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer outF.Close()
	if _, err := io.Copy(outF, in); err != nil {
		return err
	}
	return outF.Close()
}

// writeChecksums пишет SHA256SUMS в том же формате, что и упаковщик бандла.
func writeChecksums(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var lines []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "SHA256SUMS" || e.Name() == "VERSION" || e.Name() == "LICENSE-MIT" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		lines = append(lines, fmt.Sprintf("%s *%s", hex.EncodeToString(sum[:]), e.Name()))
	}
	sort.Strings(lines)
	return os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
