package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: build a Manifest with the given Files / Shared lists.
func mkManifest(agent Agent, files, shared []string) Manifest {
	return Manifest{
		SchemaVersion: ManifestSchemaVersion,
		Agent:         agent,
		Files:         files,
		Shared:        shared,
	}
}

func TestRemoveMarkerBlock_Cases(t *testing.T) {
	t.Parallel()
	body := "## gh-tasks Skills\n\n- foo"
	merged := MergeMarkerBlock("USER\n", body)
	stripped := RemoveMarkerBlock(merged)
	if stripped != "USER\n" {
		t.Errorf("undo of merge differs from original; got %q", stripped)
	}

	// With trailing footer.
	mergedWithFooter := MergeMarkerBlock("HEAD\n\nMID\n", body) + "\n\nFOOTER\n"
	got := RemoveMarkerBlock(mergedWithFooter)
	if !strings.Contains(got, "HEAD") || !strings.Contains(got, "MID") || !strings.Contains(got, "FOOTER") {
		t.Errorf("user content survived rejoin? got %q", got)
	}
	if strings.Contains(got, "begin: @ozzylabs/gh-tasks") {
		t.Errorf("marker block not stripped: %q", got)
	}

	// Idempotent on input without marker.
	plain := "no markers here\n"
	if RemoveMarkerBlock(plain) != plain {
		t.Errorf("expected no-op when no marker block")
	}

	// Marker-only input shrinks to empty.
	only := MergeMarkerBlock("", body)
	if got := RemoveMarkerBlock(only); got != "" {
		t.Errorf("marker-only input should strip to empty, got %q", got)
	}
}

func TestRemoveGeminiSettingsEntry_KeepsOtherKeys(t *testing.T) {
	t.Parallel()
	in := []byte(`{"model":"gemini-2.5-pro","temperature":0.2,"context":{"fileName":["AGENTS.md","other.md"]}}`)
	out, err := RemoveGeminiSettingsEntry(in)
	if err != nil {
		t.Fatalf("RemoveGeminiSettingsEntry: %v", err)
	}
	if !strings.Contains(string(out), "gemini-2.5-pro") {
		t.Errorf("model lost:\n%s", string(out))
	}
	if !strings.Contains(string(out), "0.2") {
		t.Errorf("temperature lost:\n%s", string(out))
	}
	if !strings.Contains(string(out), "other.md") {
		t.Errorf("other fileName entry lost:\n%s", string(out))
	}
	if strings.Contains(string(out), "AGENTS.md") {
		t.Errorf("AGENTS.md still present:\n%s", string(out))
	}
}

func TestRemoveGeminiSettingsEntry_DropsContextWhenEmpty(t *testing.T) {
	t.Parallel()
	in := []byte(`{"context":{"fileName":["AGENTS.md"]}}`)
	out, err := RemoveGeminiSettingsEntry(in)
	if err != nil {
		t.Fatalf("RemoveGeminiSettingsEntry: %v", err)
	}
	if strings.Contains(string(out), "context") {
		t.Errorf("expected context dropped after fileName became empty:\n%s", string(out))
	}
	if strings.Contains(string(out), "fileName") {
		t.Errorf("expected fileName dropped:\n%s", string(out))
	}
}

func TestRemoveGeminiSettingsEntry_NoOpForEmpty(t *testing.T) {
	t.Parallel()
	out, err := RemoveGeminiSettingsEntry([]byte(""))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) != "" {
		t.Errorf("empty input not preserved: %q", string(out))
	}
}

func TestRemoveGeminiSettingsEntry_HandlesSingleStringForm(t *testing.T) {
	t.Parallel()
	in := []byte(`{"context":{"fileName":"AGENTS.md","other":1}}`)
	out, err := RemoveGeminiSettingsEntry(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.Contains(string(out), "AGENTS.md") {
		t.Errorf("AGENTS.md not removed:\n%s", string(out))
	}
	if !strings.Contains(string(out), `"other": 1`) {
		t.Errorf("other key lost:\n%s", string(out))
	}
}

func TestClaudeCode_PlanUninstall_OwnedFilesPlusManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	prev := mkManifest(AgentClaudeCode,
		[]string{".claude/skills/task-add/SKILL.md", ".claude/skills/task-plan/SKILL.md"},
		nil,
	)
	actions, err := ClaudeCodeAdapter{}.PlanUninstall(UninstallContext{
		TargetRoot: root,
		Existing:   prev,
	})
	if err != nil {
		t.Fatalf("PlanUninstall: %v", err)
	}
	want := []string{
		".claude/skills/task-add/SKILL.md",
		".claude/skills/task-plan/SKILL.md",
		".claude/skills/.gh-tasks-manifest.json",
	}
	if len(actions) != len(want) {
		t.Fatalf("got %d actions, want %d: %+v", len(actions), len(want), actions)
	}
	for i, a := range actions {
		if a.Type != ActionRemove {
			t.Errorf("actions[%d].Type = %v, want ActionRemove", i, a.Type)
		}
		if a.RelPath != want[i] {
			t.Errorf("actions[%d].RelPath = %q, want %q", i, a.RelPath, want[i])
		}
	}
}

func TestCodexCLI_PlanUninstall_KeepsAgentsMdWhenGeminiStillReferences(t *testing.T) {
	// codex-cli uninstalled, gemini-cli still installed: AGENTS.md
	// marker block must remain because gemini still uses it.
	t.Parallel()
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "AGENTS.md"),
		MergeMarkerBlock("USER\n", "## gh-tasks Skills\n\n- task-add"))

	prev := mkManifest(AgentCodexCLI,
		[]string{".agents/skills/task-add/SKILL.md"},
		[]string{"AGENTS.md"},
	)
	others := map[Agent]Manifest{
		AgentGeminiCLI: mkManifest(AgentGeminiCLI, nil, []string{".gemini/settings.json", "AGENTS.md"}),
	}
	actions, err := CodexCLIAdapter{}.PlanUninstall(UninstallContext{
		TargetRoot: root,
		Existing:   prev,
		Others:     others,
	})
	if err != nil {
		t.Fatalf("PlanUninstall: %v", err)
	}
	for _, a := range actions {
		if a.RelPath == "AGENTS.md" {
			t.Errorf("AGENTS.md should not be touched while gemini still references it: %+v", a)
		}
	}
}

func TestCodexCLI_PlanUninstall_StripsAgentsMdWhenLastReference(t *testing.T) {
	// codex-cli uninstalled, no other adapter references AGENTS.md:
	// marker block is stripped (or AGENTS.md removed if it has no
	// other content).
	t.Parallel()
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "AGENTS.md"),
		MergeMarkerBlock("USER\n", "## gh-tasks Skills\n\n- task-add"))

	prev := mkManifest(AgentCodexCLI,
		[]string{".agents/skills/task-add/SKILL.md"},
		[]string{"AGENTS.md"},
	)
	actions, err := CodexCLIAdapter{}.PlanUninstall(UninstallContext{
		TargetRoot: root,
		Existing:   prev,
		Others:     nil,
	})
	if err != nil {
		t.Fatalf("PlanUninstall: %v", err)
	}
	var found *Action
	for i := range actions {
		if actions[i].RelPath == "AGENTS.md" {
			found = &actions[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected an AGENTS.md action when no other adapter references it")
	}
	if found.Type != ActionUpdate {
		t.Errorf("AGENTS.md action = %v, want ActionUpdate (USER\\n still present)", found.Type)
	}
	if !strings.Contains(found.Content, "USER") {
		t.Errorf("user content lost:\n%s", found.Content)
	}
	if strings.Contains(found.Content, MarkerBeginLine) {
		t.Errorf("marker still present:\n%s", found.Content)
	}
}

func TestGeminiCLI_PlanUninstall_StripsSettingsEntryAndManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	geminiDir := filepath.Join(root, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o750); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(geminiDir, "settings.json"),
		`{"model":"gemini-2.5-pro","context":{"fileName":["AGENTS.md"]}}`)

	prev := mkManifest(AgentGeminiCLI, nil,
		[]string{".gemini/settings.json", "AGENTS.md"})
	// codex-cli still installed → AGENTS.md ref-count keeps marker.
	others := map[Agent]Manifest{
		AgentCodexCLI: mkManifest(AgentCodexCLI, nil, []string{"AGENTS.md"}),
	}
	actions, err := GeminiCLIAdapter{}.PlanUninstall(UninstallContext{
		TargetRoot: root,
		Existing:   prev,
		Others:     others,
	})
	if err != nil {
		t.Fatalf("PlanUninstall: %v", err)
	}
	// Expect: settings.json Update + manifest Remove. AGENTS.md left alone.
	if len(actions) != 2 {
		t.Fatalf("got %d actions, want 2: %+v", len(actions), actions)
	}
	var settingsAct, manifestAct *Action
	for i := range actions {
		switch actions[i].RelPath {
		case ".gemini/settings.json":
			settingsAct = &actions[i]
		case ".gemini/.gh-tasks-manifest.json":
			manifestAct = &actions[i]
		}
	}
	if settingsAct == nil {
		t.Fatal("settings.json action missing")
	}
	if settingsAct.Type != ActionUpdate {
		t.Errorf("settings.json action = %v, want ActionUpdate", settingsAct.Type)
	}
	if !strings.Contains(settingsAct.Content, "gemini-2.5-pro") {
		t.Errorf("user model lost from new settings:\n%s", settingsAct.Content)
	}
	if strings.Contains(settingsAct.Content, "AGENTS.md") {
		t.Errorf("AGENTS.md not removed from settings:\n%s", settingsAct.Content)
	}
	if manifestAct == nil || manifestAct.Type != ActionRemove {
		t.Errorf("manifest action missing or wrong: %+v", manifestAct)
	}
}

func TestCopilot_PlanUninstall_StripsMarkerAndRemovesManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	githubDir := filepath.Join(root, ".github")
	if err := os.MkdirAll(githubDir, 0o750); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(githubDir, "copilot-instructions.md"),
		MergeMarkerBlock("# Copilot project context\n", "## gh-tasks Skills\n\n- foo"))

	prev := mkManifest(AgentCopilot, nil, []string{".github/copilot-instructions.md"})
	actions, err := CopilotAdapter{}.PlanUninstall(UninstallContext{
		TargetRoot: root,
		Existing:   prev,
	})
	if err != nil {
		t.Fatalf("PlanUninstall: %v", err)
	}
	var instr, manifest *Action
	for i := range actions {
		switch actions[i].RelPath {
		case ".github/copilot-instructions.md":
			instr = &actions[i]
		case ".github/.gh-tasks-copilot-manifest.json":
			manifest = &actions[i]
		}
	}
	if instr == nil || instr.Type != ActionUpdate {
		t.Fatalf("expected ActionUpdate for copilot-instructions.md, got %+v", instr)
	}
	if !strings.Contains(instr.Content, "Copilot project context") {
		t.Errorf("user content lost:\n%s", instr.Content)
	}
	if strings.Contains(instr.Content, MarkerBeginLine) {
		t.Errorf("marker still present after uninstall plan:\n%s", instr.Content)
	}
	if manifest == nil || manifest.Type != ActionRemove {
		t.Errorf("manifest action missing/wrong: %+v", manifest)
	}
}

func TestExecute_ActionRemove_DeletesAndCleansEmptyDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".claude", "skills", "task-add")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "SKILL.md")
	mustWriteFile(t, target, "x")

	res, err := Execute([]Action{{
		Type: ActionRemove, Path: target, RelPath: ".claude/skills/task-add/SKILL.md",
	}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Removed) != 1 {
		t.Errorf("Removed = %v, want 1 entry", res.Removed)
	}
	if _, err := os.Stat(target); err == nil {
		t.Errorf("target still exists")
	}
	// Empty parent dir should also be cleaned.
	if _, err := os.Stat(dir); err == nil {
		t.Errorf("parent dir not cleaned: %s", dir)
	}
}

func TestExecute_ActionRemove_MissingFileNoError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "missing.md")
	res, err := Execute([]Action{{
		Type: ActionRemove, Path: target, RelPath: "missing.md",
	}})
	if err != nil {
		t.Fatalf("Execute on missing target: %v", err)
	}
	if len(res.Removed) != 1 {
		t.Errorf("Removed = %v, want 1 entry (idempotent)", res.Removed)
	}
}

func TestTally_RemovedDrift(t *testing.T) {
	t.Parallel()
	if !(Counts{Removed: 1}).HasDrift() {
		t.Errorf("Removed>0 should be drift (uninstall changes the workspace)")
	}
}
