// Package period normalizes daily / weekly / sprint windows for plan, review,
// and standup. Periods are anchored at local-midnight in the resolved IANA
// timezone and returned as absolute UTC instants.
package period

import (
	"errors"
	"fmt"
	"strings"
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
type PeriodError struct{ i18n.Payload }

// Error satisfies the error interface.
func (e *PeriodError) Error() string { return e.Key }

// AsPeriodError unwraps err into a PeriodError.
func AsPeriodError(err error) (*PeriodError, bool) {
	var pe *PeriodError
	if errors.As(err, &pe) {
		return pe, true
	}
	return nil, false
}

func newError(key string, args ...any) *PeriodError {
	return &PeriodError{Payload: i18n.NewPayload(key, args...)}
}

// ParseFlag scans argv for --period=<value> or --period <value>.
func ParseFlag(argv []string) (Period, bool, error) {
	for i, arg := range argv {
		if strings.HasPrefix(arg, "--period=") {
			p, err := toPeriod(strings.TrimPrefix(arg, "--period="))
			return p, err == nil, err
		}
		if arg == "--period" {
			if i+1 >= len(argv) {
				return "", false, newError("error.period.flagMissingValue")
			}
			p, err := toPeriod(argv[i+1])
			return p, err == nil, err
		}
	}
	return "", false, nil
}

func toPeriod(v string) (Period, error) {
	for _, p := range Periods {
		if string(p) == v {
			return p, nil
		}
	}
	return "", newError("error.period.invalid", "value", v, "valid", joinPipe(Periods))
}

// LocationLookup resolves an IANA timezone name to a *time.Location. Tests
// override this via [Range]Of{}-style options if needed; production callers
// pass nil to use [time.LoadLocation].
type LocationLookup func(name string) (*time.Location, error)

// Of returns the local-midnight-anchored Range for the given period at now.
//
// tz selection (in priority order):
//  1. tz arg, if non-empty and a valid IANA name
//  2. TZ env (looked up via getenv)
//  3. system local time (time.Local)
//  4. UTC fallback
//
// The fallback to UTC matches the TS implementation: an explicit but invalid
// tz drops to UTC (so the caller is not silently substituted with system tz).
func Of(period Period, now time.Time, tz string, getenv func(string) string) Range {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	loc := resolveLocation(tz, getenv)
	startOfToday := localMidnight(now, loc)
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
	// Unrecognized Period values are a programmer error: ParseFlag /
	// toPeriod gate user-supplied input. Returning a zero Range would let
	// downstream filters silently treat the window as empty.
	panic(fmt.Sprintf("period: unrecognized Period value %q", string(period)))
}

// SuggestMilestoneTitle returns a human label for a period anchored at now.
func SuggestMilestoneTitle(period Period, now time.Time, tz string, getenv func(string) string) string {
	r := Of(period, now, tz, getenv)
	iso := FormatLocalISODate(r.Start, tz, getenv)
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
func FormatLocalISODate(d time.Time, tz string, getenv func(string) string) string {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	loc := resolveLocation(tz, getenv)
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
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func daysSinceMonday(w time.Weekday) int {
	// Sunday=0, Monday=1, ..., Saturday=6.
	return (int(w) + 6) % 7
}

func joinPipe(periods []Period) string {
	out := make([]string, len(periods))
	for i, p := range periods {
		out[i] = string(p)
	}
	return strings.Join(out, " | ")
}

// String renders a range for debug/log output.
func (r Range) String() string {
	return fmt.Sprintf("[%s, %s)", r.Start.Format(time.RFC3339), r.End.Format(time.RFC3339))
}
