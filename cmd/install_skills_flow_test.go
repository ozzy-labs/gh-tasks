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

// TestInstallSkills_AgentExplicit_UnregisteredErrors used to assert the
// cmd-layer error path for an agent recognized by ValidateAgent but not
// yet wired into Adapters(). PR 5 closes the adapter matrix (every agent
// in install.Agents has a registered AdapterImpl), so the failure mode is
// no longer reachable from the public API. The unknown-agent error path
// remains covered by TestInstallSkills_AgentExplicit_UnknownRejected.

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

func TestInstallSkills_CodexCLI_Flow(t *testing.T) {
	// codex-cli (PR 3): writes .agents/skills/<name>/SKILL.md AND merges
	// a marker block into AGENTS.md. The manifest tracks both Files and
	// Shared.
	t.Parallel()
	target := t.TempDir()
	// Pre-existing AGENTS.md with user content.
	if err := os.WriteFile(filepath.Join(target, "AGENTS.md"),
		[]byte("# Project AGENTS\n\nUser preamble.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	stdout, stderr, err := runCmd(t, d, "install-skills", "--target", target, "--agent", "codex-cli")
	if err != nil {
		t.Fatalf("Execute: %v (stderr=%s)", err, stderr.String())
	}
	for _, name := range []string{"alpha", "bravo"} {
		path := filepath.Join(target, ".agents", "skills", name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}
	// AGENTS.md must retain user content AND have our marker block.
	body, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "User preamble.") {
		t.Errorf("user content lost from AGENTS.md:\n%s", string(body))
	}
	if !strings.Contains(string(body), install.MarkerBeginLine) {
		t.Errorf("AGENTS.md missing marker block:\n%s", string(body))
	}
	if !strings.Contains(string(body), "alpha") {
		t.Errorf("AGENTS.md marker block missing skill alpha:\n%s", string(body))
	}

	manifestPath := filepath.Join(target, ".agents", "skills", ".gh-tasks-manifest.json")
	mb, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest read: %v", err)
	}
	var got install.Manifest
	if err := json.Unmarshal(mb, &got); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}
	if got.Agent != install.AgentCodexCLI {
		t.Errorf("manifest agent = %q, want codex-cli", got.Agent)
	}
	if len(got.Files) != 2 {
		t.Errorf("manifest Files = %v, want 2 entries", got.Files)
	}
	if len(got.Shared) != 1 || got.Shared[0] != "AGENTS.md" {
		t.Errorf("manifest Shared = %v, want [AGENTS.md]", got.Shared)
	}
	if !strings.Contains(stdout.String(), "AGENTS.md") {
		t.Errorf("stdout should mention AGENTS.md:\n%s", stdout.String())
	}
}

func TestInstallSkills_CodexCLI_Idempotent(t *testing.T) {
	// Two consecutive installs should yield identical AGENTS.md and a
	// stable manifest.
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "AGENTS.md"), []byte("# x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d1 := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	if _, _, err := runCmd(t, d1, "install-skills", "--target", target, "--agent", "codex-cli"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	body1, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	d2 := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	if _, _, err := runCmd(t, d2, "install-skills", "--target", target, "--agent", "codex-cli"); err != nil {
		t.Fatalf("second install: %v", err)
	}
	body2, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body1) != string(body2) {
		t.Errorf("AGENTS.md drifted across runs\nfirst:\n%s\nsecond:\n%s",
			string(body1), string(body2))
	}
}

func TestInstallSkills_GeminiCLI_Flow(t *testing.T) {
	// gemini-cli (PR 4): writes .gemini/settings.json (union merge) AND
	// AGENTS.md marker block. Both Shared. The settings.json must
	// preserve any unrelated keys the user already had.
	t.Parallel()
	target := t.TempDir()
	geminiDir := filepath.Join(target, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o750); err != nil {
		t.Fatal(err)
	}
	preExisting := `{"model":"gemini-2.5-pro","temperature":0.2}`
	if err := os.WriteFile(filepath.Join(geminiDir, "settings.json"), []byte(preExisting), 0o600); err != nil {
		t.Fatal(err)
	}

	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	stdout, stderr, err := runCmd(t, d, "install-skills", "--target", target, "--agent", "gemini-cli")
	if err != nil {
		t.Fatalf("Execute: %v (stderr=%s)", err, stderr.String())
	}

	got, err := os.ReadFile(filepath.Join(geminiDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "gemini-2.5-pro") {
		t.Errorf("model lost from settings.json:\n%s", string(got))
	}
	if !strings.Contains(string(got), "0.2") {
		t.Errorf("temperature lost from settings.json:\n%s", string(got))
	}
	if !strings.Contains(string(got), "AGENTS.md") {
		t.Errorf("AGENTS.md not added to settings.json:\n%s", string(got))
	}

	manifestPath := filepath.Join(geminiDir, ".gh-tasks-manifest.json")
	mb, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest read: %v", err)
	}
	var manifest install.Manifest
	if err := json.Unmarshal(mb, &manifest); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}
	if manifest.Agent != install.AgentGeminiCLI {
		t.Errorf("manifest agent = %q, want gemini-cli", manifest.Agent)
	}
	wantShared := map[string]bool{".gemini/settings.json": false, "AGENTS.md": false}
	for _, s := range manifest.Shared {
		wantShared[s] = true
	}
	for k, present := range wantShared {
		if !present {
			t.Errorf("manifest.Shared missing %q (got %v)", k, manifest.Shared)
		}
	}
	if !strings.Contains(stdout.String(), ".gemini/settings.json") {
		t.Errorf("stdout should mention settings.json:\n%s", stdout.String())
	}
}

func TestInstallSkills_Copilot_Flow(t *testing.T) {
	// copilot (PR 5): single Shared marker-block merge into
	// .github/copilot-instructions.md. Manifest at
	// .github/.gh-tasks-copilot-manifest.json.
	t.Parallel()
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(target, ".github"), 0o750); err != nil {
		t.Fatal(err)
	}
	preExisting := "# Copilot project context\n\nUse tabs for indentation.\n"
	if err := os.WriteFile(filepath.Join(target, ".github", "copilot-instructions.md"),
		[]byte(preExisting), 0o600); err != nil {
		t.Fatal(err)
	}

	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	stdout, stderr, err := runCmd(t, d, "install-skills", "--target", target, "--agent", "copilot")
	if err != nil {
		t.Fatalf("Execute: %v (stderr=%s)", err, stderr.String())
	}

	body, err := os.ReadFile(filepath.Join(target, ".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Use tabs for indentation.") {
		t.Errorf("user content lost:\n%s", string(body))
	}
	if !strings.Contains(string(body), install.MarkerBeginLine) {
		t.Errorf("missing marker:\n%s", string(body))
	}
	if !strings.Contains(string(body), "alpha") {
		t.Errorf("skill not in marker block:\n%s", string(body))
	}

	manifestPath := filepath.Join(target, ".github", ".gh-tasks-copilot-manifest.json")
	mb, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest read: %v", err)
	}
	var manifest install.Manifest
	if err := json.Unmarshal(mb, &manifest); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}
	if manifest.Agent != install.AgentCopilot {
		t.Errorf("manifest agent = %q, want copilot", manifest.Agent)
	}
	if len(manifest.Files) != 0 {
		t.Errorf("manifest.Files = %v, want empty (copilot owns no files)", manifest.Files)
	}
	if len(manifest.Shared) != 1 || manifest.Shared[0] != ".github/copilot-instructions.md" {
		t.Errorf("manifest.Shared = %v, want [.github/copilot-instructions.md]", manifest.Shared)
	}
	if !strings.Contains(stdout.String(), "copilot-instructions.md") {
		t.Errorf("stdout should mention copilot-instructions.md:\n%s", stdout.String())
	}
}

func TestInstallSkills_Namespace_RewritesPathsAndFrontmatter(t *testing.T) {
	// PR 6: --namespace should rewrite the on-disk skill directory + the
	// frontmatter `name:` field so the slash command matches the new
	// directory name.
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })

	if _, _, err := runCmd(t, d, "install-skills", "--target", target, "--namespace", "gh-tasks"); err != nil {
		t.Fatalf("install: %v", err)
	}
	// Embedded fixture skills are "alpha" and "bravo" — neither carries
	// the `task-` prefix, so they get prepended: gh-tasks-alpha,
	// gh-tasks-bravo.
	for _, name := range []string{"gh-tasks-alpha", "gh-tasks-bravo"} {
		path := filepath.Join(target, ".claude", "skills", name, "SKILL.md")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
		want := "name: " + name + "\n"
		if !strings.Contains(string(body), want) {
			t.Errorf("%s frontmatter missing %q:\n%s", path, want, string(body))
		}
	}
	// Original (un-namespaced) directories must NOT exist.
	for _, name := range []string{"alpha", "bravo"} {
		path := filepath.Join(target, ".claude", "skills", name, "SKILL.md")
		if _, err := os.Stat(path); err == nil {
			t.Errorf("un-namespaced path %s should not exist", path)
		}
	}
	// Manifest records the namespace + the new (renamed) paths.
	manifestPath := filepath.Join(target, ".claude", "skills", ".gh-tasks-manifest.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest read: %v", err)
	}
	var got install.Manifest
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}
	if got.Namespace != "gh-tasks" {
		t.Errorf("manifest Namespace = %q, want gh-tasks", got.Namespace)
	}
	for _, want := range []string{
		".claude/skills/gh-tasks-alpha/SKILL.md",
		".claude/skills/gh-tasks-bravo/SKILL.md",
	} {
		found := false
		for _, f := range got.Files {
			if f == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("manifest Files missing %q (got %v)", want, got.Files)
		}
	}
}

func TestInstallSkills_Force_OverwritesAndBackupsThirdParty(t *testing.T) {
	// PR 6: --force should overwrite an untracked existing skill and
	// preserve the original at <path>.bak.
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	skillsDir := filepath.Join(target, ".claude", "skills", "alpha")
	if err := os.MkdirAll(skillsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	original := "third-party hand-written content\n"
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	if _, _, err := runCmd(t, d, "install-skills", "--target", target, "--force"); err != nil {
		t.Fatalf("install --force: %v", err)
	}

	// New content overwrote the third-party file.
	got, err := os.ReadFile(filepath.Join(skillsDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "name: alpha") {
		t.Errorf("post-force file does not look like the embedded SSOT:\n%s", string(got))
	}
	// Backup file holds the original.
	bak, err := os.ReadFile(filepath.Join(skillsDir, "SKILL.md.bak"))
	if err != nil {
		t.Fatalf("expected .bak: %v", err)
	}
	if string(bak) != original {
		t.Errorf("backup content = %q, want %q", string(bak), original)
	}
}

func TestInstallSkills_Conflict_OutputIncludesResolutions(t *testing.T) {
	// PR 6 衝突警告強化: the trailing error must mention --namespace and
	// --force as resolution paths so users immediately see the escape
	// hatches.
	t.Parallel()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
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
		t.Fatalf("expected ErrSilent on conflict, got %v", err)
	}
	for _, want := range []string{"--namespace", "--force"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("conflict message missing %q hint:\n%s", want, stderr.String())
		}
	}
}

func TestInstallSkills_AllAdapters_AutoDetectAll(t *testing.T) {
	// PR 5: when all four traces exist, auto-detect surfaces all four
	// agents and the cmd installs everyone in one shot.
	t.Parallel()
	target := t.TempDir()
	mustExist := []string{"CLAUDE.md", "AGENTS.md", ".github/copilot-instructions.md"}
	for _, p := range mustExist {
		full := filepath.Join(target, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("seed\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(target, ".gemini"), 0o750); err != nil {
		t.Fatal(err)
	}

	d := testDeps(nil, func(d *cmd.Deps) { d.EmbeddedSkills = embeddedFixture() })
	stdout, stderr, err := runCmd(t, d, "install-skills", "--target", target)
	if err != nil {
		t.Fatalf("Execute: %v (stderr=%s)", err, stderr.String())
	}
	for _, agent := range []string{"claude-code", "codex-cli", "gemini-cli", "copilot"} {
		if !strings.Contains(stdout.String(), agent) {
			t.Errorf("stdout missing %s adapter section:\n%s", agent, stdout.String())
		}
	}
	// Sanity: each adapter's manifest exists.
	manifestPaths := []string{
		".claude/skills/.gh-tasks-manifest.json",
		".agents/skills/.gh-tasks-manifest.json",
		".gemini/.gh-tasks-manifest.json",
		".github/.gh-tasks-copilot-manifest.json",
	}
	for _, p := range manifestPaths {
		full := filepath.Join(target, filepath.FromSlash(p))
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected manifest %s: %v", full, err)
		}
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
