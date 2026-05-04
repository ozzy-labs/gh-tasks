package scope_test

import (
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

func TestDetect(t *testing.T) {
	t.Parallel()

	t.Run("flag-wins-over-everything", func(t *testing.T) {
		t.Parallel()
		got, err := scope.Detect(scope.DetectOptions{
			Flag:         "user",
			HasGitRemote: func() bool { return true },
			DefaultScope: scope.Org,
		})
		if err != nil || got != scope.User {
			t.Fatalf("got %q err=%v", got, err)
		}
	})

	t.Run("invalid-flag-errors", func(t *testing.T) {
		t.Parallel()
		_, err := scope.Detect(scope.DetectOptions{Flag: "team"})
		if err == nil {
			t.Fatalf("want error for invalid flag value")
		}
	})

	t.Run("empty-flag-falls-through", func(t *testing.T) {
		t.Parallel()
		got, err := scope.Detect(scope.DetectOptions{
			Flag:         "",
			HasGitRemote: func() bool { return true },
			DefaultScope: scope.Org,
		})
		if err != nil || got != scope.Repo {
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
