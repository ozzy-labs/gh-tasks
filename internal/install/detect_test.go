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

func TestAutoDetect_PR4Filtering(t *testing.T) {
	// AutoDetect must only return agents whose adapters are registered
	// for the current build. PR 4 adds gemini-cli; copilot is still
	// stubbed out and should not surface even when its trace exists.
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "CLAUDE.md"), "# CLAUDE\n")
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "# AGENTS\n")
	mustMkdir(t, filepath.Join(root, ".gemini"))
	mustMkdir(t, filepath.Join(root, ".github"))
	mustWrite(t, filepath.Join(root, ".github", "copilot-instructions.md"), "x")

	got := AutoDetect(root)
	want := []Agent{AgentClaudeCode, AgentCodexCLI, AgentGeminiCLI}
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
