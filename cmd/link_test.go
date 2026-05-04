package cmd_test

import (
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
)

func TestContainsCloseLink(t *testing.T) {
	t.Parallel()

	cases := []struct {
		body string
		num  int
		want bool
	}{
		{"Closes #42", 42, true},
		{"Fixes #42", 42, true},
		{"Resolves #42", 42, true},
		{"closes #42", 42, true},
		{"Closes #41", 42, false},
		{"This closes #42 in passing", 42, true},
		{"#42", 42, false},
		{"", 42, false},
		{"Closes #4", 42, false},
		{"Closes #420", 42, false},
	}
	for _, tc := range cases {
		got := cmd.ContainsCloseLink(tc.body, tc.num)
		if got != tc.want {
			t.Errorf("ContainsCloseLink(%q, %d) = %v, want %v", tc.body, tc.num, got, tc.want)
		}
	}
}

func TestAppendCloseLink(t *testing.T) {
	t.Parallel()

	cases := []struct {
		body string
		num  int
		want string
	}{
		{"", 42, "Closes #42\n"},
		{"existing body", 42, "existing body\n\nCloses #42\n"},
		{"existing body\n", 42, "existing body\n\nCloses #42\n"},
		{"existing body\n\n\n", 42, "existing body\n\nCloses #42\n"},
	}
	for _, tc := range cases {
		got := cmd.AppendCloseLink(tc.body, tc.num)
		if got != tc.want {
			t.Errorf("AppendCloseLink(%q, %d) = %q, want %q", tc.body, tc.num, got, tc.want)
		}
		if !strings.HasSuffix(got, "Closes #42\n") {
			t.Errorf("missing trailing close link in %q", got)
		}
	}
}
