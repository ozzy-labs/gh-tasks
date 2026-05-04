// Package adapters renders skill bundles into per-agent output files
// (claude / codex / copilot / gemini). Each adapter is a pure function: it
// takes the canonical skill list and returns OutputFiles rooted under its own
// `dist/{id}/` subtree. The build orchestrator clears the destination and
// writes the files.
package adapters

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

// Adapter is the contract every per-agent renderer implements.
type Adapter interface {
	ID() string
	Generate(s []skills.Skill) []skills.OutputFile
}

// All returns the four adapters in the order the orchestrator runs them.
func All() []Adapter {
	return []Adapter{
		ClaudeCode{},
		CodexCLI{},
		GeminiCLI{},
		Copilot{},
	}
}

// snippetTag is the marker tag wrapping snippet blocks merged into
// consumer-owned files (AGENTS.md, copilot-instructions.md).
const snippetTag = "@ozzylabs/gh-tasks"

func snippetBegin() string { return "<!-- begin: " + snippetTag + " -->" }
func snippetEnd() string   { return "<!-- end: " + snippetTag + " -->" }

// wrapSnippet inserts a single blank line on each side so the output is
// idempotent under Prettier's Markdown formatter.
func wrapSnippet(body string) string {
	trimmed := strings.TrimRight(body, "\n")
	return snippetBegin() + "\n\n" + trimmed + "\n\n" + snippetEnd() + "\n"
}

func sortByName(in []skills.Skill) []skills.Skill {
	out := make([]skills.Skill, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// renderAgentsMdSnippet renders the `## gh-tasks Skills` block consumed by
// AGENTS.md adapters (codex-cli + gemini-cli).
func renderAgentsMdSnippet(in []skills.Skill, locale string) string {
	sorted := sortByName(in)
	lines := []string{"## gh-tasks Skills", ""}
	for _, s := range sorted {
		desc := s.Description
		if locale == "en" {
			desc = s.DescriptionEN
		}
		lines = append(lines, "- `"+s.Name+"` — "+desc)
	}
	return wrapSnippet(strings.Join(lines, "\n"))
}

// ClaudeCode emits the canonical SKILL.md verbatim under `.claude/skills/`.
type ClaudeCode struct{}

// ID returns the adapter identifier.
func (ClaudeCode) ID() string { return "claude-code" }

// Generate returns one SKILL.md per skill, sorted by name.
func (ClaudeCode) Generate(in []skills.Skill) []skills.OutputFile {
	sorted := sortByName(in)
	out := make([]skills.OutputFile, 0, len(sorted))
	for _, s := range sorted {
		out = append(out, skills.OutputFile{
			RelativePath: ".claude/skills/" + s.Name + "/SKILL.md",
			Content:      s.Raw,
		})
	}
	return out
}

// CodexCLI emits SKILL.md files under `.agents/skills/` plus an
// AGENTS.md.snippet.
type CodexCLI struct{}

// ID returns the adapter identifier.
func (CodexCLI) ID() string { return "codex-cli" }

// Generate returns one SKILL.md per skill plus the AGENTS.md.snippet.
func (CodexCLI) Generate(in []skills.Skill) []skills.OutputFile {
	sorted := sortByName(in)
	out := make([]skills.OutputFile, 0, len(sorted)+1)
	for _, s := range sorted {
		out = append(out, skills.OutputFile{
			RelativePath: ".agents/skills/" + s.Name + "/SKILL.md",
			Content:      s.Raw,
		})
	}
	out = append(out, skills.OutputFile{
		RelativePath: "AGENTS.md.snippet",
		Content:      renderAgentsMdSnippet(sorted, "ja"),
	})
	return out
}

// GeminiCLI emits .gemini/settings.json + AGENTS.md.snippet.
type GeminiCLI struct{}

// ID returns the adapter identifier.
func (GeminiCLI) ID() string { return "gemini-cli" }

// Generate returns the gemini settings file and the AGENTS.md snippet.
func (GeminiCLI) Generate(in []skills.Skill) []skills.OutputFile {
	settings := map[string]any{
		"context": map[string]any{
			"fileName": []string{"AGENTS.md"},
		},
	}
	settingsJSON, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		panic(err)
	}
	return []skills.OutputFile{
		{RelativePath: ".gemini/settings.json", Content: string(settingsJSON) + "\n"},
		{RelativePath: "AGENTS.md.snippet", Content: renderAgentsMdSnippet(sortByName(in), "ja")},
	}
}

// Copilot emits a single `.github/copilot-instructions.md.snippet` listing
// each skill name + description.
type Copilot struct{}

// ID returns the adapter identifier.
func (Copilot) ID() string { return "copilot" }

// Generate returns the copilot snippet.
func (Copilot) Generate(in []skills.Skill) []skills.OutputFile {
	sorted := sortByName(in)
	lines := []string{"## gh-tasks Skills", ""}
	for _, s := range sorted {
		lines = append(lines, "- `"+s.Name+"` — "+s.Description)
	}
	body := wrapSnippet(strings.Join(lines, "\n"))
	return []skills.OutputFile{
		{RelativePath: ".github/copilot-instructions.md.snippet", Content: body},
	}
}
