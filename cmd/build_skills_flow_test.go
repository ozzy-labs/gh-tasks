// Cobra-rooted flow tests for the hidden `build-skills` command. These
// exercise the full `runBuildSkills` path (load skills -> generate per
// adapter -> write dist + stage into <repoRoot>/.claude/skills,
// .agents/skills) against a synthetic skill fixture and a temp dist root.
//
// Tests t.Chdir into a temp working directory so the local-stage step
// (defaultLocalStages uses os.Getwd) can never escape the test sandbox and
// pollute the surrounding worktree. t.Chdir is mutually exclusive with
// t.Parallel; a comment at the top of each test records that trade-off.
package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalSkillFixture writes a single skill (`fixture-skill`) under
// <root>/skills/<name>/ with both SKILL.md and SKILL.en.md. The frontmatter
// matches the schema enforced by skills.Load (defaultRequiredFields +
// locale=ja for canonical, locale=en for mirror).
func minimalSkillFixture(t *testing.T, root string) {
	t.Helper()
	name := "fixture-skill"
	dir := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	canonical := `---
name: fixture-skill
description: fixture skill for build-skills flow tests
description_en: fixture skill for build-skills flow tests
allowed-tools: Bash(gh:*)
locale: ja
---

# fixture-skill

Body content for the fixture skill.
`
	mirror := `---
name: fixture-skill
description: fixture skill for build-skills flow tests
description_en: fixture skill for build-skills flow tests
allowed-tools: Bash(gh:*)
locale: en
---

# fixture-skill

Body content (en mirror) for the fixture skill.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(canonical), 0o600); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.en.md"), []byte(mirror), 0o600); err != nil {
		t.Fatalf("write SKILL.en.md: %v", err)
	}
}

func TestBuildSkills_GeneratesAllAdapters(t *testing.T) {
	// No t.Parallel: t.Chdir is required to keep defaultLocalStages
	// (os.Getwd-rooted) from writing into the surrounding worktree, and
	// t.Chdir is incompatible with t.Parallel.
	tmp := t.TempDir()
	minimalSkillFixture(t, tmp)
	t.Chdir(tmp)

	dist := filepath.Join(tmp, "dist")
	d := testDeps(nil)
	stdout, _, err := runCmd(t, d, "build-skills", "--src", "skills", "--dist", dist)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	for _, adapter := range []string{"claude-code", "codex-cli", "gemini-cli", "copilot"} {
		adapterDir := filepath.Join(dist, adapter)
		info, statErr := os.Stat(adapterDir)
		if statErr != nil {
			t.Errorf("expected adapter dir %s, stat err: %v", adapterDir, statErr)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", adapterDir)
		}
	}

	got := stdout.String()
	if !strings.Contains(got, "Built 1 skill(s) for 4 adapters") {
		t.Errorf("missing build summary in stdout:\n%s", got)
	}
	if !strings.Contains(got, "fixture-skill") {
		t.Errorf("missing fixture-skill name in stdout:\n%s", got)
	}
}

func TestBuildSkills_CheckDiffClean(t *testing.T) {
	// No t.Parallel: see TestBuildSkills_GeneratesAllAdapters.
	tmp := t.TempDir()
	minimalSkillFixture(t, tmp)
	t.Chdir(tmp)

	dist := filepath.Join(tmp, "dist")

	// Seed the dist tree by running build-skills first, so --check-diff has
	// a fully matching baseline to compare against.
	d := testDeps(nil)
	if _, _, err := runCmd(t, d, "build-skills", "--src", "skills", "--dist", dist); err != nil {
		t.Fatalf("seed Execute: %v", err)
	}

	// Re-run with --check-diff against the freshly-generated tree: no drift.
	d = testDeps(nil)
	stdout, _, err := runCmd(t, d, "build-skills", "--check-diff", "--src", "skills", "--dist", dist)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "OK: dist/ matches source SSOT") {
		t.Errorf("missing OK marker:\n%s", stdout.String())
	}
}

func TestBuildSkills_CheckDiffDetectsDrift(t *testing.T) {
	// No t.Parallel: see TestBuildSkills_GeneratesAllAdapters.
	tmp := t.TempDir()
	minimalSkillFixture(t, tmp)
	t.Chdir(tmp)

	dist := filepath.Join(tmp, "dist")

	// Seed the dist tree.
	d := testDeps(nil)
	if _, _, err := runCmd(t, d, "build-skills", "--src", "skills", "--dist", dist); err != nil {
		t.Fatalf("seed Execute: %v", err)
	}

	// Inject drift: corrupt the claude-code adapter's emitted SKILL.md.
	skillPath := filepath.Join(dist, "claude-code", ".claude", "skills", "fixture-skill", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("DRIFT\n"), 0o600); err != nil {
		t.Fatalf("inject drift: %v", err)
	}

	d = testDeps(nil)
	_, stderr, err := runCmd(t, d, "build-skills", "--check-diff", "--src", "skills", "--dist", dist)
	if err == nil {
		t.Fatalf("expected non-nil err on drift, got nil (stderr=%s)", stderr.String())
	}
	se := stderr.String()
	if !strings.Contains(se, "content differs") {
		t.Errorf("stderr missing 'content differs':\n%s", se)
	}
	if !strings.Contains(se, "FAIL:") {
		t.Errorf("stderr missing FAIL summary:\n%s", se)
	}
}
