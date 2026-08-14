// Command xqs-rdp-pack собирает .xqsp — установочный бандл плагина.
//
// Формат (ADR-016 ядра): ZIP из plugin.json, бинаря, дерева ui/ и SHA256SUMS,
// где суммы покрывают всё, кроме самого файла сумм.
//
// Инструмент отдельной командой, а не скриптом на shell/PowerShell, ровно по
// одной причине: он обязан работать одинаково на всех платформах, где мы
// собираемся, а два скрипта на двух языках разъезжаются молча.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// checksumsFile — имя, которое ищет ядро.
const checksumsFile = "SHA256SUMS"

type platform struct {
	os   string
	arch string
}

func (p platform) binaryName() string {
	if p.os == "windows" {
		return "xqs-rdp.exe"
	}
	return "xqs-rdp"
}

func (p platform) bundleName() string {
	return fmt.Sprintf("xqs-rdp-%s-%s.xqsp", p.os, p.arch)
}

var targets = []platform{
	{"windows", "amd64"},
	{"windows", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
}

func main() {
	outDir := flag.String("out", "dist", "куда положить бандлы")
	only := flag.String("only", "", "собрать только одну платформу, например windows/amd64")
	flag.Parse()

	list := targets
	if *only != "" {
		p, err := parsePlatform(*only)
		if err != nil {
			log.Fatal(err)
		}
		list = []platform{p}
	}

	if err := run(*outDir, list); err != nil {
		log.Fatal(err)
	}
}

func run(outDir string, list []platform) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "plugin.json")); err != nil {
		return fmt.Errorf("запускать из корня репозитория: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	for _, p := range list {
		out := filepath.Join(outDir, p.bundleName())
		if err := buildBundle(root, out, p); err != nil {
			return fmt.Errorf("%s/%s: %w", p.os, p.arch, err)
		}
		info, err := os.Stat(out)
		if err != nil {
			return err
		}
		fmt.Printf("%-32s %6.1f МБ\n", p.bundleName(), float64(info.Size())/(1<<20))
	}
	return nil
}

func buildBundle(root, outPath string, p platform) error {
	stage, err := os.MkdirTemp("", "xqs-rdp-pack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	if err := compile(root, filepath.Join(stage, p.binaryName()), p); err != nil {
		return err
	}
	if err := writeManifest(root, stage, p); err != nil {
		return err
	}
	if err := copyTree(filepath.Join(root, "ui"), filepath.Join(stage, "ui")); err != nil {
		return fmt.Errorf("копирование ui/: %w", err)
	}
	return pack(stage, outPath)
}

// compile собирает статический бинарь.
//
// CGO_ENABLED=0 обязателен: с cgo бинарь тянет зависимость от системной libc
// той машины, где собран, и на другой просто не запустится — а плагин должен
// работать у пользователя, а не у нас.
func compile(root, outPath string, p platform) error {
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", outPath, "./cmd/xqs-rdp")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+p.os,
		"GOARCH="+p.arch,
	)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// writeManifest кладёт plugin.json, поправив engine.entry под платформу.
//
// В манифесте поле одно, а имя бинаря на Windows отличается расширением.
// Разложить это по разным файлам нельзя — plugin.json один; значит правит
// упаковщик, и правит ровно одно поле, чтобы всё остальное осталось тем, что
// лежит в репозитории и читается глазами.
func writeManifest(root, stage string, p platform) error {
	raw, err := os.ReadFile(filepath.Join(root, "plugin.json"))
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("plugin.json: %w", err)
	}
	engine, ok := m["engine"].(map[string]any)
	if !ok {
		return fmt.Errorf("plugin.json: нет объекта engine")
	}
	engine["entry"] = p.binaryName()

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stage, "plugin.json"), append(out, '\n'), 0o644)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

type fileHash struct {
	name string
	hash string
}

// pack собирает ZIP и SHA256SUMS ровно так, как их читает ядро: имена через
// прямой слэш, строки вида "<hex>  <путь>", отсортированные по имени, сам файл
// сумм в себя не входит.
func pack(stage, outPath string) error {
	hashes, err := hashTree(stage)
	if err != nil {
		return err
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	for _, h := range hashes {
		if err := addFile(zw, filepath.Join(stage, filepath.FromSlash(h.name)), h.name); err != nil {
			return err
		}
	}
	if err := addBytes(zw, checksumsFile, []byte(renderChecksums(hashes))); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return out.Close()
}

func hashTree(root string) ([]fileHash, error) {
	var files []fileHash
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == checksumsFile {
			return nil
		}
		sum, err := hashFile(path)
		if err != nil {
			return err
		}
		files = append(files, fileHash{name: rel, hash: sum})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func renderChecksums(hashes []fileHash) string {
	var b strings.Builder
	for _, h := range hashes {
		b.WriteString(h.hash)
		b.WriteString("  ")
		b.WriteString(h.name)
		b.WriteByte('\n')
	}
	return b.String()
}

func addFile(zw *zip.Writer, srcPath, name string) error {
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.Method = zip.Deflate
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

func addBytes(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func parsePlatform(s string) (platform, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return platform{}, fmt.Errorf("платформа задаётся как os/arch, получено %q", s)
	}
	p := platform{os: parts[0], arch: parts[1]}
	for _, t := range targets {
		if t == p {
			return p, nil
		}
	}
	return platform{}, fmt.Errorf("неизвестная платформа %q", s)
}
