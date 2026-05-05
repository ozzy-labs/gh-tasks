package i18ncheck_test

// Edge / negative cases for the i18ncheck scanner. The happy paths and a
// representative non-ASCII set are covered by check_test.go; this file pins
// the boundary behavior of the Decorative whitelist, the parser-error path
// for ScanFile, the non-string AST nodes that must be skipped, the
// missing-root error path of Scan, and the truncation contract of FormatHit.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/i18ncheck"
)

func TestDecorative_RangeBoundaries(t *testing.T) {
	t.Parallel()

	// Every Decorative range is treated as a closed interval. We pin the
	// first and last rune of each whitelisted block (they must be true) and
	// one rune just outside each block (must be false). A regression that
	// flips an inequality (e.g. <= → <) is caught by the boundary cases.
	cases := []struct {
		name string
		r    rune
		want bool
	}{
		{"general-punctuation-start", 0x2010, true},
		{"general-punctuation-end", 0x2027, true},
		{"general-punctuation-just-after", 0x2028, false},
		{"arrows-start", 0x2190, true},
		{"arrows-end", 0x21FF, true},
		{"arrows-just-after", 0x2200, true}, // start of Math Operators
		{"math-operators-end", 0x22FF, true},
		{"math-operators-just-after", 0x2300, false},
		{"box-drawing-start", 0x2500, true},
		{"box-drawing-end", 0x257F, true},
		{"geometric-shapes-start", 0x25A0, true},
		{"geometric-shapes-end", 0x25FF, true},
		{"misc-symbols-start", 0x2600, true},
		{"misc-symbols-end", 0x26FF, true},
		{"dingbats-start", 0x2700, true},
		{"dingbats-end", 0x27BF, true},
		{"dingbats-just-after", 0x27C0, false},
		{"plain-ascii-A", 'A', false},
		{"japanese-ka", 'か', false}, // not whitelisted
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := i18ncheck.Decorative(tc.r)
			if got != tc.want {
				t.Errorf("Decorative(U+%04X) = %v, want %v", tc.r, got, tc.want)
			}
		})
	}
}

func TestScanFile_ParseErrorPropagates(t *testing.T) {
	t.Parallel()

	// A file that fails go/parser must return the underlying error rather
	// than silently producing 0 hits — that distinction matters because a
	// CI that swallows parse errors would mask a forgotten translation in a
	// file with a temporary syntax bug.
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.go")
	if err := os.WriteFile(path, []byte("package x\nfunc broken( {\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, err := i18ncheck.ScanFile(path)
	if err == nil {
		t.Fatalf("expected parse error, got hits=%v", hits)
	}
}

func TestScanFile_NonStringLiteralsAreIgnored(t *testing.T) {
	t.Parallel()

	// The scanner only flags string literals — int / float / char (rune)
	// literals are AST BasicLits too, but their Kind is INT / FLOAT / CHAR.
	// A regression that drops the Kind == STRING guard would start flagging
	// rune literals like 'こ' (which look non-ASCII but are not strings).
	dir := t.TempDir()
	src := "package x\n\nvar (\n\tN = 42\n\tF = 3.14\n\tR = 'こ'\n\tH = 0xFF\n)\n"
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, err := i18ncheck.ScanFile(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("non-string literals must not be flagged, got %+v", hits)
	}
}

func TestScanFile_RawStringLiteralsAreScanned(t *testing.T) {
	t.Parallel()

	// Backtick-delimited raw strings are still STRING-kind BasicLits. A
	// translation hardcoded in a raw string (e.g. for a multiline message
	// that escapes backslashes) must be flagged just like a "..." literal.
	dir := t.TempDir()
	src := "package x\n\nvar S = `こんにちは`\n"
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, err := i18ncheck.ScanFile(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("expected 1 hit on raw-string literal, got %d: %+v", len(hits), hits)
	}
}

func TestScan_MissingRootErrors(t *testing.T) {
	t.Parallel()

	// filepath.WalkDir on a non-existent root surfaces the os.Stat error to
	// the WalkDirFunc, which Scan must propagate. Pinning this prevents a
	// regression where Scan silently returns 0 hits for a typo'd path —
	// CI would falsely report "OK" on a misconfigured workflow.
	_, err := i18ncheck.Scan([]string{"/nonexistent-path-for-i18ncheck-test"})
	if err == nil {
		t.Error("expected error for missing root, got nil")
	}
}

func TestFormatHit_TruncatesLongPreview(t *testing.T) {
	t.Parallel()

	// FormatHit caps the preview at 80 chars (77 + "..."). A 200-char
	// literal must surface a truncation marker so terminal output stays
	// readable while still pinning that a hit was found.
	long := strings.Repeat("a", 200)
	h := i18ncheck.Hit{File: "x.go", Line: 1, Col: 1, Text: long}
	got := i18ncheck.FormatHit(h)
	if !strings.Contains(got, "...") {
		t.Errorf("expected truncation marker in %q", got)
	}
	if strings.Contains(got, strings.Repeat("a", 100)) {
		t.Errorf("expected preview to be capped, got long string in %q", got)
	}
}

func TestFormatHit_ShortPreviewUntouched(t *testing.T) {
	t.Parallel()

	// Mirror invariant: short literals must NOT pick up a "..." marker —
	// the truncation is a length-conditional branch that is easy to flip.
	h := i18ncheck.Hit{File: "x.go", Line: 1, Col: 1, Text: "short"}
	got := i18ncheck.FormatHit(h)
	if strings.Contains(got, "...") {
		t.Errorf("short text must not be marked truncated, got %q", got)
	}
	if !strings.Contains(got, `"short"`) {
		t.Errorf("expected literal text in %q", got)
	}
}
