package repo_test

import (
	"context"
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
