package jsonout

import (
	"encoding/json"
	"fmt"
	"io"
)

// Render serializes items as a JSON array to w, optionally restricted to
// the requested field names and post-processed by a gojq query.
//
//   - items: caller-built rows whose keys are camelCase and match catalog
//     names. Empty slice produces `[]`.
//   - fields: when non-empty, only those keys are kept (in catalog order so
//     output is stable across runs). Names not in the catalog produce
//     UnknownFieldError.
//   - jq: optional gojq expression applied to the (possibly filtered) array.
//   - catalog: complete field list used for validation. Must be non-empty.
//
// Output is two-space indented JSON. Caller is responsible for stream
// separation: stdout for data only; localized warnings stay on stderr per
// `docs/design/json-output.md`.
func Render(w io.Writer, items []map[string]any, fields []string, jq string, catalog FieldList) error {
	if len(catalog) == 0 {
		return fmt.Errorf("jsonout.Render: empty catalog")
	}
	if unknown := catalog.Validate(fields); len(unknown) > 0 {
		return &UnknownFieldError{Fields: unknown}
	}

	out := projectFields(items, fields, catalog)
	if jq != "" {
		// gojq operates on Go's reflect-friendly generic JSON shapes
		// (`[]any`, `map[string]any`, primitives). `[]map[string]any` is
		// not assignable to `[]any` directly, so we round-trip through
		// json.Marshal/Unmarshal to land on the canonical types gojq
		// expects. The cost is one extra encode pass per --jq invocation,
		// which is negligible for typical task lists.
		buf, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("marshal items for jq: %w", err)
		}
		var data any
		if err := json.Unmarshal(buf, &data); err != nil {
			return fmt.Errorf("unmarshal items for jq: %w", err)
		}
		return runJQ(w, data, jq)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// projectFields returns items filtered to keep only the requested keys.
// When fields is empty, all catalog keys are kept. Missing keys are emitted
// as JSON `null` so selected fields always appear (see json-output.md
// "null vs omitempty"). Output key order is alphabetical because that is
// how `encoding/json` serializes `map[string]any` — and how `gh` itself
// orders `--json` output, so our contract matches that convention.
func projectFields(items []map[string]any, fields []string, catalog FieldList) []map[string]any {
	keep := fields
	if len(keep) == 0 {
		keep = make([]string, len(catalog))
		for i, f := range catalog {
			keep[i] = f.Name
		}
	}
	out := make([]map[string]any, len(items))
	for i, item := range items {
		row := make(map[string]any, len(keep))
		for _, name := range keep {
			if v, ok := item[name]; ok {
				row[name] = v
			} else {
				row[name] = nil
			}
		}
		out[i] = row
	}
	return out
}
