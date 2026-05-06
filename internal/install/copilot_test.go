package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func TestCopilot_ManifestPath(t *testing.T) {
	t.Parallel()
	root := "/tmp/repo"
	got := CopilotAdapter{}.ManifestPath(root)
	want := filepath.Join(root, ".github", ".gh-tasks-copilot-manifest.json")
	if got != want {
		t.Errorf("ManifestPath = %q, want %q", got, want)
	}
}

func TestCopilot_Plan_FreshTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	loaded := []skills.Skill{
		{Name: "task-add", Description: "追加"},
	}
	actions, err := CopilotAdapter{}.Plan(PlanContext{TargetRoot: root, Skills: loaded})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("got %d actions, want 1: %+v", len(actions), actions)
	}
	a := actions[0]
	if a.Type != ActionCreate {
		t.Errorf("Type = %v, want ActionCreate", a.Type)
	}
	if !a.Shared {
		t.Errorf("Shared = false, want true")
	}
	if a.RelPath != ".github/copilot-instructions.md" {
		t.Errorf("RelPath = %q", a.RelPath)
	}
	if !strings.Contains(a.Content, MarkerBeginLine) {
		t.Errorf("missing marker:\n%s", a.Content)
	}
}

func TestCopilot_Plan_PreservesUserContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	githubDir := filepath.Join(root, ".github")
	if err := os.MkdirAll(githubDir, 0o750); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(githubDir, "copilot-instructions.md"),
		"# Copilot project context\n\nUse 4-space indent.\n")

	loaded := []skills.Skill{{Name: "task-add", Description: "追加"}}
	actions, err := CopilotAdapter{}.Plan(PlanContext{TargetRoot: root, Skills: loaded})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) != 1 || actions[0].Type != ActionUpdate {
		t.Fatalf("expected 1 ActionUpdate, got %+v", actions)
	}
	if !strings.Contains(actions[0].Content, "Use 4-space indent.") {
		t.Errorf("user content lost:\n%s", actions[0].Content)
	}
	if !strings.Contains(actions[0].Content, "task-add") {
		t.Errorf("skill body missing:\n%s", actions[0].Content)
	}
}

func TestCopilot_Plan_SkipWhenIdentical(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	githubDir := filepath.Join(root, ".github")
	if err := os.MkdirAll(githubDir, 0o750); err != nil {
		t.Fatal(err)
	}
	loaded := []skills.Skill{{Name: "task-add", Description: "追加"}}
	body := RenderAgentsSnippet(loaded, "ja")
	merged := MergeMarkerBlock("", body)
	mustWriteFile(t, filepath.Join(githubDir, "copilot-instructions.md"), merged)

	actions, err := CopilotAdapter{}.Plan(PlanContext{TargetRoot: root, Skills: loaded})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) != 1 || actions[0].Type != ActionSkip {
		t.Errorf("expected ActionSkip, got %+v", actions)
	}
}

func TestCopilot_Plan_EmptyTargetErrors(t *testing.T) {
	t.Parallel()
	_, err := CopilotAdapter{}.Plan(PlanContext{Skills: nil})
	if err == nil {
		t.Errorf("expected error for empty TargetRoot")
	}
}
