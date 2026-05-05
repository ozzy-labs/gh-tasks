package cmd

import (
	"testing"
	"time"

	"github.com/ozzy-labs/gh-tasks/internal/period"
)

// TestWithinPeriodRange pins the boundary semantics of the timestamp filter
// used by `runReviewRepo` / `runReviewProject` to decide whether a closed
// issue / merged PR / project-item update falls into the active period.
//
// The contract is "inclusive Start, exclusive End" matching period.Range's
// documented semantics. Every boundary case is asserted independently so a
// regression that flips one bound (e.g. End → inclusive) is caught here
// rather than indirectly through cmd-flow snapshot diffs.
func TestWithinPeriodRange(t *testing.T) {
	t.Parallel()

	rng := period.Range{
		Start: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC),
	}

	cases := []struct {
		name string
		iso  string
		want bool
	}{
		{
			name: "empty-string-is-out-of-range",
			// The closedAt / mergedAt fields are *string in the GraphQL
			// response — runReviewRepo dereferences them to "" when nil.
			// "" must be filtered out (as if the issue was never closed)
			// rather than parsed as the zero time.
			iso:  "",
			want: false,
		},
		{
			name: "malformed-iso-is-out-of-range",
			// A non-RFC3339 string (e.g. broken upstream data, an old
			// `2024-01-01` date-only stamp) must NOT panic and must NOT be
			// treated as a successful 0001-01-01 timestamp — fail-soft to
			// false.
			iso:  "2026-05-03",
			want: false,
		},
		{
			name: "before-range-start-is-out-of-range",
			iso:  "2026-04-30T23:59:59Z",
			want: false,
		},
		{
			name: "exactly-at-range-start-is-in-range",
			// Inclusive lower bound: closedAt == rng.Start counts.
			iso:  "2026-05-01T00:00:00Z",
			want: true,
		},
		{
			name: "mid-range-is-in-range",
			iso:  "2026-05-04T12:34:56Z",
			want: true,
		},
		{
			name: "just-before-range-end-is-in-range",
			iso:  "2026-05-07T23:59:59Z",
			want: true,
		},
		{
			name: "exactly-at-range-end-is-out-of-range",
			// Exclusive upper bound: closedAt == rng.End is OUT.
			// Pinning this prevents accidental "issue closed at midnight on
			// Sunday" rolling into the next week's review.
			iso:  "2026-05-08T00:00:00Z",
			want: false,
		},
		{
			name: "after-range-end-is-out-of-range",
			iso:  "2026-05-08T00:00:01Z",
			want: false,
		},
		{
			name: "rfc3339-with-offset-normalised-to-utc",
			// 2026-05-04T21:34:56+09:00 == 2026-05-04T12:34:56Z, which is
			// inside the range. Pin that the parser respects the offset
			// rather than stripping it.
			iso:  "2026-05-04T21:34:56+09:00",
			want: true,
		},
		{
			name: "rfc3339-fractional-seconds-accepted",
			// time.Parse(time.RFC3339, ...) accepts the fractional form,
			// which GitHub does emit. Pin compatibility.
			iso:  "2026-05-04T12:34:56.789Z",
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := withinPeriodRange(tc.iso, rng)
			if got != tc.want {
				t.Errorf("withinPeriodRange(%q, rng) = %v, want %v", tc.iso, got, tc.want)
			}
		})
	}
}
