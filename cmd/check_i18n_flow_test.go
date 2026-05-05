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

func TestCheckI18n_RefsFlag_CleanState(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	// Fixture references only keys that exist in the real en/ja catalog so
	// the scanner walks a non-trivial AST yet finds zero missing refs. We
	// stub a `T` method on a local type so the SelectorExpr matches without
	// importing the real i18n package.
	src := `package x

type R struct{}

func (r R) T(key string, args ...any) string { _ = key; _ = args; return "" }

func F(r R) { _ = r.T("list.empty") }
`
	if err := os.WriteFile(filepath.Join(tmp, "src.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	d := testDeps(nil)
	stdout, _, err := runCmd(t, d, "check-i18n", "--refs", tmp)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "OK: scanned") {
		t.Errorf("missing OK marker:\n%s", got)
	}
	if !strings.Contains(got, "every r.T / NewPayload literal key resolves") {
		t.Errorf("missing summary marker:\n%s", got)
	}
}

func TestCheckI18n_RefsFlag_DetectsUndefinedKey(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	src := `package x

type R struct{}

func (r R) T(key string, args ...any) string { _ = key; _ = args; return "" }

func F(r R) { _ = r.T("definitely.not.in.catalog.xyzzy") }
`
	if err := os.WriteFile(filepath.Join(tmp, "src.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	d := testDeps(nil)
	_, stderr, err := runCmd(t, d, "check-i18n", "--refs", tmp)
	if err == nil {
		t.Fatalf("expected non-nil err on undefined key, got nil (stderr=%s)", stderr.String())
	}
	if !errors.Is(err, cmd.ErrSilent) {
		t.Errorf("expected ErrSilent chain, got %v", err)
	}
	se := stderr.String()
	if !strings.Contains(se, "undefined i18n key") {
		t.Errorf("stderr missing per-hit line:\n%s", se)
	}
	if !strings.Contains(se, "definitely.not.in.catalog.xyzzy") {
		t.Errorf("stderr missing the offending key:\n%s", se)
	}
	if !strings.Contains(se, "1 undefined i18n key reference(s) detected.") {
		t.Errorf("stderr missing summary line:\n%s", se)
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
