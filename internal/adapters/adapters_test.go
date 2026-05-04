package adapters_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/adapters"
	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func sample() []skills.Skill {
	return []skills.Skill{
		{Name: "task-add", Description: "ja desc add", DescriptionEN: "en desc add", Locale: "ja", Raw: "raw add"},
		{Name: "task-plan", Description: "ja desc plan", DescriptionEN: "en desc plan", Locale: "ja", Raw: "raw plan"},
	}
}

func TestClaudeCode(t *testing.T) {
	t.Parallel()
	got := (adapters.ClaudeCode{}).Generate(sample())
	want := []skills.OutputFile{
		{RelativePath: ".claude/skills/task-add/SKILL.md", Content: "raw add"},
		{RelativePath: ".claude/skills/task-plan/SKILL.md", Content: "raw plan"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ClaudeCode (-want +got):\n%s", diff)
	}
}

func TestCodexCLI_IncludesAgentsSnippet(t *testing.T) {
	t.Parallel()
	got := (adapters.CodexCLI{}).Generate(sample())
	if len(got) != 3 {
		t.Fatalf("got %d outputs", len(got))
	}
	last := got[len(got)-1]
	if last.RelativePath != "AGENTS.md.snippet" {
		t.Errorf("expected last file to be AGENTS.md.snippet, got %q", last.RelativePath)
	}
	if !strings.Contains(last.Content, "## gh-tasks Skills") {
		t.Errorf("snippet missing header:\n%s", last.Content)
	}
	if !strings.Contains(last.Content, "<!-- begin: @ozzylabs/gh-tasks -->") {
		t.Errorf("snippet missing begin marker:\n%s", last.Content)
	}
}

func TestGeminiCLI_SettingsAndSnippet(t *testing.T) {
	t.Parallel()
	got := (adapters.GeminiCLI{}).Generate(sample())
	if got[0].RelativePath != ".gemini/settings.json" {
		t.Errorf("got %q", got[0].RelativePath)
	}
	if !strings.Contains(got[0].Content, `"AGENTS.md"`) {
		t.Errorf("settings missing AGENTS.md:\n%s", got[0].Content)
	}
	if got[1].RelativePath != "AGENTS.md.snippet" {
		t.Errorf("got %q", got[1].RelativePath)
	}
}

func TestCopilot_Snippet(t *testing.T) {
	t.Parallel()
	got := (adapters.Copilot{}).Generate(sample())
	if len(got) != 1 {
		t.Fatalf("got %d outputs", len(got))
	}
	if got[0].RelativePath != ".github/copilot-instructions.md.snippet" {
		t.Errorf("got %q", got[0].RelativePath)
	}
	if !strings.Contains(got[0].Content, "task-add") || !strings.Contains(got[0].Content, "task-plan") {
		t.Errorf("missing skill names:\n%s", got[0].Content)
	}
}

func TestAll_OrderingMatchesTS(t *testing.T) {
	t.Parallel()
	got := []string{}
	for _, a := range adapters.All() {
		got = append(got, a.ID())
	}
	want := []string{"claude-code", "codex-cli", "gemini-cli", "copilot"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("All() order (-want +got):\n%s", diff)
	}
}
