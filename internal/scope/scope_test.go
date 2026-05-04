package scope_test

import (
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

func TestParseFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		argv     []string
		wantOK   bool
		want     scope.Scope
		wantFail bool
	}{
		{name: "empty", wantOK: false},
		{name: "equals-form", argv: []string{"--scope=repo"}, wantOK: true, want: scope.Repo},
		{name: "space-form", argv: []string{"--scope", "org"}, wantOK: true, want: scope.Org},
		{name: "user", argv: []string{"--scope=user"}, wantOK: true, want: scope.User},
		{name: "invalid-value", argv: []string{"--scope=team"}, wantFail: true},
		{name: "missing-value", argv: []string{"--scope"}, wantFail: true},
		{name: "ignores-other-flags", argv: []string{"--repo=a/b", "--scope=repo"}, wantOK: true, want: scope.Repo},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok, err := scope.ParseFlag(tc.argv)
			if tc.wantFail {
				if err == nil {
					t.Fatalf("want error, got nil; result=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("got (%q,%v), want (%q,%v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()

	t.Run("flag-wins-over-everything", func(t *testing.T) {
		t.Parallel()
		got, err := scope.Detect(scope.DetectOptions{
			Argv:         []string{"--scope=user"},
			HasGitRemote: func() bool { return true },
			DefaultScope: scope.Org,
		})
		if err != nil || got != scope.User {
			t.Fatalf("got %q err=%v", got, err)
		}
	})

	t.Run("git-remote-wins-over-default", func(t *testing.T) {
		t.Parallel()
		got, err := scope.Detect(scope.DetectOptions{
			HasGitRemote: func() bool { return true },
			DefaultScope: scope.Org,
		})
		if err != nil || got != scope.Repo {
			t.Fatalf("got %q err=%v", got, err)
		}
	})

	t.Run("default-when-no-remote", func(t *testing.T) {
		t.Parallel()
		got, err := scope.Detect(scope.DetectOptions{
			HasGitRemote: func() bool { return false },
			DefaultScope: scope.Org,
		})
		if err != nil || got != scope.Org {
			t.Fatalf("got %q err=%v", got, err)
		}
	})

	t.Run("fallback-to-user", func(t *testing.T) {
		t.Parallel()
		got, err := scope.Detect(scope.DetectOptions{
			HasGitRemote: func() bool { return false },
		})
		if err != nil || got != scope.User {
			t.Fatalf("got %q err=%v", got, err)
		}
	})
}
