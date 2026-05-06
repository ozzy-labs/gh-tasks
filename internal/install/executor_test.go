package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecute_BackupTo_PreservesOriginalAtBak(t *testing.T) {
	// PR 6: an Action with BackupTo set must rename the existing file
	// to BackupTo before writing the new Content. The user can then
	// recover the third-party file from <path>.bak.
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, target, "third-party body\n")

	bak := target + ".bak"
	actions := []Action{{
		Type:     ActionUpdate,
		Path:     target,
		RelPath:  "skill/SKILL.md",
		Content:  "new gh-tasks body\n",
		BackupTo: bak,
	}}
	res, err := Execute(actions)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Files) != 1 || res.Files[0] != "skill/SKILL.md" {
		t.Errorf("Files = %v, want [skill/SKILL.md]", res.Files)
	}

	// Target now holds new content.
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new gh-tasks body\n" {
		t.Errorf("target content = %q, want new gh-tasks body", string(got))
	}
	// .bak holds the original.
	bakBody, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	if string(bakBody) != "third-party body\n" {
		t.Errorf("backup content = %q, want third-party body", string(bakBody))
	}
}

func TestExecute_BackupTo_OverwritesPriorBackup(t *testing.T) {
	// Repeated `--force` runs should not pile up multiple .bak files.
	// We accept overwriting the prior backup so the workspace stays
	// clean (matches `cp -f` semantics).
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "SKILL.md")
	bak := target + ".bak"
	mustWriteFile(t, target, "current third-party\n")
	mustWriteFile(t, bak, "stale prior backup\n")

	_, err := Execute([]Action{{
		Type: ActionUpdate, Path: target, RelPath: "SKILL.md",
		Content: "fresh\n", BackupTo: bak,
	}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got, err := os.ReadFile(bak)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "current third-party\n" {
		t.Errorf("expected backup overwritten with current third-party, got %q", string(got))
	}
}

func TestExecute_BackupTo_NoFileNoError(t *testing.T) {
	// If the path BackupTo points to has already vanished (race or
	// concurrent rm), Execute should still succeed: the new content
	// gets written, and there is simply nothing to back up.
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "SKILL.md")
	bak := target + ".bak"
	// Note: no pre-existing file at target.

	_, err := Execute([]Action{{
		Type: ActionCreate, Path: target, RelPath: "SKILL.md",
		Content: "fresh\n", BackupTo: bak,
	}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("target missing after Execute: %v", err)
	}
	if _, err := os.Stat(bak); err == nil {
		t.Errorf("expected no .bak when source absent, got one anyway")
	}
}
