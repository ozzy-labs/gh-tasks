// Package jsonout provides the shared `--json [fields]` / `--jq <query>`
// rendering pipeline for `gh tasks` commands. The package is intentionally
// agnostic of the cobra command and the GraphQL DTOs — callers pass already-
// constructed `[]map[string]any` items keyed by the camelCase field names
// listed in their FieldList catalog. See `docs/design/json-output.md` for
// the contract this package implements (#367).
package jsonout

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Field describes one publicly-exposed JSON output field for a command.
type Field struct {
	// Name is the JSON output key (camelCase, English only).
	Name string
	// Type is the JSON value type as it would appear in TypeScript / JSON
	// Schema notation. Examples: `string`, `int`, `string | null`,
	// `object | null`, `array`. Empty string means "unspecified" (used by
	// older catalogs before the Type field landed); the schema dump renders
	// such rows with a placeholder dash.
	Type string
	// Description is a one-line English explanation surfaced when the user
	// runs `--json` with no argument. Keep ASCII-only; this is part of the
	// CLI surface, not localized message catalog.
	Description string
}

// FieldList is the per-command catalog of selectable JSON fields. Order is
// preserved when ListFields prints the catalog (callers should declare the
// list in the most useful reading order).
type FieldList []Field

// Has reports whether name is a known field.
func (l FieldList) Has(name string) bool {
	for _, f := range l {
		if f.Name == name {
			return true
		}
	}
	return false
}

// Validate returns the names from requested that are not in the catalog.
// The returned slice is sorted alphabetically for stable error messages.
func (l FieldList) Validate(requested []string) []string {
	var unknown []string
	for _, name := range requested {
		if !l.Has(name) {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	return unknown
}

// ListFields prints the catalog to w in a stable, gh-style format. Width is
// padded to the longest field name so descriptions align in a fixed column.
func ListFields(w io.Writer, catalog FieldList) {
	fmt.Fprintln(w, "Specify one or more comma-separated fields for `--json`:")
	maxName := 0
	for _, f := range catalog {
		if len(f.Name) > maxName {
			maxName = len(f.Name)
		}
	}
	for _, f := range catalog {
		fmt.Fprintf(w, "  %-*s  %s\n", maxName, f.Name, f.Description)
	}
}

// UnknownFieldError signals that one or more requested fields are missing
// from the catalog. Callers (cmd/*) typically render the catalog via
// ListFields when they receive this error.
type UnknownFieldError struct {
	Fields []string
}

func (e *UnknownFieldError) Error() string {
	return fmt.Sprintf("unknown JSON field(s): %s", strings.Join(e.Fields, ", "))
}

// CompleteFields returns shell-completion candidates for the comma-
// separated `--json` value the user is editing. `current` is the raw
// flag value seen by cobra; the helper handles the trailing token,
// excludes fields that already appear earlier in the list, and returns
// each candidate as `<prefix>,<remaining-name>` so the shell drops the
// completion in place. When current has no commas yet, the prefix is
// empty and bare field names are returned.
func CompleteFields(catalog FieldList, current string) []string {
	parts := strings.Split(current, ",")
	prefix := ""
	if len(parts) > 1 {
		prefix = strings.Join(parts[:len(parts)-1], ",") + ","
	}
	tail := strings.TrimSpace(parts[len(parts)-1])
	used := map[string]bool{}
	for _, p := range parts[:len(parts)-1] {
		used[strings.TrimSpace(p)] = true
	}
	var out []string
	for _, f := range catalog {
		if used[f.Name] {
			continue
		}
		if tail == "" || strings.HasPrefix(f.Name, tail) {
			out = append(out, prefix+f.Name)
		}
	}
	return out
}
