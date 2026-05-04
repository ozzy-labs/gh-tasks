// Package i18ncheck scans Go source files for non-ASCII string literals that
// almost always signal a forgotten translation key. CLI text MUST live in
// internal/i18n/{en,ja}.json and be retrieved via i18n.T (repo-internal
// ADR-0005).
//
// Comments are not scanned. A small whitelist of decorative / structural
// Unicode characters (arrows, em-dashes, ellipses, math operators, bullets) is
// stripped before the scan so source code can use arrows in formatted output
// without flagging.
package i18ncheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Hit is a single violation: a non-ASCII string literal in a Go source file.
type Hit struct {
	File string
	Line int
	Col  int
	Text string
}

// Decorative returns whether r is a structural / decorative Unicode rune that
// is allowed in source code.
func Decorative(r rune) bool {
	// Block ranges:
	//   General Punctuation (U+2010..U+2027): hyphens, dashes, quotes, ellipsis
	//   Arrows (U+2190..U+21FF)
	//   Mathematical Operators (U+2200..U+22FF)
	//   Box Drawing (U+2500..U+257F)
	//   Geometric Shapes (U+25A0..U+25FF)
	//   Misc Symbols (U+2600..U+26FF)
	//   Dingbats (U+2700..U+27BF) — checkmarks, crosses, status indicators
	switch {
	case r >= 0x2010 && r <= 0x2027:
		return true
	case r >= 0x2190 && r <= 0x21FF:
		return true
	case r >= 0x2200 && r <= 0x22FF:
		return true
	case r >= 0x2500 && r <= 0x257F:
		return true
	case r >= 0x25A0 && r <= 0x25FF:
		return true
	case r >= 0x2600 && r <= 0x26FF:
		return true
	case r >= 0x2700 && r <= 0x27BF:
		return true
	}
	return false
}

// HasNonASCII reports whether s contains any non-decorative non-ASCII rune.
func HasNonASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII && !Decorative(r) {
			return true
		}
	}
	return false
}

// FindFiles walks roots and returns Go source files (excluding _test.go files
// and any path containing /i18n/, since those packages legitimately embed the
// catalog text).
func FindFiles(roots []string) ([]string, error) {
	out := []string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if name == "vendor" || name == "node_modules" || name == ".git" || name == "dist" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") {
				return nil
			}
			if strings.HasSuffix(p, "_test.go") {
				return nil
			}
			if strings.Contains(p, string(filepath.Separator)+"i18n"+string(filepath.Separator)) {
				return nil
			}
			out = append(out, p)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// ScanFile parses path and returns any non-ASCII string literal hits.
func ScanFile(path string) ([]Hit, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	hits := []Hit{}
	ast.Inspect(f, func(n ast.Node) bool {
		bl, ok := n.(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			return true
		}
		v, err := strconv.Unquote(bl.Value)
		if err != nil {
			return true
		}
		if !HasNonASCII(v) {
			return true
		}
		pos := fset.Position(bl.Pos())
		hits = append(hits, Hit{
			File: path,
			Line: pos.Line,
			Col:  pos.Column,
			Text: v,
		})
		return true
	})
	return hits, nil
}

// Scan returns all hits across the given roots.
func Scan(roots []string) ([]Hit, error) {
	files, err := FindFiles(roots)
	if err != nil {
		return nil, err
	}
	all := []Hit{}
	for _, file := range files {
		hits, err := ScanFile(file)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", file, err)
		}
		all = append(all, hits...)
	}
	return all, nil
}

// FormatHit renders a hit for human-readable error output.
func FormatHit(h Hit) string {
	preview := h.Text
	if len(preview) > 80 {
		preview = preview[:77] + "..."
	}
	return fmt.Sprintf("%s:%d:%d  hardcoded non-ASCII literal  %q",
		h.File, h.Line, h.Col, preview)
}
