package period_test

import (
	"testing"
	"time"

	"github.com/ozzy-labs/gh-tasks/internal/period"
)

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("tzdata not available for %s: %v", name, err)
	}
	return loc
}

func TestParseFlag(t *testing.T) {
	t.Parallel()

	cases := []struct {
		argv []string
		want period.Period
		ok   bool
		err  bool
	}{
		{[]string{"--period=daily"}, period.Daily, true, false},
		{[]string{"--period", "weekly"}, period.Weekly, true, false},
		{[]string{"--period=sprint"}, period.Sprint, true, false},
		{[]string{"--period=monthly"}, "", false, true},
		{[]string{"--period"}, "", false, true},
		{[]string{}, "", false, false},
		{[]string{"--scope=org"}, "", false, false},
	}
	for _, tc := range cases {
		t.Run(joinArgv(tc.argv), func(t *testing.T) {
			t.Parallel()
			got, ok, err := period.ParseFlag(tc.argv)
			if tc.err {
				if err == nil {
					t.Fatalf("want error, got %v ok=%v", got, ok)
				}
				return
			}
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tc.want || ok != tc.ok {
				t.Errorf("got (%v,%v) want (%v,%v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestOf_DailyJST(t *testing.T) {
	t.Parallel()
	jst := mustLoadLocation(t, "Asia/Tokyo")

	// Wed 2026-04-29 10:00 JST (= 2026-04-29 01:00 UTC)
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, jst)
	r := period.Of(period.Daily, now, "Asia/Tokyo", func(string) string { return "" })

	wantStart := time.Date(2026, 4, 29, 0, 0, 0, 0, jst)
	wantEnd := time.Date(2026, 4, 30, 0, 0, 0, 0, jst)
	if !r.Start.Equal(wantStart) || !r.End.Equal(wantEnd) {
		t.Errorf("got %s, want [%s, %s)", r, wantStart, wantEnd)
	}
}

func TestOf_WeeklyAnchorsOnMonday_JST(t *testing.T) {
	t.Parallel()
	jst := mustLoadLocation(t, "Asia/Tokyo")

	// Wed 2026-04-29 (any time)
	now := time.Date(2026, 4, 29, 15, 30, 0, 0, jst)
	r := period.Of(period.Weekly, now, "Asia/Tokyo", nil)

	// Monday this week is 2026-04-27.
	wantStart := time.Date(2026, 4, 27, 0, 0, 0, 0, jst)
	wantEnd := time.Date(2026, 5, 4, 0, 0, 0, 0, jst)
	if !r.Start.Equal(wantStart) || !r.End.Equal(wantEnd) {
		t.Errorf("got %s, want [%s, %s)", r, wantStart, wantEnd)
	}
}

func TestOf_WeeklySundayBacksUpOneWeek(t *testing.T) {
	t.Parallel()
	jst := mustLoadLocation(t, "Asia/Tokyo")

	// Sun 2026-05-03 10:00 JST → week starts Mon 2026-04-27.
	now := time.Date(2026, 5, 3, 10, 0, 0, 0, jst)
	r := period.Of(period.Weekly, now, "Asia/Tokyo", nil)

	wantStart := time.Date(2026, 4, 27, 0, 0, 0, 0, jst)
	if !r.Start.Equal(wantStart) {
		t.Errorf("got start %s, want %s", r.Start, wantStart)
	}
}

func TestOf_Sprint(t *testing.T) {
	t.Parallel()
	jst := mustLoadLocation(t, "Asia/Tokyo")

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, jst)
	r := period.Of(period.Sprint, now, "Asia/Tokyo", nil)

	wantStart := time.Date(2026, 4, 29, 0, 0, 0, 0, jst)
	wantEnd := time.Date(2026, 5, 13, 0, 0, 0, 0, jst)
	if !r.Start.Equal(wantStart) || !r.End.Equal(wantEnd) {
		t.Errorf("got %s, want [%s, %s)", r, wantStart, wantEnd)
	}
}

func TestOf_UTCFallbackWhenTZInvalid(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 29, 1, 0, 0, 0, time.UTC)
	r := period.Of(period.Daily, now, "Not/A_TZ", nil)
	wantStart := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)
	if !r.Start.Equal(wantStart) {
		t.Errorf("got %s, want %s", r.Start, wantStart)
	}
}

func TestSuggestMilestoneTitle(t *testing.T) {
	t.Parallel()
	jst := mustLoadLocation(t, "Asia/Tokyo")
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, jst)

	cases := map[period.Period]string{
		period.Daily:  "Daily 2026-04-29",
		period.Weekly: "Week of 2026-04-27",
		period.Sprint: "Sprint 2026-04-29",
	}
	for p, want := range cases {
		t.Run(string(p), func(t *testing.T) {
			t.Parallel()
			got := period.SuggestMilestoneTitle(p, now, "Asia/Tokyo", nil)
			if got != want {
				t.Errorf("got %q want %q", got, want)
			}
		})
	}
}

func joinArgv(argv []string) string {
	out := "empty"
	for i, s := range argv {
		if i == 0 {
			out = s
		} else {
			out += " " + s
		}
	}
	return out
}
