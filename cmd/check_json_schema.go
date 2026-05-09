package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/jsonout"
)

// newCheckJSONSchemaCmd registers the Hidden `check-json-schema` command.
// It prints every public --json catalog as a markdown-friendly table so
// docs/manual/{en,ja}/reference/json-output.md can be regenerated (or
// diffed) without copy-pasting field metadata by hand. Hidden because
// it is dev-only — end-users do not need this surface.
func newCheckJSONSchemaCmd(_ Deps) *cobra.Command {
	c := &cobra.Command{
		Use:    "check-json-schema",
		Short:  "Print --json catalogs as markdown (dev only)",
		Hidden: true,
		RunE: func(c *cobra.Command, _ []string) error {
			out := c.OutOrStdout()
			for _, e := range jsonSchemaCatalogs() {
				fmt.Fprintf(out, "### `%s`\n\n", e.Name)
				fmt.Fprintln(out, "| Field | Description |")
				fmt.Fprintln(out, "| --- | --- |")
				for _, f := range e.Catalog {
					fmt.Fprintf(out, "| `%s` | %s |\n", f.Name, f.Description)
				}
				fmt.Fprintln(out)
			}
			return nil
		},
	}
	return c
}

// jsonSchemaCatalogs returns the curated list of catalogs to render.
// New catalogs added under cmd/jsonpath.go must be appended here so the
// dev tool stays a single source of truth for the user-facing reference.
type jsonSchemaCatalogEntry struct {
	Name    string
	Catalog jsonout.FieldList
}

func jsonSchemaCatalogs() []jsonSchemaCatalogEntry {
	return []jsonSchemaCatalogEntry{
		{Name: "item", Catalog: itemJSONFields},
		{Name: "activity", Catalog: activityJSONFields},
		{Name: "link", Catalog: linkJSONFields},
		{Name: "projectInit", Catalog: projectInitJSONFields},
	}
}
