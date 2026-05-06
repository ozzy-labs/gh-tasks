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

func TestAutoDetect_PR3Filtering(t *testing.T) {
	// AutoDetect must only return agents whose adapters are registered
	// for the current build. PR 3 registers claude-code + codex-cli;
	// .gemini/ should not surface as an auto-detected agent until PR 4.
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "CLAUDE.md"), "# CLAUDE\n")
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "# AGENTS\n")
	mustMkdir(t, filepath.Join(root, ".gemini"))

	got := AutoDetect(root)
	if len(got) != 2 || got[0] != AgentClaudeCode || got[1] != AgentCodexCLI {
		t.Errorf("AutoDetect = %v; PR 3 expects [claude-code, codex-cli]", got)
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
