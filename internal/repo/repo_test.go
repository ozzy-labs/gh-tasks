package repo_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/repo"
)

func TestParseOwnerName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want repo.Ident
		err  bool
	}{
		{"ozzy-labs/gh-tasks", repo.Ident{Owner: "ozzy-labs", Name: "gh-tasks"}, false},
		{"a/b", repo.Ident{Owner: "a", Name: "b"}, false},
		{"too/many/slashes", repo.Ident{}, true},
		{"only-one", repo.Ident{}, true},
		{"/missing-owner", repo.Ident{}, true},
		{"missing-name/", repo.Ident{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := repo.ParseOwnerName(tc.in)
			if tc.err {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExtractFromRemote(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"git@github.com:ozzy-labs/gh-tasks.git", "ozzy-labs/gh-tasks", false},
		{"https://github.com/ozzy-labs/gh-tasks.git", "ozzy-labs/gh-tasks", false},
		{"https://github.com/ozzy-labs/gh-tasks", "ozzy-labs/gh-tasks", false},
		{"ssh://git@github.com/ozzy-labs/gh-tasks.git", "ozzy-labs/gh-tasks", false},
		{"https://github.com/ozzy-labs/gh-tasks/", "ozzy-labs/gh-tasks", false},
		{"not-a-url", "", true},
		// Multi-segment paths must be rejected. The legacy regex-only
		// implementation matched the last two segments greedily, silently
		// extracting "extra/path" from these URLs.
		{"https://github.com/owner/name/extra/path", "", true},
		{"https://github.com/owner/name/issues/42", "", true},
		{"git@github.com:owner/name/extra", "", true},
		// A single-segment path has no name component to extract.
		{"https://github.com/just-an-owner", "", true},
		// A scheme-less, colon-less string (no host/path boundary) is
		// not a valid git remote.
		{"github.com/owner/name", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := repo.ExtractFromRemote(tc.in)
			if tc.err {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolve_FlagWins(t *testing.T) {
	t.Parallel()
	got, err := repo.Resolve(repo.ResolveOptions{Flag: "ozzy-labs/gh-tasks"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.String() != "ozzy-labs/gh-tasks" {
		t.Errorf("got %q", got.String())
	}
}

func TestResolve_RemoteFallback(t *testing.T) {
	t.Parallel()
	got, err := repo.Resolve(repo.ResolveOptions{
		GetRemoteURL: func() (string, bool) { return "git@github.com:foo/bar.git", true },
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.String() != "foo/bar" {
		t.Errorf("got %q", got.String())
	}
}

func TestResolve_NoSourceFails(t *testing.T) {
	t.Parallel()
	_, err := repo.Resolve(repo.ResolveOptions{
		GetRemoteURL: func() (string, bool) { return "", false },
	})
	if err == nil {
		t.Fatalf("want error")
	}
}

// TestResolve_EmptyFlagFallsThroughToRemote covers the case where cobra
// reports an empty --repo value (e.g. user typed `--repo=` or didn't pass
// it at all). Resolve must fall through to the git remote rather than
// surfacing an invalid-identifier error on "".
func TestResolve_EmptyFlagFallsThroughToRemote(t *testing.T) {
	t.Parallel()
	got, err := repo.Resolve(repo.ResolveOptions{
		Flag:         "",
		GetRemoteURL: func() (string, bool) { return "git@github.com:foo/bar.git", true },
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.String() != "foo/bar" {
		t.Errorf("got %q; want fallthrough to remote", got.String())
	}
}

// TestResolve_NilContextUsesBackground verifies that an unset Context falls
// through to context.Background() so callers don't have to plumb a context
// through trivial flag-only resolutions.
func TestResolve_NilContextUsesBackground(t *testing.T) {
	t.Parallel()
	got, err := repo.Resolve(repo.ResolveOptions{
		Flag: "ozzy-labs/gh-tasks",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.String() != "ozzy-labs/gh-tasks" {
		t.Errorf("got %q", got.String())
	}
}

// TestResolve_ContextHonoured verifies that ResolveOptions.Context propagates
// to the default git remote lookup. We pass an already-cancelled context and
// confirm that — when GetRemoteURL is nil and the flag is absent — the
// resolution surfaces a missing-remote error rather than blocking.
func TestResolve_ContextHonoured(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := repo.Resolve(repo.ResolveOptions{
		Context: ctx,
		// No --repo flag; no override GetRemoteURL. Falls to default which
		// runs git under the cancelled context and returns ("", false).
	})
	if err == nil {
		t.Fatalf("want error from missing remote; got nil")
	}
}

// TestResolve_DefaultGetRemoteURL_NoRemote pins the failure branch of
// defaultGetRemoteURL by exercising it through Resolve in a directory that
// is *not* a git repository. The helper must return ("", false) so Resolve
// surfaces the localized "error.repo.notResolved" code.
//
// Together with TestResolve_DefaultGetRemoteURL_Success (success branch via a
// real git fixture) and TestResolve_ContextHonoured (cancelled-context
// branch), the three tests cover defaultGetRemoteURL's full surface without
// stubbing exec.Command.
func TestResolve_DefaultGetRemoteURL_NoRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	// Cannot t.Parallel() because we mutate cwd.
	chdirToTempOrSkip(t)
	_, err := repo.Resolve(repo.ResolveOptions{}) // no flag, no override → default
	if err == nil {
		t.Fatal("expected error when no remote is resolvable; got nil")
	}
}

// TestResolve_DefaultGetRemoteURL_Success pins the happy path of
// defaultGetRemoteURL by initialising a temp git repo with a fixed `origin`
// remote, chdir-ing into it, and asserting that Resolve (with default
// helper) returns the parsed Ident.
//
// This is the only test that exercises the "git ran successfully and
// returned a non-empty URL" branch of defaultGetRemoteURL without
// stubbing exec.Command. It is necessarily non-parallel (chdir).
func TestResolve_DefaultGetRemoteURL_Success(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := chdirToTempOrSkip(t)

	// Initialise a minimal repo. `git init` writes only into dir.
	mustRunGit(t, dir, "init", "--quiet", "-b", "main")
	// Set deterministic identity so the init doesn't barf on missing
	// user.email in sandboxed CI. Use --local to keep the change scoped.
	mustRunGit(t, dir, "config", "--local", "user.email", "test@example.invalid")
	mustRunGit(t, dir, "config", "--local", "user.name", "test")
	// Add a fixed origin we can assert against. The URL doesn't need to
	// be reachable — `git remote get-url` just reads .git/config.
	const wantURL = "git@github.com:ozzy-labs/test-fixture.git"
	mustRunGit(t, dir, "remote", "add", "origin", wantURL)

	got, err := repo.Resolve(repo.ResolveOptions{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.String() != "ozzy-labs/test-fixture" {
		t.Errorf("got %q, want %q", got.String(), "ozzy-labs/test-fixture")
	}
}

// chdirToTempOrSkip creates a TempDir, resolves symlinks, chdirs into it
// via t.Chdir (auto-restored on cleanup, mutually exclusive with t.Parallel
// so concurrent tests can't race the process-global cwd), unsets git
// environment variables that would force `git remote get-url origin` to
// read from a different repo, and skips the test if any ancestor contains
// `.git` (which would taint git's lookup). Returns the resolved temp path.
//
// The git env-var hygiene matters because lefthook invokes the test binary
// from a `git push` pre-push hook, and `git push` exports GIT_DIR /
// GIT_WORK_TREE so subprocess `git` calls bypass the cwd-based lookup. The
// production defaultGetRemoteURL inherits these too — that's the
// historically intended behaviour for users invoking the CLI from within a
// hook — but the tests in this file need a deterministic isolated repo.
func chdirToTempOrSkip(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		resolved = tmp
	}
	if hasAncestorGitDir(resolved) {
		t.Skipf("temp dir %q has an ancestor .git; git would inherit it", resolved)
	}
	// Unset every GIT_* variable that would redirect a child `git`
	// invocation away from the cwd-based discovery path. We register the
	// original value with a Cleanup so a panic-free teardown restores
	// inherited state (lefthook → `git push` populates GIT_DIR /
	// GIT_WORK_TREE before invoking the test binary from a pre-push hook).
	//
	// List sourced from `git help environment`'s "repository locations"
	// section. We use os.Unsetenv (not t.Setenv("","")) because git
	// distinguishes "GIT_DIR unset" from "GIT_DIR=''" — the latter is
	// treated as a literal empty path and aborts.
	for _, key := range []string{
		"GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_COMMON_DIR",
		"GIT_INDEX_FILE",
		"GIT_OBJECT_DIRECTORY",
	} {
		key := key
		prev, ok := os.LookupEnv(key)
		_ = os.Unsetenv(key)
		t.Cleanup(func() {
			if ok {
				_ = os.Setenv(key, prev)
			}
		})
	}
	t.Chdir(resolved)
	return resolved
}

// mustRunGit runs `git -C dir <args...>` and fails the test on non-zero exit.
// Used by the success-path fixture to set up a deterministic origin remote.
func mustRunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	// Ignore any user-level git config that might interfere (commit.gpgsign,
	// init.defaultBranch hooks, etc).
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// hasAncestorGitDir walks up from dir looking for a `.git` directory or
// file (worktree marker). Used by chdirToTempOrSkip to skip when running
// inside a git repo would taint the result.
func hasAncestorGitDir(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
