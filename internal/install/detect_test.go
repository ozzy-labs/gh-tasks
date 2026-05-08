package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectClaudeCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  bool
	}{
		{
			name:  "empty-dir-not-detected",
			setup: func(_ *testing.T, _ string) {},
			want:  false,
		},
		{
			name: "claude-dir-detected",
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o750); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "claude-md-detected",
			setup: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# CLAUDE\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "claude-md-as-directory-still-counts-as-file-check-fails",
			// The detector for CLAUDE.md only triggers on a regular file.
			// Verifies isFile filters out directories named CLAUDE.md.
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "CLAUDE.md"), 0o750); err != nil {
					t.Fatal(err)
				}
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			tc.setup(t, root)
			if got := DetectClaudeCode(root); got != tc.want {
				t.Errorf("DetectClaudeCode = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDetectCodexCLI(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  bool
	}{
		{
			name:  "empty-dir-not-detected",
			setup: func(_ *testing.T, _ string) {},
			want:  false,
		},
		{
			name: "codex-dir-detected",
			// Codex CLI's project-local config dir, parallel to .claude/
			// and .gemini/ — this is the primary "Codex is installed" signal
			// before any gh-tasks files exist.
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o750); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "agents-dir-detected-without-skills-subdir",
			// Regression: `.agents/` alone (without `.agents/skills/`) used
			// to slip past the detector, leaving codex-cli out of auto-detect
			// before `gh tasks install-skills` had ever run.
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o750); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "agents-skills-subdir-still-detected",
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, ".agents", "skills"), 0o750); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
		{
			name: "agents-md-detected",
			setup: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# AGENTS\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			tc.setup(t, root)
			if got := DetectCodexCLI(root); got != tc.want {
				t.Errorf("DetectCodexCLI = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAutoDetect_CodexCLIFromCodexDir(t *testing.T) {
	// Regression for the real-world report: a repo with `.codex/` and
	// `.agents/` but no AGENTS.md and no `.agents/skills/` subdir was
	// being mis-detected as claude-code + copilot only.
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, root+"/CLAUDE.md", "# CLAUDE\n")
	mustMkdir(t, root+"/.codex")
	mustMkdir(t, root+"/.agents")
	mustMkdir(t, root+"/.github")
	mustWrite(t, root+"/.github/copilot-instructions.md", "x")

	got := AutoDetect(root)
	want := []Agent{AgentClaudeCode, AgentCodexCLI, AgentCopilot}
	if len(got) != len(want) {
		t.Fatalf("AutoDetect len = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AutoDetect[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAutoDetect_PR5AllFourAdapters(t *testing.T) {
	// PR 5 closes the adapter matrix: every agent now both has a Detect
	// hook and a registered adapter, so AutoDetect should surface all
	// four when their traces exist.
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "CLAUDE.md"), "# CLAUDE\n")
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "# AGENTS\n")
	mustMkdir(t, filepath.Join(root, ".gemini"))
	mustMkdir(t, filepath.Join(root, ".github"))
	mustWrite(t, filepath.Join(root, ".github", "copilot-instructions.md"), "x")

	got := AutoDetect(root)
	want := []Agent{AgentClaudeCode, AgentCodexCLI, AgentGeminiCLI, AgentCopilot}
	if len(got) != len(want) {
		t.Fatalf("AutoDetect len = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AutoDetect[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAutoDetect_NoAgent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if got := AutoDetect(root); len(got) != 0 {
		t.Errorf("AutoDetect on empty dir = %v, want []", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatal(err)
	}
}
