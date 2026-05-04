// Package period normalizes daily / weekly / sprint windows for plan, review,
// and standup. Periods are anchored at local-midnight in the resolved IANA
// timezone and returned as absolute UTC instants.
package period

import (
	"fmt"
	"time"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

// Period is the supported aggregation window.
type Period string

// Recognized period values.
const (
	Daily  Period = "daily"
	Weekly Period = "weekly"
	Sprint Period = "sprint"
)

// Periods lists the accepted period values in canonical order.
var Periods = []Period{Daily, Weekly, Sprint}

// Range is an inclusive-start, exclusive-end window expressed as UTC instants
// that correspond to local midnights in the resolved timezone.
type Range struct {
	Start time.Time
	End   time.Time
}

// PeriodError is returned when a --period flag is missing a value or has an
// unrecognized value.
//
// Use errors.As(err, &target) to test for this type:
//
//	var pe *period.PeriodError
//	if errors.As(err, &pe) { ... }
type PeriodError struct{ i18n.Payload }

// Error satisfies the error interface.
func (e *PeriodError) Error() string { return e.Key }

func newError(key string, args ...any) *PeriodError {
	return &PeriodError{Payload: i18n.NewPayload(key, args...)}
}

// Parse validates value as a recognized [Period]. Empty values return the
// zero Period without error so callers can apply their own default.
//
// Callers read --period via cobra's c.Flags().GetString("period") and pass
// the result here directly; the previous argv-based ParseFlag scanner was
// retired together with the rest of the legacy argv parsers.
func Parse(value string) (Period, error) {
	if value == "" {
		return "", nil
	}
	for _, p := range Periods {
		if string(p) == value {
			return p, nil
		}
	}
	return "", newError("error.period.invalid", "value", value, "valid", i18n.JoinPipe(Periods))
}

// Options collects the contextual inputs shared by [Of],
// [SuggestMilestoneTitle], and [FormatLocalISODate].
//
// All fields are optional:
//   - Tz: an IANA timezone name. Empty falls back to TZ env, then system
//     local time, then UTC. An explicit but invalid name drops to UTC
//     (so callers are not silently substituted with system tz).
//   - Getenv: env lookup function. Defaults to a no-op (returns "").
//   - Now: clock reference. Used only by [Of] and [SuggestMilestoneTitle].
//     Defaults to [time.Now] when zero.
type Options struct {
	Tz     string
	Getenv func(string) string
	Now    time.Time
}

func (o Options) getenv() func(string) string {
	if o.Getenv != nil {
		return o.Getenv
	}
	return func(string) string { return "" }
}

func (o Options) now() time.Time {
	if o.Now.IsZero() {
		return time.Now()
	}
	return o.Now
}

// Of returns the local-midnight-anchored Range for the given period at
// opts.Now.
func Of(period Period, opts Options) Range {
	loc := resolveLocation(opts.Tz, opts.getenv())
	startOfToday := localMidnight(opts.now(), loc)
	switch period {
	case Daily:
		return Range{Start: startOfToday, End: startOfToday.AddDate(0, 0, 1)}
	case Weekly:
		dsm := daysSinceMonday(startOfToday.In(loc).Weekday())
		monday := startOfToday.AddDate(0, 0, -dsm)
		return Range{Start: monday, End: monday.AddDate(0, 0, 7)}
	case Sprint:
		return Range{Start: startOfToday, End: startOfToday.AddDate(0, 0, 14)}
	}
	// Unrecognized Period values are a programmer error: Parse gates
	// user-supplied input. Returning a zero Range would let downstream
	// filters silently treat the window as empty.
	panic(fmt.Sprintf("period: unrecognized Period value %q", string(period)))
}

// SuggestMilestoneTitle returns a human label for a period anchored at
// opts.Now.
func SuggestMilestoneTitle(period Period, opts Options) string {
	r := Of(period, opts)
	iso := FormatLocalISODate(r.Start, opts)
	switch period {
	case Daily:
		return "Daily " + iso
	case Weekly:
		return "Week of " + iso
	case Sprint:
		return "Sprint " + iso
	}
	panic(fmt.Sprintf("period: unrecognized Period value %q", string(period)))
}

// FormatLocalISODate renders d as YYYY-MM-DD in the resolved timezone.
// opts.Now is ignored.
func FormatLocalISODate(d time.Time, opts Options) string {
	loc := resolveLocation(opts.Tz, opts.getenv())
	return d.In(loc).Format("2006-01-02")
}

func resolveLocation(tz string, getenv func(string) string) *time.Location {
	if tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
		// Explicit invalid tz → UTC fallback (matches TS behavior).
		return time.UTC
	}
	if env := getenv("TZ"); env != "" {
		if loc, err := time.LoadLocation(env); err == nil {
			return loc
		}
	}
	if loc, err := time.LoadLocation("Local"); err == nil {
		return loc
	}
	return time.UTC
}

func localMidnight(t time.Time, loc *time.Location) time.Time {
	tl := t.In(loc)
	return time.Date(tl.Year(), tl.Month(), tl.Day(), 0, 0, 0, 0, loc)
}

func daysSinceMonday(w time.Weekday) int {
	// Sunday=0, Monday=1, ..., Saturday=6.
	return (int(w) + 6) % 7
}

// String renders a range for debug/log output.
func (r Range) String() string {
	return fmt.Sprintf("[%s, %s)", r.Start.Format(time.RFC3339), r.End.Format(time.RFC3339))
}
