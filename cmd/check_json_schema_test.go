package cmd_test

import (
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

// TestCheckJSONSchema_PrintsAllCatalogs pins the dev tool: every public
// `--json` catalog name appears as a markdown heading in the output, in
// the canonical order item → activity → link → projectInit. New catalogs
// must be registered in jsonSchemaCatalogs to keep the docs reference
// regenerable.
func TestCheckJSONSchema_PrintsAllCatalogs(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "check-json-schema")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"### `item`", "### `activity`", "### `link`", "### `projectInit`"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in catalog output:\n%s", want, got)
		}
	}
	// Order check: item must come before activity, etc.
	indices := []int{
		strings.Index(got, "### `item`"),
		strings.Index(got, "### `activity`"),
		strings.Index(got, "### `link`"),
		strings.Index(got, "### `projectInit`"),
	}
	for i := 1; i < len(indices); i++ {
		if indices[i-1] >= indices[i] {
			t.Errorf("catalog order broken at index %d: indices=%v", i, indices)
		}
	}
}
