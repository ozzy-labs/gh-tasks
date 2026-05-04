package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/adapters"
	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func TestSanitizeDist_Rejects(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		".",
		"./",
		"..",
		"../",
		"/",
		"./.",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			_, err := sanitizeDist(in)
			if err == nil {
				t.Fatalf("sanitizeDist(%q) succeeded; want error", in)
			}
			if !strings.Contains(err.Error(), "refusing to use unsafe --dist value") {
				t.Errorf("error message missing guard prefix: %v", err)
			}
		})
	}
}

func TestSanitizeDist_Accepts(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"dist":         "dist",
		"./dist":       "dist",
		"build/dist":   "build/dist",
		"/tmp/foo":     "/tmp/foo",
		"./out/skills": "out/skills",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			got, err := sanitizeDist(in)
			if err != nil {
				t.Fatalf("sanitizeDist(%q) error: %v", in, err)
			}
			if got != want {
				t.Errorf("sanitizeDist(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

// stubAdapter is a minimal Adapter that emits a fixed list of OutputFiles.
type stubAdapter struct {
	id    string
	files []skills.OutputFile
}

func (s stubAdapter) ID() string { return s.id }
func (s stubAdapter) Generate(_ []skills.Skill) []skills.OutputFile {
	return s.files
}

func writeAll(t *testing.T, root string, files []skills.OutputFile) {
	t.Helper()
	for _, f := range files {
		dest := filepath.Join(root, f.RelativePath)
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(dest, []byte(f.Content), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
}

func newTestCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&stdout)
	c.SetErr(&stderr)
	return c, &stdout, &stderr
}

func TestRunCheckDiff_Match(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := stubAdapter{
		id: "stub",
		files: []skills.OutputFile{
			{RelativePath: "a.md", Content: "hello\n"},
			{RelativePath: "sub/b.md", Content: "world\n"},
		},
	}
	writeAll(t, filepath.Join(dir, stub.ID()), stub.files)

	c, stdout, stderr := newTestCmd()
	err := runCheckDiff(c, dir, []adapters.Adapter{stub}, nil)
	if err != nil {
		t.Fatalf("runCheckDiff error: %v (stderr=%s)", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK: dist/ matches source SSOT") {
		t.Errorf("stdout missing OK marker: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty on match: %q", stderr.String())
	}
}

func TestRunCheckDiff_ContentDiffers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := stubAdapter{
		id: "stub",
		files: []skills.OutputFile{
			{RelativePath: "a.md", Content: "expected\n"},
		},
	}
	// On-disk content differs from generated.
	writeAll(t, filepath.Join(dir, stub.ID()), []skills.OutputFile{
		{RelativePath: "a.md", Content: "stale\n"},
	})

	c, _, stderr := newTestCmd()
	err := runCheckDiff(c, dir, []adapters.Adapter{stub}, nil)
	if !errors.Is(err, ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	se := stderr.String()
	if !strings.Contains(se, "content differs") {
		t.Errorf("stderr missing 'content differs': %q", se)
	}
	if !strings.Contains(se, "FAIL: 1 file(s) differ") {
		t.Errorf("stderr missing FAIL summary: %q", se)
	}
}

func TestRunCheckDiff_MissingOnDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := stubAdapter{
		id: "stub",
		files: []skills.OutputFile{
			{RelativePath: "a.md", Content: "expected\n"},
		},
	}
	// Adapter dir does not exist on disk.

	c, _, stderr := newTestCmd()
	err := runCheckDiff(c, dir, []adapters.Adapter{stub}, nil)
	if !errors.Is(err, ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), "missing on disk") {
		t.Errorf("stderr missing 'missing on disk': %q", stderr.String())
	}
}

func TestRunCheckDiff_ExtraOnDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := stubAdapter{id: "stub", files: nil}
	writeAll(t, filepath.Join(dir, stub.ID()), []skills.OutputFile{
		{RelativePath: "stale.md", Content: "leftover\n"},
	})

	c, _, stderr := newTestCmd()
	err := runCheckDiff(c, dir, []adapters.Adapter{stub}, nil)
	if !errors.Is(err, ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), "present on disk but not generated") {
		t.Errorf("stderr missing 'present on disk' marker: %q", stderr.String())
	}
}

func TestFirstLine(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"single":        "single",
		"first\nsecond": "first ...",
		"first\n":       "first",
		"":              "",
		"a\nb\nc":       "a ...",
	}
	for in, want := range cases {
		got := firstLine(in)
		if got != want {
			t.Errorf("firstLine(%q) = %q, want %q", in, got, want)
		}
	}
}
