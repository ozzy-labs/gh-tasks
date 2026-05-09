package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/jsonout"
)

// jsonSchemaDocs are the user-facing references whose catalog tables are
// auto-generated. The marker pair `<!-- begin: jsonout-catalog NAME -->`
// ... `<!-- end: jsonout-catalog NAME -->` is replaced in place; content
// outside the markers (incl. surrounding prose) is byte-for-byte
// preserved.
var jsonSchemaDocs = []string{
	"docs/manual/en/reference/json-output.md",
	"docs/manual/ja/reference/json-output.md",
}

// newCheckJSONSchemaCmd registers the Hidden `check-json-schema` command.
// It either prints every public --json catalog as a markdown-friendly
// table (default), updates the user-facing reference docs in place
// (`--update`), or fails when the docs drift from the in-source catalogs
// (`--check`). Hidden because it is dev-only — end users do not need
// this surface; CI / pre-commit hooks call it via the `--check` mode.
func newCheckJSONSchemaCmd(_ Deps) *cobra.Command {
	c := &cobra.Command{
		Use:    "check-json-schema",
		Short:  "Print --json catalogs as markdown (dev only)",
		Hidden: true,
		RunE: func(c *cobra.Command, _ []string) error {
			update, _ := c.Flags().GetBool("update")
			check, _ := c.Flags().GetBool("check")
			if update && check {
				return errors.New("--update and --check are mutually exclusive")
			}
			if update {
				return runCheckJSONSchemaUpdate(c)
			}
			if check {
				return runCheckJSONSchemaCheck(c)
			}
			fmt.Fprint(c.OutOrStdout(), renderAllCatalogs())
			return nil
		},
	}
	c.Flags().Bool("update", false, "rewrite the catalog tables in user-facing docs (in-place, idempotent)")
	c.Flags().Bool("check", false, "exit non-zero when the docs drift from the in-source catalogs")
	return c
}

// jsonSchemaCatalogEntry pairs a catalog with the slug used inside
// markdown markers (`<!-- begin: jsonout-catalog SLUG -->`).
type jsonSchemaCatalogEntry struct {
	Name    string
	Catalog jsonout.FieldList
}

// jsonSchemaCatalogs returns the curated list in canonical order. New
// catalogs added under cmd/jsonpath.go must be appended here so the dev
// tool stays a single source of truth for the user-facing reference.
func jsonSchemaCatalogs() []jsonSchemaCatalogEntry {
	return []jsonSchemaCatalogEntry{
		{Name: "item", Catalog: itemJSONFields},
		{Name: "activity", Catalog: activityJSONFields},
		{Name: "link", Catalog: linkJSONFields},
		{Name: "projectInit", Catalog: projectInitJSONFields},
	}
}

// renderAllCatalogs returns the legacy "print everything" output used
// when neither --update nor --check is set. Each catalog gets a
// `### \`name\“ heading + a 3-column table.
func renderAllCatalogs() string {
	var buf bytes.Buffer
	for _, e := range jsonSchemaCatalogs() {
		fmt.Fprintf(&buf, "### `%s`\n\n", e.Name)
		buf.WriteString(renderCatalogTable(e.Catalog))
		buf.WriteString("\n")
	}
	return buf.String()
}

// renderCatalogTable returns the markdown table body (header + rows)
// for a single catalog. No surrounding heading; the caller decides the
// slug context. Placeholder `—` is used for missing types so the column
// renders cleanly. Pipe characters in Type / Description are escaped so
// they do not collide with the table-cell separator (e.g. `object |
// null` → `object \| null`).
func renderCatalogTable(catalog jsonout.FieldList) string {
	var buf bytes.Buffer
	buf.WriteString("| Field | Type | Notes |\n")
	buf.WriteString("| --- | --- | --- |\n")
	for _, f := range catalog {
		typ := f.Type
		if typ == "" {
			typ = "—"
		}
		fmt.Fprintf(&buf, "| `%s` | %s | %s |\n", f.Name, escapeMDPipes(typ), escapeMDPipes(f.Description))
	}
	return buf.String()
}

// escapeMDPipes escapes `|` characters that would otherwise collide with
// markdown table cell separators.
func escapeMDPipes(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// runCheckJSONSchemaUpdate rewrites every doc in jsonSchemaDocs in
// place, replacing the body inside each `<!-- begin: jsonout-catalog
// SLUG -->` ... `<!-- end: jsonout-catalog SLUG -->` pair with the
// freshly-rendered table for that catalog.
func runCheckJSONSchemaUpdate(c *cobra.Command) error {
	for _, path := range jsonSchemaDocs {
		fullPath := resolveDocPath(path)
		// Hidden dev tool over a fixed const list of repo-relative
		// paths — G304 / G306 are false positives (we want the docs
		// world-readable).
		raw, err := os.ReadFile(fullPath) //nolint:gosec
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		updated, _, err := rewriteCatalogMarkers(string(raw))
		if err != nil {
			return fmt.Errorf("rewrite %s: %w", path, err)
		}
		if string(raw) != updated {
			if err := os.WriteFile(fullPath, []byte(updated), 0o644); err != nil { //nolint:gosec
				return fmt.Errorf("write %s: %w", path, err)
			}
			fmt.Fprintf(c.ErrOrStderr(), "updated: %s\n", path)
		}
	}
	return nil
}

// runCheckJSONSchemaCheck reports drift without writing. Returns
// ErrSilentRuntime so cobra exits non-zero and the CI / pre-commit hook
// can flag the regression. The actual diff is written to stderr so the
// human running the hook sees what would change.
func runCheckJSONSchemaCheck(c *cobra.Command) error {
	drift := false
	for _, path := range jsonSchemaDocs {
		fullPath := resolveDocPath(path)
		raw, err := os.ReadFile(fullPath) //nolint:gosec
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		updated, _, err := rewriteCatalogMarkers(string(raw))
		if err != nil {
			return fmt.Errorf("check %s: %w", path, err)
		}
		if string(raw) != updated {
			fmt.Fprintf(c.ErrOrStderr(), "drift detected in %s — run `gh tasks check-json-schema --update` to regenerate.\n", path)
			drift = true
		}
	}
	if drift {
		return ErrSilentRuntime
	}
	return nil
}

// resolveDocPath turns a repo-relative path into an absolute path. The
// dev tool runs from the repo root in CI / pre-commit, so this is a
// straight identity unless something invokes the binary from
// elsewhere. Kept as a function so future callers can swap in a custom
// resolver (tests use a tempdir-rooted variant).
func resolveDocPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return path
}

// rewriteCatalogMarkers replaces the body inside every catalog marker
// pair with a freshly-rendered table. Returns the rewritten content
// and the number of markers that were processed. Unknown slugs (i.e.
// markers naming a catalog not in jsonSchemaCatalogs) are left
// untouched so future markers can be added without a hard-coupled
// release order.
func rewriteCatalogMarkers(src string) (string, int, error) {
	bySlug := map[string]jsonout.FieldList{}
	for _, e := range jsonSchemaCatalogs() {
		bySlug[e.Name] = e.Catalog
	}

	out := src
	processed := 0
	for slug, catalog := range bySlug {
		begin := fmt.Sprintf("<!-- begin: jsonout-catalog %s -->", slug)
		end := fmt.Sprintf("<!-- end: jsonout-catalog %s -->", slug)
		for {
			beginIdx := strings.Index(out, begin)
			if beginIdx < 0 {
				break
			}
			endIdx := strings.Index(out[beginIdx:], end)
			if endIdx < 0 {
				return "", processed, fmt.Errorf("missing closing marker for slug %q", slug)
			}
			endIdx += beginIdx
			before := out[:beginIdx+len(begin)]
			after := out[endIdx:]
			body := "\n\n" + renderCatalogTable(catalog) + "\n"
			next := before + body + after
			if next == out {
				break
			}
			out = next
			processed++
		}
	}
	return out, processed, nil
}
