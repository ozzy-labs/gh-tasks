package install

import (
	"sort"
	"strings"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

// AgentsSnippetHeading is the H2 heading that opens the gh-tasks block in
// AGENTS.md. Pinned in this package so the codex-cli + gemini-cli
// adapters render byte-identical bodies — they share the same marker
// block in the consumer's AGENTS.md.
const AgentsSnippetHeading = "## gh-tasks Skills"

// RenderAgentsSnippet returns the body (no marker delimiters) of the
// gh-tasks skills block as it should appear inside AGENTS.md. locale
// selects which description field to render per skill; values other than
// "en" use the canonical (ja) description.
//
// The output is a deterministic function of the inputs: skills sorted by
// Name in byte order, two-line preamble (heading + blank), one bullet per
// skill. No trailing newline. Wrap with [MergeMarkerBlock] before writing
// to disk.
func RenderAgentsSnippet(in []skills.Skill, locale string) string {
	sorted := make([]skills.Skill, len(in))
	copy(sorted, in)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	lines := []string{AgentsSnippetHeading, ""}
	for _, s := range sorted {
		desc := s.Description
		if locale == "en" {
			desc = s.DescriptionEN
		}
		lines = append(lines, "- `"+s.Name+"` — "+desc)
	}
	return strings.Join(lines, "\n")
}
