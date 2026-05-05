package cmd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
)

// TestErrSilentSentinelChain verifies the wrapping invariant: both args and
// runtime variants satisfy errors.Is(err, ErrSilent), but they remain
// distinguishable from each other so main can pick the exit code.
func TestErrSilentSentinelChain(t *testing.T) {
	t.Parallel()

	if !errors.Is(cmd.ErrSilentArgs, cmd.ErrSilent) {
		t.Errorf("ErrSilentArgs must satisfy errors.Is(_, ErrSilent)")
	}
	if !errors.Is(cmd.ErrSilentRuntime, cmd.ErrSilent) {
		t.Errorf("ErrSilentRuntime must satisfy errors.Is(_, ErrSilent)")
	}
	if errors.Is(cmd.ErrSilentArgs, cmd.ErrSilentRuntime) {
		t.Errorf("ErrSilentArgs must not satisfy errors.Is(_, ErrSilentRuntime)")
	}
	if errors.Is(cmd.ErrSilentRuntime, cmd.ErrSilentArgs) {
		t.Errorf("ErrSilentRuntime must not satisfy errors.Is(_, ErrSilentArgs)")
	}
}

// TestList_ScopeFlagInvalidExitCode pins the arg-validation path: an invalid
// --scope must surface ErrSilentArgs (exit 2) rather than ErrSilentRuntime.
func TestList_ScopeFlagInvalidExitCode(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.Argv = []string{"gh-tasks", "list", "--scope=bogus"}
	})
	_, _, err := runCmd(t, d, "list", "--scope=bogus")
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs (exit 2) for invalid --scope, got %v", err)
	}
	if errors.Is(err, cmd.ErrSilentRuntime) {
		t.Fatalf("expected NOT-runtime classification for invalid --scope, got %v", err)
	}
}

// TestReview_PeriodFlagInvalidExitCode pins arg-validation classification for
// invalid --period values (PeriodError).
func TestReview_PeriodFlagInvalidExitCode(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "review", "--period", "yearly")
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs for invalid --period, got %v", err)
	}
}

// TestList_ProjectFlagInvalidExitCode pins arg-validation classification for
// invalid --project values (ProjectError).
func TestList_ProjectFlagInvalidExitCode(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.Argv = []string{"gh-tasks", "list", "--scope=org", "--project=bogus"}
	})
	_, _, err := runCmd(t, d, "list", "--scope=org", "--project=bogus")
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs for invalid --project, got %v", err)
	}
}

// TestList_RepoNotResolvedExitCode pins arg-validation classification for an
// unresolvable repo (RepoError) — even though it surfaces as a "could not
// resolve" message, it stems from missing argv / config.
func TestList_RepoNotResolvedExitCode(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return true }
		d.GetRemoteURL = func() (string, bool) { return "", false }
	})
	_, _, err := runCmd(t, d, "list")
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs for unresolved repo, got %v", err)
	}
}

// TestAdd_TitleRequiredExitCode pins direct-return arg-validation: missing
// positional argument must classify as ErrSilentArgs.
func TestAdd_TitleRequiredExitCode(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "add", "")
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs for missing add <title>, got %v", err)
	}
}

// TestDone_IDRequiredExitCode pins direct-return arg-validation: missing
// positional argument for `done` must classify as ErrSilentArgs.
func TestDone_IDRequiredExitCode(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "done", "")
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs for empty done <id>, got %v", err)
	}
}

// TestList_RepoNotFoundExitCode pins runtime classification: GraphQL responded
// successfully but the repository node is null (issued because the user-facing
// repo just isn't there in the API result, which is a runtime — not arg —
// failure).
func TestList_RepoNotFoundExitCode(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query ListRepoIssues(", data: map[string]any{"repository": nil}},
	}}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "list")
	if !errors.Is(err, cmd.ErrSilentRuntime) {
		t.Fatalf("expected ErrSilentRuntime for repo notFound, got %v", err)
	}
	if errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected NOT-arg classification for repo notFound, got %v", err)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected localized notFound message on stderr, got:\n%s", stderr.String())
	}
}
