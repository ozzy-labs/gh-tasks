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

// AgentsMdScaffold is the minimal consumer-owned preamble we lay down
// when AGENTS.md does not yet exist, so a freshly-created file does not
// open with our `## gh-tasks Skills` heading and has a proper H1 plus a
// hint for the project to record its own agent instructions.
//
// The scaffold lives strictly outside the gh-tasks marker block; once
// written it is never re-examined by install-skills (subsequent runs
// only rewrite the marker block, leaving every byte outside it intact).
// Uninstall similarly only excises the marker block — the scaffold,
// being consumer-owned, remains for the project to extend or delete.
const AgentsMdScaffold = "# AGENTS.md\n\n<!-- Add project-level agent instructions here. -->\n"

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
