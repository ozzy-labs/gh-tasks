package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadManifest_NotExist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m, err := ReadManifest(filepath.Join(dir, "absent.json"))
	if err != nil {
		t.Fatalf("ReadManifest(absent) error: %v", err)
	}
	if !m.IsZero() {
		t.Errorf("ReadManifest(absent).IsZero() = false, want true: %+v", m)
	}
}

func TestReadManifest_Valid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	body := `{
  "schema_version": 1,
  "gh_tasks_version": "0.1.1",
  "agent": "claude-code",
  "files": [".claude/skills/task-add/SKILL.md"]
}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest error: %v", err)
	}
	if m.Agent != AgentClaudeCode {
		t.Errorf("Agent = %q, want %q", m.Agent, AgentClaudeCode)
	}
	if !m.Has(".claude/skills/task-add/SKILL.md") {
		t.Errorf("Has(...) = false, want true")
	}
	if m.Has(".claude/skills/task-zzz/SKILL.md") {
		t.Errorf("Has(unknown) = true, want false")
	}
}

func TestReadManifest_Malformed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(path); err == nil {
		t.Errorf("ReadManifest(malformed) returned nil error")
	} else if !strings.Contains(err.Error(), "parse manifest") {
		t.Errorf("error message missing parse marker: %v", err)
	}
}

func TestWriteManifest_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "manifest.json") // exercises mkdir
	want := Manifest{
		Agent: AgentClaudeCode,
		Files: []string{
			".claude/skills/task-plan/SKILL.md",
			".claude/skills/task-add/SKILL.md", // out of order on purpose
		},
		GHTasksVer: "0.1.1",
	}
	if err := WriteManifest(path, want); err != nil {
		t.Fatalf("WriteManifest error: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written manifest: %v", err)
	}
	if !strings.Contains(string(body), `"schema_version": 1`) {
		t.Errorf("manifest missing schema_version stamp: %s", string(body))
	}
	got, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest error: %v", err)
	}
	// Files must be sorted on disk.
	if got.Files[0] != ".claude/skills/task-add/SKILL.md" {
		t.Errorf("Files not sorted: %v", got.Files)
	}
	if got.Agent != want.Agent {
		t.Errorf("Agent = %q, want %q", got.Agent, want.Agent)
	}
}
