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

func TestParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value string
		want  period.Period
		err   bool
	}{
		{"daily", period.Daily, false},
		{"weekly", period.Weekly, false},
		{"sprint", period.Sprint, false},
		{"", "", false},
		{"monthly", "", true},
		{"DAILY", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			t.Parallel()
			got, err := period.Parse(tc.value)
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
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestOf_DailyJST(t *testing.T) {
	t.Parallel()
	jst := mustLoadLocation(t, "Asia/Tokyo")

	// Wed 2026-04-29 10:00 JST (= 2026-04-29 01:00 UTC)
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, jst)
	r := period.Of(period.Daily, period.Options{
		Tz:     "Asia/Tokyo",
		Getenv: func(string) string { return "" },
		Now:    now,
	})

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
	r := period.Of(period.Weekly, period.Options{Tz: "Asia/Tokyo", Now: now})

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
	r := period.Of(period.Weekly, period.Options{Tz: "Asia/Tokyo", Now: now})

	wantStart := time.Date(2026, 4, 27, 0, 0, 0, 0, jst)
	if !r.Start.Equal(wantStart) {
		t.Errorf("got start %s, want %s", r.Start, wantStart)
	}
}

func TestOf_Sprint(t *testing.T) {
	t.Parallel()
	jst := mustLoadLocation(t, "Asia/Tokyo")

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, jst)
	r := period.Of(period.Sprint, period.Options{Tz: "Asia/Tokyo", Now: now})

	wantStart := time.Date(2026, 4, 29, 0, 0, 0, 0, jst)
	wantEnd := time.Date(2026, 5, 13, 0, 0, 0, 0, jst)
	if !r.Start.Equal(wantStart) || !r.End.Equal(wantEnd) {
		t.Errorf("got %s, want [%s, %s)", r, wantStart, wantEnd)
	}
}

func TestOf_UTCFallbackWhenTZInvalid(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 29, 1, 0, 0, 0, time.UTC)
	r := period.Of(period.Daily, period.Options{Tz: "Not/A_TZ", Now: now})
	wantStart := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)
	if !r.Start.Equal(wantStart) {
		t.Errorf("got %s, want %s", r.Start, wantStart)
	}
}

// When tz is empty and getenv is nil, Of must not panic and must still
// produce a sane window: it should fall back to system Local (or UTC if
// Local cannot be loaded). We only assert the call succeeds and yields
// a 24h window for daily.
func TestOf_NilGetenvAndEmptyTzDoesNotPanic(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	r := period.Of(period.Daily, period.Options{Now: now})
	if got := r.End.Sub(r.Start); got != 24*time.Hour {
		t.Errorf("daily span = %s, want 24h", got)
	}
}

// When tz is empty but TZ env points at a valid IANA name, getenv should
// be honored.
func TestOf_GetenvTZIsHonored(t *testing.T) {
	t.Parallel()
	jst := mustLoadLocation(t, "Asia/Tokyo")
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, jst)
	r := period.Of(period.Daily, period.Options{
		Getenv: func(key string) string {
			if key == "TZ" {
				return "Asia/Tokyo"
			}
			return ""
		},
		Now: now,
	})
	wantStart := time.Date(2026, 4, 29, 0, 0, 0, 0, jst)
	if !r.Start.Equal(wantStart) {
		t.Errorf("got %s, want %s", r.Start, wantStart)
	}
}

// 2026-03-08 is the spring-forward Sunday in America/New_York: at
// 02:00 local the clock jumps to 03:00. The daily window should still
// be anchored at local midnight on both ends; the absolute UTC span
// covers only 23h that day.
func TestOf_DailyAcrossSpringForwardNY(t *testing.T) {
	t.Parallel()
	ny := mustLoadLocation(t, "America/New_York")
	// Sun 2026-03-08 10:00 NY local (after spring-forward).
	now := time.Date(2026, 3, 8, 10, 0, 0, 0, ny)
	r := period.Of(period.Daily, period.Options{Tz: "America/New_York", Now: now})

	wantStart := time.Date(2026, 3, 8, 0, 0, 0, 0, ny)
	wantEnd := time.Date(2026, 3, 9, 0, 0, 0, 0, ny)
	if !r.Start.Equal(wantStart) || !r.End.Equal(wantEnd) {
		t.Errorf("got %s, want [%s, %s)", r, wantStart, wantEnd)
	}
	// Spring-forward day is 23 hours wall-clock-to-wall-clock.
	if got := r.End.Sub(r.Start); got != 23*time.Hour {
		t.Errorf("daily span across spring-forward = %s, want 23h", got)
	}
}

// Weekly anchored on Monday 2026-03-02 must include Sunday 2026-03-08
// (the spring-forward day) and run through to Monday 2026-03-09.
func TestOf_WeeklyAcrossSpringForwardNY(t *testing.T) {
	t.Parallel()
	ny := mustLoadLocation(t, "America/New_York")
	// Sun 2026-03-08 10:00 NY local — last day of the week.
	now := time.Date(2026, 3, 8, 10, 0, 0, 0, ny)
	r := period.Of(period.Weekly, period.Options{Tz: "America/New_York", Now: now})

	wantStart := time.Date(2026, 3, 2, 0, 0, 0, 0, ny)
	wantEnd := time.Date(2026, 3, 9, 0, 0, 0, 0, ny)
	if !r.Start.Equal(wantStart) || !r.End.Equal(wantEnd) {
		t.Errorf("got %s, want [%s, %s)", r, wantStart, wantEnd)
	}
	// 7 calendar days containing one DST transition is 7*24 - 1 = 167h.
	if got := r.End.Sub(r.Start); got != 167*time.Hour {
		t.Errorf("weekly span across spring-forward = %s, want 167h", got)
	}
}

// daysSinceMonday=0 case: when "now" is Monday in JST, the weekly
// window must start that same Monday at 00:00, not back up by 7 days.
func TestOf_WeeklyOnMonday_DaysSinceMondayZero(t *testing.T) {
	t.Parallel()
	jst := mustLoadLocation(t, "Asia/Tokyo")
	// Mon 2026-04-27 09:30 JST.
	now := time.Date(2026, 4, 27, 9, 30, 0, 0, jst)
	r := period.Of(period.Weekly, period.Options{Tz: "Asia/Tokyo", Now: now})

	wantStart := time.Date(2026, 4, 27, 0, 0, 0, 0, jst)
	wantEnd := time.Date(2026, 5, 4, 0, 0, 0, 0, jst)
	if !r.Start.Equal(wantStart) || !r.End.Equal(wantEnd) {
		t.Errorf("got %s, want [%s, %s)", r, wantStart, wantEnd)
	}
}

func TestOf_PanicsOnUnrecognizedPeriod(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unrecognized period")
		}
	}()
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	_ = period.Of(period.Period("monthly"), period.Options{Tz: "UTC", Now: now})
}

func TestSuggestMilestoneTitle_PanicsOnUnrecognizedPeriod(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unrecognized period")
		}
	}()
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	_ = period.SuggestMilestoneTitle(period.Period("monthly"), period.Options{Tz: "UTC", Now: now})
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
			got := period.SuggestMilestoneTitle(p, period.Options{Tz: "Asia/Tokyo", Now: now})
			if got != want {
				t.Errorf("got %q want %q", got, want)
			}
		})
	}
}
