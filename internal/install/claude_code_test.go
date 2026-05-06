package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func TestClaudeCode_ManifestPath(t *testing.T) {
	t.Parallel()
	root := "/tmp/repo"
	got := ClaudeCodeAdapter{}.ManifestPath(root)
	want := filepath.Join(root, ".claude", "skills", ".gh-tasks-manifest.json")
	if got != want {
		t.Errorf("ManifestPath = %q, want %q", got, want)
	}
}

func TestClaudeCode_Plan_AllCreate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	loaded := []skills.Skill{
		{Name: "task-add", Raw: "raw-task-add\n"},
		{Name: "task-plan", Raw: "raw-task-plan\n"},
	}
	actions, err := ClaudeCodeAdapter{}.Plan(PlanContext{
		TargetRoot: root, Skills: loaded,
	})
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("got %d actions, want 2: %+v", len(actions), actions)
	}
	for _, a := range actions {
		if a.Type != ActionCreate {
			t.Errorf("expected ActionCreate, got %v for %s", a.Type, a.RelPath)
		}
	}
}

func TestClaudeCode_Plan_SkipUpdateConflict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	skillsRoot := filepath.Join(root, ".claude", "skills")
	if err := os.MkdirAll(filepath.Join(skillsRoot, "task-skip"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(skillsRoot, "task-update"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(skillsRoot, "task-conflict"), 0o750); err != nil {
		t.Fatal(err)
	}
	// task-skip: tracked + matching content -> Skip
	mustWrite(t, filepath.Join(skillsRoot, "task-skip", "SKILL.md"), "skip-content")
	// task-update: tracked + content drift -> Update
	mustWrite(t, filepath.Join(skillsRoot, "task-update", "SKILL.md"), "old-content")
	// task-conflict: untracked -> Conflict
	mustWrite(t, filepath.Join(skillsRoot, "task-conflict", "SKILL.md"), "third-party")

	loaded := []skills.Skill{
		{Name: "task-skip", Raw: "skip-content"},
		{Name: "task-update", Raw: "new-content"},
		{Name: "task-conflict", Raw: "fresh-content"},
		{Name: "task-new", Raw: "fresh-new"},
	}
	prev := Manifest{
		Agent: AgentClaudeCode,
		Files: []string{
			".claude/skills/task-skip/SKILL.md",
			".claude/skills/task-update/SKILL.md",
		},
	}
	actions, err := ClaudeCodeAdapter{}.Plan(PlanContext{
		TargetRoot: root, Skills: loaded, Existing: prev,
	})
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(actions) != 4 {
		t.Fatalf("got %d actions, want 4: %+v", len(actions), actions)
	}
	byName := map[string]Action{}
	for _, a := range actions {
		byName[a.RelPath] = a
	}
	want := map[string]ActionType{
		".claude/skills/task-skip/SKILL.md":     ActionSkip,
		".claude/skills/task-update/SKILL.md":   ActionUpdate,
		".claude/skills/task-conflict/SKILL.md": ActionConflict,
		".claude/skills/task-new/SKILL.md":      ActionCreate,
	}
	for rel, wantType := range want {
		got, ok := byName[rel]
		if !ok {
			t.Errorf("missing action for %s", rel)
			continue
		}
		if got.Type != wantType {
			t.Errorf("%s: got %v, want %v", rel, got.Type, wantType)
		}
	}
}

func TestClaudeCode_Plan_EmptyTarget(t *testing.T) {
	t.Parallel()
	_, err := ClaudeCodeAdapter{}.Plan(PlanContext{Skills: nil})
	if err == nil {
		t.Errorf("Plan with empty TargetRoot should error")
	}
}

func TestClaudeCode_Plan_Force_DowngradesConflictToUpdateWithBackup(t *testing.T) {
	// PR 6: when --force is set, an untracked existing file becomes an
	// ActionUpdate carrying BackupTo = <abs>.bak. The cmd layer never
	// reaches the conflict guard in that case.
	t.Parallel()
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills", "task-add")
	if err := os.MkdirAll(skillsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(skillsDir, "SKILL.md"), "third-party body\n")

	loaded := []skills.Skill{{Name: "task-add", Raw: "fresh body\n"}}
	actions, err := ClaudeCodeAdapter{}.Plan(PlanContext{
		TargetRoot: root, Skills: loaded, Force: true,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("got %d actions, want 1: %+v", len(actions), actions)
	}
	got := actions[0]
	if got.Type != ActionUpdate {
		t.Errorf("Type = %v, want ActionUpdate (force should downgrade conflict)", got.Type)
	}
	wantBak := filepath.Join(skillsDir, "SKILL.md") + ".bak"
	if got.BackupTo != wantBak {
		t.Errorf("BackupTo = %q, want %q", got.BackupTo, wantBak)
	}
}

func TestClaudeCode_Plan_Force_DoesNotAffectTrackedPaths(t *testing.T) {
	// `--force` is a conflict resolver, not a backup-everything flag.
	// Tracked files (already in the previous manifest) should still go
	// through the normal Update path with BackupTo unset — there is no
	// third-party content to preserve.
	t.Parallel()
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".claude", "skills", "task-add")
	if err := os.MkdirAll(skillsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(skillsDir, "SKILL.md"), "old\n")

	loaded := []skills.Skill{{Name: "task-add", Raw: "new\n"}}
	prev := Manifest{
		Agent: AgentClaudeCode,
		Files: []string{".claude/skills/task-add/SKILL.md"},
	}
	actions, err := ClaudeCodeAdapter{}.Plan(PlanContext{
		TargetRoot: root, Skills: loaded, Existing: prev, Force: true,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("got %d actions, want 1: %+v", len(actions), actions)
	}
	if actions[0].Type != ActionUpdate {
		t.Errorf("Type = %v, want ActionUpdate", actions[0].Type)
	}
	if actions[0].BackupTo != "" {
		t.Errorf("BackupTo = %q, want empty for tracked update", actions[0].BackupTo)
	}
}

func TestExecute_RefusesOnConflict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	abs := filepath.Join(root, "x.md")
	actions := []Action{
		{Type: ActionConflict, Path: abs, RelPath: "x.md"},
	}
	if _, err := Execute(actions); err == nil {
		t.Errorf("Execute with conflict returned nil err, want guard error")
	}
	if _, err := os.Stat(abs); err == nil {
		t.Errorf("Execute wrote conflict path %s anyway", abs)
	}
}

func TestExecute_SplitsOwnedAndShared(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	owned := filepath.Join(root, "owned.md")
	shared := filepath.Join(root, "shared.md")
	actions := []Action{
		{Type: ActionCreate, Path: owned, RelPath: "owned.md", Content: "o\n"},
		{Type: ActionCreate, Path: shared, RelPath: "shared.md", Content: "s\n", Shared: true},
	}
	res, err := Execute(actions)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Files) != 1 || res.Files[0] != "owned.md" {
		t.Errorf("Files = %v, want [owned.md]", res.Files)
	}
	if len(res.Shared) != 1 || res.Shared[0] != "shared.md" {
		t.Errorf("Shared = %v, want [shared.md]", res.Shared)
	}
}

func TestExecute_WritesCreate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	abs := filepath.Join(root, "sub", "y.md")
	actions := []Action{
		{Type: ActionCreate, Path: abs, RelPath: "sub/y.md", Content: "hi\n"},
	}
	res, err := Execute(actions)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if len(res.Files) != 1 || res.Files[0] != "sub/y.md" {
		t.Errorf("res.Files = %v, want [sub/y.md]", res.Files)
	}
	if len(res.Shared) != 0 {
		t.Errorf("res.Shared = %v, want empty for owned action", res.Shared)
	}
	body, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read created: %v", err)
	}
	if string(body) != "hi\n" {
		t.Errorf("content = %q, want %q", string(body), "hi\n")
	}
}

func TestTally_Drift(t *testing.T) {
	t.Parallel()
	if (Counts{}).HasDrift() {
		t.Errorf("zero Counts should not be drift")
	}
	if (Counts{Skipped: 5}).HasDrift() {
		t.Errorf("only-skipped should not be drift")
	}
	if !(Counts{Created: 1}).HasDrift() {
		t.Errorf("Created>0 should be drift")
	}
	if !(Counts{Updated: 1}).HasDrift() {
		t.Errorf("Updated>0 should be drift")
	}
	if !(Counts{Conflicts: 1}).HasDrift() {
		t.Errorf("Conflicts>0 should be drift")
	}
}
