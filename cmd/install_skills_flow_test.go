// Cobra-rooted flow tests for `gh tasks install-skills`. The Deps
// EmbeddedSkills is supplied as a fstest.MapFS so the tests do not depend
// on the work tree's skills/ directory; the consumer "target" is a
// t.TempDir() so on-disk side effects stay scoped to the test.
package cmd_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/install"
)

// embeddedFixture returns a MapFS with two minimal skills under
// `skills/<name>/SKILL.md` (+ SKILL.en.md mirror) so skills.LoadFS
// accepts the bundle without reaching into the real source tree.
func embeddedFixture() fstest.MapFS {
	const ja = `---
name: %s
description: %s ja
description_en: %s en
allowed-tools: Bash
locale: ja
---

%s body
`
	const en = `---
name: %s
description: %s ja
description_en: %s en
allowed-tools: Bash
locale: en
---

%s body en
`
	mk := func(name string) (string, string) {
		return fmt.Sprintf(ja, name, name, name, name),
			fmt.Sprintf(en, name, name, name, name)
	}
	a1, a2 := mk("alpha")
	b1, b2 := mk("bravo")
	return fstest.MapFS{
		"skills/alpha/SKILL.md":    &fstest.MapFile{Data: []byte(a1)},
		"skills/alpha/SKILL.en.md": &fstest.MapFile{Data: []byte(a2)},
		"skills/bravo/SKILL.md":    &fstest.MapFile{Data: []byte(b1)},
		"skills/bravo/SKILL.en.md": &fstest.MapFile{Data: []byte(b2)},
	}
}

func TestInstallSkills_HappyPath_ClaudeCode(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	embed := embeddedFixture()
	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embed })

	stdout, stderr, err := runCmd(t, d, "install-skills", "--target", target)
	if err != nil {
		t.Fatalf("Execute: %v (stderr=%s)", err, stderr.String())
	}

	for _, name := range []string{"alpha", "bravo"} {
		path := filepath.Join(target, ".claude", "skills", name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}
	if !strings.Contains(stdout.String(), "Auto-detected agents") &&
		!strings.Contains(stdout.String(), "自動検出") {
		t.Errorf("missing auto-detect heading:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "claude-code") {
		t.Errorf("missing claude-code adapter heading:\n%s", stdout.String())
	}

	manifestPath := filepath.Join(target, ".claude", "skills", ".gh-tasks-manifest.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest read: %v", err)
	}
	var got install.Manifest
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}
	if got.Agent != install.AgentClaudeCode {
		t.Errorf("manifest agent = %q, want claude-code", got.Agent)
	}
	if len(got.Files) != 2 {
		t.Errorf("manifest files = %d, want 2: %+v", len(got.Files), got.Files)
	}
}

func TestInstallSkills_DryRun(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })

	stdout, stderr, err := runCmd(t, d, "install-skills", "--target", target, "--dry-run")
	if err != nil {
		t.Fatalf("Execute: %v (stderr=%s)", err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(target, ".claude", "skills", "alpha", "SKILL.md")); err == nil {
		t.Errorf("--dry-run wrote files; expected no-op")
	}
	if !strings.Contains(stdout.String(), "alpha") {
		t.Errorf("dry-run did not list planned alpha action:\n%s", stdout.String())
	}
}

func TestInstallSkills_Check_Drift(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	_, stderr, err := runCmd(t, d, "install-skills", "--target", target, "--check")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent on drift, got %v", err)
	}
	if !strings.Contains(stderr.String(), "differ") &&
		!strings.Contains(stderr.String(), "差分") {
		t.Errorf("expected drift message, got: %s", stderr.String())
	}
}

func TestInstallSkills_Check_Clean(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	// Seed by running install once.
	if _, _, err := runCmd(t, d, "install-skills", "--target", target); err != nil {
		t.Fatalf("seed install: %v", err)
	}
	d2 := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	stdout, stderr, err := runCmd(t, d2, "install-skills", "--target", target, "--check")
	if err != nil {
		t.Fatalf("clean --check error: %v (stderr=%s)", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("clean --check missing OK marker:\n%s", stdout.String())
	}
}

func TestInstallSkills_NoAgentDetected(t *testing.T) {
	t.Parallel()
	target := t.TempDir() // empty — no claude-code traces
	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })

	_, stderr, err := runCmd(t, d, "install-skills", "--target", target)
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs, got %v", err)
	}
	if !strings.Contains(stderr.String(), "agent") {
		t.Errorf("error stderr missing 'agent' hint: %s", stderr.String())
	}
}

func TestInstallSkills_Conflict(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Pre-create a third-party SKILL.md that does NOT come from gh-tasks
	// (no manifest entry).
	skillsDir := filepath.Join(target, ".claude", "skills", "alpha")
	if err := os.MkdirAll(skillsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("third-party\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	_, stderr, err := runCmd(t, d, "install-skills", "--target", target)
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), "conflict") &&
		!strings.Contains(stderr.String(), "衝突") {
		t.Errorf("expected conflict message, got: %s", stderr.String())
	}
	// Third-party file must remain untouched.
	body, err := os.ReadFile(filepath.Join(skillsDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "third-party\n" {
		t.Errorf("conflict path was overwritten: %q", string(body))
	}
}

func TestInstallSkills_AgentExplicit_UnknownRejected(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	_, stderr, err := runCmd(t, d, "install-skills", "--target", target, "--agent", "bogus")
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs, got %v", err)
	}
	if !strings.Contains(stderr.String(), "bogus") {
		t.Errorf("error stderr missing offending agent name: %s", stderr.String())
	}
}

func TestInstallSkills_AgentExplicit_UnregisteredErrors(t *testing.T) {
	// `codex-cli` is recognized by ValidateAgent but not yet wired into
	// PR 2. The cmd should surface a localized unknown-agent error rather
	// than silently no-op.
	t.Parallel()
	target := t.TempDir()
	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	_, stderr, err := runCmd(t, d, "install-skills", "--target", target, "--agent", "codex-cli")
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs, got %v (stderr=%s)", err, stderr.String())
	}
}

func TestInstallSkills_EmbedMissing(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := testDeps(nil) // EmbeddedSkills left nil
	_, stderr, err := runCmd(t, d, "install-skills", "--target", target)
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "embed") {
		t.Errorf("expected embed-missing hint, got: %s", stderr.String())
	}
}

func TestInstallSkills_TargetNotADir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	file := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	_, stderr, err := runCmd(t, d, "install-skills", "--target", file)
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs, got %v", err)
	}
	if !strings.Contains(stderr.String(), "directory") &&
		!strings.Contains(stderr.String(), "ディレクトリ") {
		t.Errorf("expected target-not-a-directory hint, got: %s", stderr.String())
	}
}

func TestInstallSkills_UpdateThenSkip(t *testing.T) {
	// First install creates files; second install with the same SSOT
	// produces ActionSkip for everything (no drift).
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d1 := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	if _, _, err := runCmd(t, d1, "install-skills", "--target", target); err != nil {
		t.Fatalf("first install: %v", err)
	}
	d2 := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	stdout, _, err := runCmd(t, d2, "install-skills", "--target", target)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if !strings.Contains(stdout.String(), "0 created") &&
		!strings.Contains(stdout.String(), "0 件新規") {
		t.Errorf("second install should have 0 created; output:\n%s", stdout.String())
	}
}
