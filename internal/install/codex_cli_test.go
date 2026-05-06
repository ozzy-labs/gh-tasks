package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func TestCodexCLI_ManifestPath(t *testing.T) {
	t.Parallel()
	root := "/tmp/repo"
	got := CodexCLIAdapter{}.ManifestPath(root)
	want := filepath.Join(root, ".agents", "skills", ".gh-tasks-manifest.json")
	if got != want {
		t.Errorf("ManifestPath = %q, want %q", got, want)
	}
}

func TestCodexCLI_Plan_FreshTarget(t *testing.T) {
	// Empty target → all skill files are Create + AGENTS.md is Create
	// (Shared=true) because there is no existing aggregator.
	t.Parallel()
	root := t.TempDir()
	loaded := []skills.Skill{
		{Name: "task-add", Raw: "raw-add\n", Description: "追加"},
		{Name: "task-plan", Raw: "raw-plan\n", Description: "計画"},
	}
	actions, err := CodexCLIAdapter{}.Plan(PlanContext{
		TargetRoot: root, Skills: loaded,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// 2 skill SKILL.md files + 1 AGENTS.md = 3 actions.
	if len(actions) != 3 {
		t.Fatalf("got %d actions, want 3:\n%+v", len(actions), actions)
	}
	var sharedSeen int
	for _, a := range actions {
		if a.Type != ActionCreate {
			t.Errorf("expected ActionCreate, got %v for %s", a.Type, a.RelPath)
		}
		if a.Shared {
			sharedSeen++
			if a.RelPath != "AGENTS.md" {
				t.Errorf("Shared action has unexpected RelPath %q", a.RelPath)
			}
			if !strings.Contains(a.Content, MarkerBeginLine) ||
				!strings.Contains(a.Content, MarkerEndLine) {
				t.Errorf("AGENTS.md content lacks markers:\n%s", a.Content)
			}
		}
	}
	if sharedSeen != 1 {
		t.Errorf("expected exactly 1 Shared action, got %d", sharedSeen)
	}
}

func TestCodexCLI_Plan_AgentsMd_PreservesUserContent(t *testing.T) {
	// Pre-existing AGENTS.md with no marker block must be preserved
	// verbatim (above the appended marker block).
	t.Parallel()
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "AGENTS.md"), "# Project AGENTS\n\nUser preamble.\n")

	loaded := []skills.Skill{
		{Name: "task-add", Raw: "raw\n", Description: "追加"},
	}
	actions, err := CodexCLIAdapter{}.Plan(PlanContext{
		TargetRoot: root, Skills: loaded,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	var agentsAct *Action
	for i := range actions {
		if actions[i].RelPath == "AGENTS.md" {
			agentsAct = &actions[i]
			break
		}
	}
	if agentsAct == nil {
		t.Fatal("AGENTS.md action missing")
	}
	if agentsAct.Type != ActionUpdate {
		t.Errorf("AGENTS.md type = %v, want ActionUpdate", agentsAct.Type)
	}
	if !strings.Contains(agentsAct.Content, "User preamble.") {
		t.Errorf("user content lost:\n%s", agentsAct.Content)
	}
	if !strings.Contains(agentsAct.Content, MarkerBeginLine) {
		t.Errorf("marker missing:\n%s", agentsAct.Content)
	}
}

func TestCodexCLI_Plan_AgentsMd_SkipWhenIdentical(t *testing.T) {
	// AGENTS.md already contains an up-to-date marker block — the action
	// must be ActionSkip (idempotent re-run on a clean tree).
	t.Parallel()
	root := t.TempDir()
	loaded := []skills.Skill{
		{Name: "task-add", Raw: "raw\n", Description: "追加"},
	}
	body := RenderAgentsSnippet(loaded, "ja")
	merged := MergeMarkerBlock("", body)
	mustWriteFile(t, filepath.Join(root, "AGENTS.md"), merged)

	actions, err := CodexCLIAdapter{}.Plan(PlanContext{
		TargetRoot: root, Skills: loaded,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, a := range actions {
		if a.RelPath == "AGENTS.md" {
			if a.Type != ActionSkip {
				t.Errorf("AGENTS.md type = %v, want ActionSkip", a.Type)
			}
			return
		}
	}
	t.Fatal("AGENTS.md action missing")
}

func TestCodexCLI_Plan_SkillFile_ConflictOnUntracked(t *testing.T) {
	// Pre-existing SKILL.md NOT in the previous manifest must produce
	// ActionConflict (parity with claude-code).
	t.Parallel()
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".agents", "skills", "task-add")
	if err := os.MkdirAll(skillsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(skillsDir, "SKILL.md"), "third-party\n")

	loaded := []skills.Skill{{Name: "task-add", Raw: "raw\n", Description: "追加"}}
	actions, err := CodexCLIAdapter{}.Plan(PlanContext{TargetRoot: root, Skills: loaded})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, a := range actions {
		if a.RelPath == ".agents/skills/task-add/SKILL.md" {
			if a.Type != ActionConflict {
				t.Errorf("SKILL.md type = %v, want ActionConflict", a.Type)
			}
			if a.Shared {
				t.Errorf("SKILL.md must not be Shared")
			}
			return
		}
	}
	t.Fatal("expected SKILL.md action missing")
}

func TestCodexCLI_Plan_EmptyTargetErrors(t *testing.T) {
	t.Parallel()
	_, err := CodexCLIAdapter{}.Plan(PlanContext{Skills: nil})
	if err == nil {
		t.Errorf("expected error for empty TargetRoot")
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
