// Catalog reference scanner: walks the same Go source tree as the hardcoded
// non-ASCII scanner and verifies that every `r.T("key", ...)` /
// `i18n.NewPayload("key", ...)` call refers to a key defined in the en/ja
// catalog (internal/i18n/{en,ja}.json). This catches the silent-fallback
// failure mode where i18n.T returns the key string verbatim when the entry
// is missing — pre-#256 only the hardcoded literal scanner ran, so a stale
// reference to a deleted key would ship undetected.
//
// AST-based detection (no semantic types resolved):
//   - SelectorExpr call where Sel.Name is "T" or "NewPayload"
//   - first call argument is a *ast.BasicLit of Kind STRING
//
// Non-string first arguments (variables, concatenations, function results)
// are recorded as DynamicRefs warnings — they are skipped from the catalog
// completeness check (we cannot statically resolve them) but surfaced to
// reviewers so dynamic key construction stays visible.
package i18ncheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
)

// CatalogRefs is the result of a catalog-reference scan.
type CatalogRefs struct {
	// Refs is the set of statically-known catalog keys referenced from a
	// scanned source file. Keys appear once regardless of citation count.
	Refs map[string]struct{}
	// Dynamic lists callsites where the first argument was not a string
	// literal (variable, concatenation, etc). These are skipped from the
	// completeness check but reported as warnings.
	Dynamic []DynamicRef
}

// DynamicRef is a callsite where the catalog key could not be statically
// resolved.
type DynamicRef struct {
	File   string
	Line   int
	Col    int
	Caller string // "T" or "NewPayload"
}

// MissingRef is a callsite that references a catalog key not defined in any
// of the configured catalogs.
type MissingRef struct {
	File   string
	Line   int
	Col    int
	Key    string
	Caller string // "T" or "NewPayload"
}

// scanFileRefs parses path and returns its reference set + dynamic warnings.
func scanFileRefs(path string) (map[string]locatedRef, []DynamicRef, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	refs := map[string]locatedRef{}
	dyn := []DynamicRef{}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		caller := sel.Sel.Name
		if caller != "T" && caller != "NewPayload" {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		first := call.Args[0]
		bl, ok := first.(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			pos := fset.Position(first.Pos())
			dyn = append(dyn, DynamicRef{
				File:   path,
				Line:   pos.Line,
				Col:    pos.Column,
				Caller: caller,
			})
			return true
		}
		key, err := strconv.Unquote(bl.Value)
		if err != nil {
			return true
		}
		pos := fset.Position(bl.Pos())
		// Keep the first observed location for this key in the file.
		if _, exists := refs[key]; !exists {
			refs[key] = locatedRef{
				File:   path,
				Line:   pos.Line,
				Col:    pos.Column,
				Caller: caller,
			}
		}
		return true
	})
	return refs, dyn, nil
}

type locatedRef struct {
	File   string
	Line   int
	Col    int
	Caller string
}

// ScanCatalogReferences walks roots and aggregates every statically-resolved
// `r.T("key")` / `i18n.NewPayload("key")` reference. The same files-walker
// rules as the hardcoded literal scanner apply: _test.go files are ignored,
// internal/i18n/ is skipped (catalog source), and dist / .claude / vendor /
// node_modules subtrees are pruned.
func ScanCatalogReferences(roots []string) (*CatalogRefs, error) {
	files, err := FindFiles(roots)
	if err != nil {
		return nil, err
	}
	out := &CatalogRefs{Refs: map[string]struct{}{}}
	for _, file := range files {
		refs, dyn, err := scanFileRefs(file)
		if err != nil {
			return nil, fmt.Errorf("scan refs %s: %w", file, err)
		}
		for k := range refs {
			out.Refs[k] = struct{}{}
		}
		out.Dynamic = append(out.Dynamic, dyn...)
	}
	return out, nil
}

// CheckCatalogReferences walks roots and returns every callsite whose
// catalog key is missing from the union of the supplied catalog keysets.
//
// catalogs is keyed by an arbitrary label (typically the locale name) so
// callers can render which catalog is missing a key in the diagnostic. A key
// must be present in **at least one** catalog to count as defined — the i18n
// runtime falls back en→key, so requiring presence in every locale would
// false-flag intentionally-untranslated keys.
//
// Returns the missing refs sorted by (File, Line, Col) for deterministic
// output, plus the same Dynamic slice from ScanCatalogReferences for warning
// rendering.
func CheckCatalogReferences(roots []string, catalogs map[string]map[string]struct{}) ([]MissingRef, []DynamicRef, error) {
	files, err := FindFiles(roots)
	if err != nil {
		return nil, nil, err
	}
	// Union of all configured catalogs is the "defined" set.
	defined := map[string]struct{}{}
	for _, keys := range catalogs {
		for k := range keys {
			defined[k] = struct{}{}
		}
	}
	missing := []MissingRef{}
	dyn := []DynamicRef{}
	for _, file := range files {
		refs, fileDyn, err := scanFileRefs(file)
		if err != nil {
			return nil, nil, fmt.Errorf("scan refs %s: %w", file, err)
		}
		dyn = append(dyn, fileDyn...)
		for key, loc := range refs {
			if _, ok := defined[key]; ok {
				continue
			}
			missing = append(missing, MissingRef{
				File:   loc.File,
				Line:   loc.Line,
				Col:    loc.Col,
				Key:    key,
				Caller: loc.Caller,
			})
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].File != missing[j].File {
			return missing[i].File < missing[j].File
		}
		if missing[i].Line != missing[j].Line {
			return missing[i].Line < missing[j].Line
		}
		return missing[i].Col < missing[j].Col
	})
	return missing, dyn, nil
}

// FormatMissingRef renders a MissingRef for human-readable error output.
func FormatMissingRef(m MissingRef) string {
	return fmt.Sprintf("%s:%d:%d  undefined i18n key  %q (via %s)",
		m.File, m.Line, m.Col, m.Key, m.Caller)
}

// FormatDynamicRef renders a DynamicRef for human-readable warning output.
func FormatDynamicRef(d DynamicRef) string {
	return fmt.Sprintf("%s:%d:%d  dynamic i18n key (skipped from catalog check) via %s",
		d.File, d.Line, d.Col, d.Caller)
}
