package project_test

import (
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

func TestParseIdentifier(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want project.Ref
		ok   bool
	}{
		{"ozzy-labs/3", project.Ref{Owner: "ozzy-labs", Number: 3}, true},
		{"a/1", project.Ref{Owner: "a", Number: 1}, true},
		{"  ozzy-labs/12  ", project.Ref{Owner: "ozzy-labs", Number: 12}, true},
		{"ozzy-labs/abc", project.Ref{}, false},
		{"ozzy-labs/0", project.Ref{}, false},
		{"ozzy-labs/-1", project.Ref{}, false},
		{"ozzy-labs/", project.Ref{}, false},
		{"/3", project.Ref{}, false},
		{"", project.Ref{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, ok := project.ParseIdentifier(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Errorf("got (%v,%v) want (%v,%v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestParseFlag(t *testing.T) {
	t.Parallel()

	t.Run("absent", func(t *testing.T) {
		t.Parallel()
		_, present, err := project.ParseFlag([]string{"--scope=org"})
		if err != nil || present {
			t.Fatalf("got (present=%v, err=%v)", present, err)
		}
	})

	t.Run("equals-form", func(t *testing.T) {
		t.Parallel()
		ref, present, err := project.ParseFlag([]string{"--project=foo/7"})
		if err != nil || !present {
			t.Fatalf("got (present=%v, err=%v)", present, err)
		}
		if ref.Owner != "foo" || ref.Number != 7 {
			t.Errorf("got %v", ref)
		}
	})

	t.Run("missing-value-is-present-with-error", func(t *testing.T) {
		t.Parallel()
		_, present, err := project.ParseFlag([]string{"--project"})
		if err == nil {
			t.Fatalf("want error")
		}
		if !present {
			t.Errorf("present=false on malformed flag; want true (flag was present)")
		}
	})

	t.Run("invalid-identifier-is-present-with-error", func(t *testing.T) {
		t.Parallel()
		_, present, err := project.ParseFlag([]string{"--project=bad"})
		if err == nil {
			t.Fatalf("want error")
		}
		if !present {
			t.Errorf("present=false on malformed flag; want true (flag was present)")
		}
	})
}

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("repo-scope-errors", func(t *testing.T) {
		t.Parallel()
		_, err := project.Resolve(project.ResolveOptions{Scope: scope.Repo})
		if err == nil {
			t.Fatalf("want error")
		}
	})

	t.Run("flag-wins", func(t *testing.T) {
		t.Parallel()
		got, err := project.Resolve(project.ResolveOptions{
			Scope:      scope.Org,
			Argv:       []string{"--project=foo/7"},
			OrgProject: project.Ref{Owner: "config", Number: 9},
		})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		want := project.Ref{Owner: "foo", Number: 7}
		if got != want {
			t.Errorf("got %v want %v", got, want)
		}
	})

	t.Run("falls-back-to-org-config", func(t *testing.T) {
		t.Parallel()
		got, err := project.Resolve(project.ResolveOptions{
			Scope:      scope.Org,
			OrgProject: project.Ref{Owner: "configured", Number: 5},
		})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got.Number != 5 || got.Owner != "configured" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("falls-back-to-user-config", func(t *testing.T) {
		t.Parallel()
		got, err := project.Resolve(project.ResolveOptions{
			Scope:       scope.User,
			UserProject: project.Ref{Owner: "u", Number: 1},
		})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got.Owner != "u" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("no-flag-no-config-errors", func(t *testing.T) {
		t.Parallel()
		_, err := project.Resolve(project.ResolveOptions{Scope: scope.Org})
		if err == nil {
			t.Fatalf("want error")
		}
	})
}
