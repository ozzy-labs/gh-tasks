package cmd

import (
	"testing"
	"time"
)

// TestIsItemDone pins the case-fold contract of isItemDone (cmd/standup.go:379)
// shared by runStandup and runReviewProject: EqualFold(status, "done") with
// the empty-string short circuit. The filter relies on this for completed-item
// classification, so a "DONE" / "Done" / "done" project template must all
// flow through to the same bucket and a future == swap must be caught.
//
// Reuses makeStatusItem from triage_internal_test.go — both helpers live in
// package cmd so a single helper covers all status-driven matchers.
func TestIsItemDone(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status string
		want   bool
	}{
		{status: "", want: false},
		{status: "done", want: true},
		{status: "Done", want: true},
		{status: "DONE", want: true},
		{status: "DoNe", want: true},
		{status: "Todo", want: false},
		{status: "In Progress", want: false},
		{status: "Triage", want: false},
		// Substring of "done" must not match — EqualFold compares full strings.
		{status: "do", want: false},
		{status: "done!", want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.status, func(t *testing.T) {
			t.Parallel()
			got := isItemDone(makeStatusItem(tc.status))
			if got != tc.want {
				t.Errorf("isItemDone(status=%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// TestTimeAtOrAfter pins the contract of timeAtOrAfter (cmd/standup.go:334),
// the per-item time filter behind runStandup's `--since` and the implicit
// 24-hour window. Three branches matter as regression guards: empty input,
// malformed RFC3339, and the boundary where the parsed time equals the
// threshold (which must be inclusive — exactly equal counts as "at or after").
func TestTimeAtOrAfter(t *testing.T) {
	t.Parallel()

	threshold := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	t.Run("empty-iso-returns-false", func(t *testing.T) {
		t.Parallel()
		if timeAtOrAfter("", threshold) {
			t.Error("empty iso must return false")
		}
	})

	t.Run("malformed-iso-returns-false", func(t *testing.T) {
		t.Parallel()
		if timeAtOrAfter("not-a-date", threshold) {
			t.Error("malformed iso must return false")
		}
	})

	t.Run("equal-to-threshold-returns-true", func(t *testing.T) {
		t.Parallel()
		// Inclusive boundary: items whose timestamp lands exactly on the
		// threshold must count as "at or after".
		if !timeAtOrAfter("2026-05-04T12:00:00Z", threshold) {
			t.Error("iso == threshold must return true (inclusive)")
		}
	})

	t.Run("after-threshold-returns-true", func(t *testing.T) {
		t.Parallel()
		if !timeAtOrAfter("2026-05-04T12:00:01Z", threshold) {
			t.Error("iso after threshold must return true")
		}
	})

	t.Run("before-threshold-returns-false", func(t *testing.T) {
		t.Parallel()
		if timeAtOrAfter("2026-05-04T11:59:59Z", threshold) {
			t.Error("iso before threshold must return false")
		}
	})
}
