package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

func newWarningResolved() Resolved {
	return Resolved{Locale: i18n.LocaleEN}
}

func TestWarnIfTruncated_BoundaryConditions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		fetched   int
		limit     int
		wantWarn  bool
		wantLabel string
	}{
		{name: "below limit emits nothing", fetched: 99, limit: 100, wantWarn: false},
		{name: "at limit warns", fetched: 100, limit: 100, wantWarn: true, wantLabel: "repository issues"},
		{name: "above limit warns (defensive)", fetched: 101, limit: 100, wantWarn: true, wantLabel: "repository issues"},
		{name: "zero limit treats every response as truncated", fetched: 0, limit: 0, wantWarn: true, wantLabel: "repository issues"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			c := &cobra.Command{}
			c.SetErr(&stderr)
			warnIfTruncated(c, newWarningResolved(), kindRepoIssues, tc.fetched, tc.limit)
			out := stderr.String()
			if tc.wantWarn {
				if out == "" {
					t.Fatalf("expected warning, got empty stderr")
				}
				if !strings.Contains(out, tc.wantLabel) {
					t.Errorf("warning %q does not contain localized label %q", out, tc.wantLabel)
				}
			} else if out != "" {
				t.Errorf("expected no warning, got %q", out)
			}
		})
	}
}

func TestTruncationKindLabel_AllKindsResolve(t *testing.T) {
	t.Parallel()
	r := newWarningResolved()
	cases := map[truncationKind]string{
		kindRepoIssues:   "repository issues",
		kindClosedIssues: "closed issues",
		kindMergedPRs:    "merged pull requests",
		kindOpenIssues:   "open issues",
		kindMilestones:   "milestones",
		kindProjectItems: "project items",
	}
	for kind, want := range cases {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			if got := truncationKindLabel(r, kind); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestTruncationKindLabel_UnknownFallsBackToRawKind(t *testing.T) {
	t.Parallel()
	r := newWarningResolved()
	// Unknown kinds fall back to the raw string so a future caller that
	// adds a kind without wiring up the i18n catalog still produces a
	// recognizable (if untranslated) label rather than the empty string.
	got := truncationKindLabel(r, truncationKind("brand_new_kind"))
	if got != "brand_new_kind" {
		t.Errorf("got %q, want raw fallback", got)
	}
}
