package i18ncheck_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/i18ncheck"
)

// TestScanCatalogReferences_ExtractsLiteralKeys covers the happy path of the
// AST scanner: r.T("...") and i18n.NewPayload("...") with literal first
// arguments are collected, while non-literal first arguments produce a
// dynamic-warning entry instead of a key.
func TestScanCatalogReferences_ExtractsLiteralKeys(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Fixture exercises both SelectorExpr forms the scanner must catch:
	//   - method call: r.T("...")
	//   - package-qualified call: i18n.NewPayload("...")
	// The fake `i18n` and `r` symbols don't need to resolve — the scanner is
	// purely syntactic.
	src := `package x

type R struct{ Locale string }

func (r R) T(key string, args ...any) string { _ = key; _ = args; return "" }

type fakeI18n struct{}

func (fakeI18n) NewPayload(key string, args ...any) string { _ = key; _ = args; return "" }

var i18n = fakeI18n{}

func F(r R) {
	_ = r.T("list.empty")
	_ = r.T("error.repo.notFound", "owner", "x", "name", "y")
	_ = i18n.NewPayload("scope.invalid", "value", "bogus")
	dynamic := "dyn." + "key"
	_ = r.T(dynamic)
	other := "x"
	_ = other
}
`
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := i18ncheck.ScanCatalogReferences([]string{dir})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	wantKeys := []string{"list.empty", "error.repo.notFound", "scope.invalid"}
	for _, k := range wantKeys {
		if _, ok := got.Refs[k]; !ok {
			t.Errorf("missing key %q in refs %v", k, got.Refs)
		}
	}
	if len(got.Refs) != len(wantKeys) {
		t.Errorf("got %d refs, want %d: %v", len(got.Refs), len(wantKeys), got.Refs)
	}
	if len(got.Dynamic) != 1 {
		t.Fatalf("got %d dynamic refs, want 1: %+v", len(got.Dynamic), got.Dynamic)
	}
	if got.Dynamic[0].Caller != "T" {
		t.Errorf("Dynamic[0].Caller = %q, want T", got.Dynamic[0].Caller)
	}
}

// TestCheckCatalogReferences_DetectsUndefinedKey wires a fixture that holds a
// stale reference to a key not present in the supplied catalog. The scanner
// must surface the missing reference, while keys defined in either locale
// pass.
func TestCheckCatalogReferences_DetectsUndefinedKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := `package x

type R struct{}

func (r R) T(key string, args ...any) string { _ = key; _ = args; return "" }

func F(r R) {
	_ = r.T("good.key")
	_ = r.T("nonexistent.stale.key")
}
`
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	catalogs := map[string]map[string]struct{}{
		"en": {"good.key": {}},
		"ja": {"good.key": {}},
	}
	missing, _, err := i18ncheck.CheckCatalogReferences([]string{dir}, catalogs)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(missing) != 1 {
		t.Fatalf("got %d missing refs, want 1: %+v", len(missing), missing)
	}
	if missing[0].Key != "nonexistent.stale.key" {
		t.Errorf("missing key = %q, want nonexistent.stale.key", missing[0].Key)
	}
	if missing[0].Caller != "T" {
		t.Errorf("missing caller = %q, want T", missing[0].Caller)
	}
}

// TestCheckCatalogReferences_KeyDefinedInOnlyOneLocaleCounts asserts that a
// key present in the en catalog but missing from ja is treated as defined.
// The i18n runtime falls back en→key for missing translations, so requiring
// every key to exist in every locale would false-flag intentionally
// untranslated strings.
func TestCheckCatalogReferences_KeyDefinedInOnlyOneLocaleCounts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := `package x

type R struct{}

func (r R) T(key string, args ...any) string { _ = key; _ = args; return "" }

func F(r R) { _ = r.T("only.in.en") }
`
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	catalogs := map[string]map[string]struct{}{
		"en": {"only.in.en": {}},
		"ja": {},
	}
	missing, _, err := i18ncheck.CheckCatalogReferences([]string{dir}, catalogs)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("got %d missing, want 0: %+v", len(missing), missing)
	}
}

// TestCheckCatalogReferences_DynamicKeyIsNotMissing verifies that non-literal
// first arguments are funneled into Dynamic rather than treated as missing
// catalog references — we cannot statically resolve them, so they must not
// fail the gate.
func TestCheckCatalogReferences_DynamicKeyIsNotMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := `package x

type R struct{}

func (r R) T(key string, args ...any) string { _ = key; _ = args; return "" }

func F(r R, dynamic string) { _ = r.T(dynamic) }
`
	path := filepath.Join(dir, "src.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	catalogs := map[string]map[string]struct{}{
		"en": {},
		"ja": {},
	}
	missing, dynamic, err := i18ncheck.CheckCatalogReferences([]string{dir}, catalogs)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("dynamic key surfaced as missing: %+v", missing)
	}
	if len(dynamic) != 1 {
		t.Errorf("got %d dynamic refs, want 1: %+v", len(dynamic), dynamic)
	}
}

// TestCheckCatalogReferences_RealCatalogPasses is the integration assert: the
// real internal/i18n catalog plus the live cmd/ + internal/ source must scan
// clean. When this fails on main, it means a recent change either deleted a
// catalog key still referenced by code (silent fallback) or introduced an
// r.T call to a key that was never defined.
func TestCheckCatalogReferences_RealCatalogPasses(t *testing.T) {
	t.Parallel()

	// Resolve the repo root from the test file's location: this file lives
	// at <root>/internal/i18ncheck/refs_test.go.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	cmdDir := filepath.Join(root, "cmd")
	internalDir := filepath.Join(root, "internal")

	catalogs := map[string]map[string]struct{}{
		string(i18n.LocaleEN): i18n.Keys(i18n.LocaleEN),
		string(i18n.LocaleJA): i18n.Keys(i18n.LocaleJA),
	}
	missing, _, err := i18ncheck.CheckCatalogReferences([]string{cmdDir, internalDir}, catalogs)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(missing) != 0 {
		var sb strings.Builder
		for _, m := range missing {
			sb.WriteString(i18ncheck.FormatMissingRef(m))
			sb.WriteString("\n")
		}
		t.Errorf("real catalog has %d undefined references:\n%s", len(missing), sb.String())
	}
}

// TestFormatMissingRef checks the diagnostic format stays stable so CI log
// scrapers and the lefthook hook output keep parsing the same shape.
func TestFormatMissingRef(t *testing.T) {
	t.Parallel()

	got := i18ncheck.FormatMissingRef(i18ncheck.MissingRef{
		File:   "cmd/foo.go",
		Line:   42,
		Col:    7,
		Key:    "stale.key",
		Caller: "T",
	})
	want := `cmd/foo.go:42:7  undefined i18n key  "stale.key" (via T)`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestFormatDynamicRef pins the warning shape.
func TestFormatDynamicRef(t *testing.T) {
	t.Parallel()

	got := i18ncheck.FormatDynamicRef(i18ncheck.DynamicRef{
		File:   "cmd/foo.go",
		Line:   42,
		Col:    7,
		Caller: "NewPayload",
	})
	want := "cmd/foo.go:42:7  dynamic i18n key (skipped from catalog check) via NewPayload"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
