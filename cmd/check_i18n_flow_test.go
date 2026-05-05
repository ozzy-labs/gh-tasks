// Cobra-rooted flow tests for the hidden `check-i18n` command. These wrap
// runCheckI18n with a fixture root passed as a positional argument so the
// scan target is fully under the test's control (no dependency on the
// surrounding worktree's actual code).
package cmd_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
)

func TestCheckI18n_CleanState(t *testing.T) {
	t.Parallel()

	// Fixture: ASCII-only Go source under a tmp root. check-i18n accepts
	// positional args as scan roots; passing the tmp dir keeps the test
	// independent of repo state.
	tmp := t.TempDir()
	src := `package x

import "fmt"

func F() {
	fmt.Println("plain ascii only")
}
`
	if err := os.WriteFile(filepath.Join(tmp, "src.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	d := testDeps(nil)
	stdout, _, err := runCmd(t, d, "check-i18n", tmp)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "OK: scanned") || !strings.Contains(got, "no hardcoded non-ASCII literals") {
		t.Errorf("missing OK marker in stdout:\n%s", got)
	}
}

func TestCheckI18n_DetectsHardcodedLiteral(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	// Hardcoded ja literal — the scanner's primary target.
	src := "package x\n\nvar S = \"" + "こんにちは" + "\"\n"
	if err := os.WriteFile(filepath.Join(tmp, "bad.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	d := testDeps(nil)
	_, stderr, err := runCmd(t, d, "check-i18n", tmp)
	if err == nil {
		t.Fatalf("expected non-nil err on hit, got nil (stderr=%s)", stderr.String())
	}
	if !errors.Is(err, cmd.ErrSilent) {
		t.Errorf("expected ErrSilent chain, got %v", err)
	}
	se := stderr.String()
	if !strings.Contains(se, "hardcoded non-ASCII literal") {
		t.Errorf("stderr missing per-hit line:\n%s", se)
	}
	if !strings.Contains(se, "1 hardcoded non-ASCII literal(s) detected.") {
		t.Errorf("stderr missing summary line:\n%s", se)
	}
}
