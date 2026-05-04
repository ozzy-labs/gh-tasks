package i18ncheck_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/i18ncheck"
)

func TestHasNonASCII(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"plain ascii":       false,
		"こんにちは":             true,
		"item → next":       false, // decorative arrow
		"a — b":             false, // em-dash
		"hello 世界":          true,
		"":                  false,
		"key.with.dots":     false,
		"emoji 🎉":           true, // non-decorative
		"box ┌──┐ drawing":  false,
		"café":              true,
		"naïve":             true,
		"Zürich":            true,
		"한국어":               true,
		"中文":                true,
		"only decorative ✓": false, // Dingbats whitelist
		"item ▀ block":      true,  // Block Elements is flagged (#144 reject)
	}
	for in, want := range cases {
		got := i18ncheck.HasNonASCII(in)
		if got != want {
			t.Errorf("HasNonASCII(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestScanFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := `package x

import "fmt"

func F() {
	fmt.Println("hello")        // ASCII, ok
	fmt.Println("こんにちは")        // hit
	fmt.Println("a → b")        // decorative-only
	const k = "world 🎉"          // hit
}
`
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, err := i18ncheck.ScanFile(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2: %+v", len(hits), hits)
	}
}

func TestScan_SkipsTestFilesAndI18n(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := map[string]string{
		"main.go":      "package x\n\nvar S = \"こんにちは\"\n",
		"main_test.go": "package x\n\nvar T = \"こんにちは\"\n",
		"i18n/i18n.go": "package i18n\n\nvar M = \"こんにちは\"\n",
	}
	for rel, body := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	hits, err := i18ncheck.Scan([]string{dir})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("got %d hits, want 1: %+v", len(hits), hits)
	}
	if filepath.Base(hits[0].File) != "main.go" {
		t.Errorf("hit on wrong file: %s", hits[0].File)
	}
}

func TestScanFile_PositionIs1Indexed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := "package x\n\nvar S = \"こんにちは\"\n"
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, err := i18ncheck.ScanFile(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("got %d hits", len(hits))
	}
	if hits[0].Line != 3 {
		t.Errorf("Line = %d, want 3 (1-indexed)", hits[0].Line)
	}
	if hits[0].Col != 9 {
		// "var S = " is 8 chars before the opening quote
		t.Errorf("Col = %d, want 9", hits[0].Col)
	}
}

func TestScanFile_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.go")
	if err := os.WriteFile(path, []byte("package x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, err := i18ncheck.ScanFile(path)
	if err != nil || len(hits) != 0 {
		t.Errorf("hits=%v err=%v", hits, err)
	}
}
