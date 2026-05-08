package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func TestGeminiCLI_ManifestPath(t *testing.T) {
	t.Parallel()
	root := "/tmp/repo"
	got := GeminiCLIAdapter{}.ManifestPath(root)
	want := filepath.Join(root, ".gemini", ".gh-tasks-manifest.json")
	if got != want {
		t.Errorf("ManifestPath = %q, want %q", got, want)
	}
}

func TestMergeGeminiSettings_Empty(t *testing.T) {
	t.Parallel()
	got, err := MergeGeminiSettings(nil)
	if err != nil {
		t.Fatal(err)
	}
	files := unmarshalContextFileName(t, got)
	if len(files) != 1 || files[0] != "AGENTS.md" {
		t.Errorf("got %v, want [AGENTS.md]", files)
	}
}

func TestMergeGeminiSettings_PreservesUnrelatedKeys(t *testing.T) {
	t.Parallel()
	in := `{"model":"gemini-2.5-pro","temperature":0.5}`
	got, err := MergeGeminiSettings([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["model"] != "gemini-2.5-pro" {
		t.Errorf("model = %v, want preserved", parsed["model"])
	}
	if parsed["temperature"] != 0.5 {
		t.Errorf("temperature = %v, want preserved", parsed["temperature"])
	}
	files := unmarshalContextFileName(t, got)
	if len(files) != 1 || files[0] != "AGENTS.md" {
		t.Errorf("fileName = %v, want [AGENTS.md]", files)
	}
}

func TestMergeGeminiSettings_AppendsToExistingFileNames(t *testing.T) {
	t.Parallel()
	in := `{"context":{"fileName":["custom.md"]}}`
	got, err := MergeGeminiSettings([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	files := unmarshalContextFileName(t, got)
	if len(files) != 2 {
		t.Fatalf("fileName = %v, want 2 entries", files)
	}
	if files[0] != "custom.md" || files[1] != "AGENTS.md" {
		t.Errorf("fileName = %v, want [custom.md, AGENTS.md]", files)
	}
}

func TestMergeGeminiSettings_IdempotentWhenAlreadyPresent(t *testing.T) {
	t.Parallel()
	in := `{"context":{"fileName":["AGENTS.md"]}}`
	got, err := MergeGeminiSettings([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	files := unmarshalContextFileName(t, got)
	if len(files) != 1 || files[0] != "AGENTS.md" {
		t.Errorf("fileName = %v, want [AGENTS.md] (no duplicate)", files)
	}
	// And calling again with our own output should yield byte-identical.
	got2, err := MergeGeminiSettings(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(got2) {
		t.Errorf("not idempotent at byte level\nfirst:\n%s\nsecond:\n%s", string(got), string(got2))
	}
}

func TestMergeGeminiSettings_PreservesContextSiblings(t *testing.T) {
	t.Parallel()
	in := `{"context":{"otherKey":"X","fileName":["A.md"]}}`
	got, err := MergeGeminiSettings([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatal(err)
	}
	ctx := parsed["context"].(map[string]any)
	if ctx["otherKey"] != "X" {
		t.Errorf("context.otherKey lost: %v", ctx["otherKey"])
	}
	files := ctx["fileName"].([]any)
	if len(files) != 2 {
		t.Errorf("fileName = %v, want 2 entries", files)
	}
}

func TestMergeGeminiSettings_PromotesStringFileName(t *testing.T) {
	// Legacy form: fileName as a single string instead of an array.
	t.Parallel()
	in := `{"context":{"fileName":"single.md"}}`
	got, err := MergeGeminiSettings([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	files := unmarshalContextFileName(t, got)
	if len(files) != 2 || files[0] != "single.md" || files[1] != "AGENTS.md" {
		t.Errorf("fileName = %v, want [single.md, AGENTS.md]", files)
	}
}

func TestMergeGeminiSettings_RejectsWeirdContext(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"context-not-object": `{"context":"x"}`,
		"fileName-bool":      `{"context":{"fileName":true}}`,
		"malformed-json":     `{not json`,
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := MergeGeminiSettings([]byte(in)); err == nil {
				t.Errorf("expected error for %q", in)
			}
		})
	}
}

func TestGeminiCLI_Plan_FreshTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	loaded := []skills.Skill{
		{Name: "task-add", Raw: "raw\n", Description: "追加"},
	}
	actions, err := GeminiCLIAdapter{}.Plan(PlanContext{TargetRoot: root, Skills: loaded})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// settings.json + AGENTS.md = 2 actions, both Shared, both Create.
	if len(actions) != 2 {
		t.Fatalf("got %d actions, want 2:\n%+v", len(actions), actions)
	}
	for _, a := range actions {
		if !a.Shared {
			t.Errorf("expected Shared=true for %s", a.RelPath)
		}
		if a.Type != ActionCreate {
			t.Errorf("expected ActionCreate for %s, got %v", a.RelPath, a.Type)
		}
		if a.RelPath == "AGENTS.md" && !strings.HasPrefix(a.Content, AgentsMdScaffold) {
			t.Errorf("AGENTS.md must begin with scaffold on fresh create:\n%s", a.Content)
		}
	}
}

func TestGeminiCLI_Plan_AgentsMd_IdempotentAfterFreshInstall(t *testing.T) {
	// After a fresh install the on-disk AGENTS.md is scaffold + marker
	// block. Re-running Plan must produce ActionSkip.
	t.Parallel()
	root := t.TempDir()
	geminiDir := filepath.Join(root, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o750); err != nil {
		t.Fatal(err)
	}
	merged, err := MergeGeminiSettings(nil)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(geminiDir, "settings.json"), string(merged))

	loaded := []skills.Skill{{Name: "task-add", Raw: "raw\n", Description: "追加"}}
	body := RenderAgentsSnippet(loaded, "ja")
	mustWriteFile(t, filepath.Join(root, "AGENTS.md"),
		MergeMarkerBlock(AgentsMdScaffold, body))

	actions, err := GeminiCLIAdapter{}.Plan(PlanContext{TargetRoot: root, Skills: loaded})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, a := range actions {
		if a.RelPath == "AGENTS.md" && a.Type != ActionSkip {
			t.Errorf("AGENTS.md type = %v, want ActionSkip", a.Type)
		}
	}
}

func TestGeminiCLI_Plan_PreservesUnrelatedSettingsKeys(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	geminiDir := filepath.Join(root, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o750); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(geminiDir, "settings.json")
	mustWriteFile(t, settingsPath, `{"model":"gemini-2.5-pro"}`)

	loaded := []skills.Skill{{Name: "task-add", Raw: "raw\n", Description: "追加"}}
	actions, err := GeminiCLIAdapter{}.Plan(PlanContext{TargetRoot: root, Skills: loaded})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	var settingsAct *Action
	for i := range actions {
		if actions[i].RelPath == ".gemini/settings.json" {
			settingsAct = &actions[i]
			break
		}
	}
	if settingsAct == nil {
		t.Fatal("settings.json action missing")
	}
	if settingsAct.Type != ActionUpdate {
		t.Errorf("settings.json Type = %v, want ActionUpdate", settingsAct.Type)
	}
	if !strings.Contains(settingsAct.Content, "gemini-2.5-pro") {
		t.Errorf("model preserved? content:\n%s", settingsAct.Content)
	}
	if !strings.Contains(settingsAct.Content, "AGENTS.md") {
		t.Errorf("AGENTS.md added? content:\n%s", settingsAct.Content)
	}
}

func TestGeminiCLI_Plan_SkipWhenAlreadyMerged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	geminiDir := filepath.Join(root, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o750); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(geminiDir, "settings.json")
	merged, err := MergeGeminiSettings(nil)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, settingsPath, string(merged))

	loaded := []skills.Skill{{Name: "task-add", Raw: "raw\n", Description: "追加"}}
	body := RenderAgentsSnippet(loaded, "ja")
	mustWriteFile(t, filepath.Join(root, "AGENTS.md"), MergeMarkerBlock("", body))

	actions, err := GeminiCLIAdapter{}.Plan(PlanContext{TargetRoot: root, Skills: loaded})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, a := range actions {
		if a.Type != ActionSkip {
			t.Errorf("%s: expected ActionSkip, got %v", a.RelPath, a.Type)
		}
	}
}

func TestGeminiCLI_Plan_EmptyTargetErrors(t *testing.T) {
	t.Parallel()
	_, err := GeminiCLIAdapter{}.Plan(PlanContext{Skills: nil})
	if err == nil {
		t.Errorf("expected error for empty TargetRoot")
	}
}

// unmarshalContextFileName decodes settings JSON and returns
// context.fileName as a string slice for assertion convenience.
func unmarshalContextFileName(t *testing.T, body []byte) []string {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ctx, ok := parsed["context"].(map[string]any)
	if !ok {
		t.Fatalf("no context object in: %s", string(body))
	}
	raw, ok := ctx["fileName"].([]any)
	if !ok {
		t.Fatalf("fileName missing or wrong type in: %s", string(body))
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("fileName entry %v is not a string", v)
		}
		out = append(out, s)
	}
	return out
}
