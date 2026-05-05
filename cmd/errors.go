package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/period"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

// ErrSilent is the parent sentinel for any localized error already printed to
// stderr; main inspects it to suppress cobra's duplicate error output.
//
// To restore the TS implementation's exit-code distinction (arg validation =>
// 2, runtime failure => 1), every silent error wraps either [ErrSilentArgs]
// or [ErrSilentRuntime]. errors.Is(err, ErrSilent) keeps working for callers
// that don't care about the distinction (sentinel chain via fmt.Errorf %w).
var (
	// ErrSilent is the parent sentinel; main maps it to a non-zero exit
	// without printing the wrapped error again. Tests and callers that don't
	// care about the args/runtime distinction can keep using this.
	ErrSilent = errors.New("silent error")

	// ErrSilentArgs marks failures caused by invalid user-supplied input
	// (flags, positional args, config file syntax). main maps it to exit 2,
	// matching the TS implementation. errors.Is(err, ErrSilent) is still
	// true for chained checks.
	ErrSilentArgs = fmt.Errorf("%w: arg validation", ErrSilent)

	// ErrSilentRuntime marks failures from API calls, missing resources, or
	// other non-arg runtime conditions. main maps it to exit 1.
	ErrSilentRuntime = fmt.Errorf("%w: runtime", ErrSilent)
)

// classifyArgError reports whether err is a domain error type that originates
// from arg / flag / config validation. These map to ErrSilentArgs (exit 2).
//
// Anything else (auth failure, GraphQL error, "not found" responses from the
// API, etc.) is treated as runtime and maps to ErrSilentRuntime (exit 1).
func classifyArgError(err error) bool {
	var (
		scopeErr   *scope.ScopeError
		repoErr    *repo.RepoError
		projectErr *project.ProjectError
		periodErr  *period.PeriodError
		configErr  *config.ConfigError
		argErr     *cmdArgError
	)
	switch {
	case errors.As(err, &scopeErr),
		errors.As(err, &repoErr),
		errors.As(err, &projectErr),
		errors.As(err, &periodErr),
		errors.As(err, &configErr),
		errors.As(err, &argErr):
		return true
	}
	return false
}

// cmdArgError is a localized error owned by a cmd-layer flag/arg validator
// that doesn't have a domain package counterpart (e.g. `--since` parsing on
// the standup command). It satisfies [i18n.Localized] via the embedded
// Payload and is recognized by [classifyArgError] so it maps to
// [ErrSilentArgs] / exit 2.
type cmdArgError struct{ i18n.Payload }

// Error renders the en-locale message so log/wrap paths still surface a
// human-readable string when bypassing localizedError.
func (e *cmdArgError) Error() string { return e.Localize(i18n.LocaleEN) }

func newArgError(key string, args ...any) *cmdArgError {
	return &cmdArgError{Payload: i18n.NewPayload(key, args...)}
}

// cmdRuntimeError is a localized error owned by a cmd-layer runtime check
// that doesn't have a domain package counterpart (e.g. an unexpectedly empty
// viewer login on standup --mine). Maps to [ErrSilentRuntime] / exit 1.
type cmdRuntimeError struct{ i18n.Payload }

// Error renders the en-locale message so log/wrap paths still surface a
// human-readable string when bypassing localizedError.
func (e *cmdRuntimeError) Error() string { return e.Localize(i18n.LocaleEN) }

func newRuntimeError(key string, args ...any) *cmdRuntimeError {
	return &cmdRuntimeError{Payload: i18n.NewPayload(key, args...)}
}

// wrapTransport wraps a transport-layer error returned by a paginator or
// mutation. When err carries a [*api.GraphQLError] (rate limit, partial
// errors[] response, permission failure, etc.), a localized hint is printed
// to stderr before the wrap is returned — operators get an actionable
// message even though the wrap chain still preserves the raw error for
// errors.Is callers and cobra's default `Error: <op>: <cause>` rendering.
//
// The hint path is a deliberate non-breaking addition to the existing
// `fmt.Errorf("<op>: %w", err)` pattern (audit follow-up C-5 / refs #285):
// callers replace the fmt.Errorf line with a call to this helper, and the
// wrapped error keeps the same shape so `errors.Is(err, transportErr)` and
// the `<op>:` substring contract pinned by cmd_transport_error_test.go are
// preserved verbatim. Errors that are not [*api.GraphQLError] (HTTP 5xx,
// network failure, context cancellation) fall through silently — they are
// already surfaced by cobra's default error path with the wrapped cause.
func wrapTransport(stderr io.Writer, locale i18n.Locale, op string, err error) error {
	if err == nil {
		return nil
	}
	var gqlErr *api.GraphQLError
	if errors.As(err, &gqlErr) && gqlErr != nil {
		hintTransportPartial(stderr, locale, op, gqlErr)
	}
	return fmt.Errorf("%s: %w", op, err)
}

// hintTransportPartial prints a one-line localized warning describing a
// partial GraphQL response (errors[] populated). The summary surfaces the
// op label so the user can correlate the warning with the wrapped error
// cobra prints next, and includes the first error item's message verbatim
// (no further translation — the upstream message is already English) so
// rate-limit / permission causes are immediately visible without spelunking
// into a verbose stack trace.
func hintTransportPartial(stderr io.Writer, locale i18n.Locale, op string, gqlErr *api.GraphQLError) {
	if stderr == nil || gqlErr == nil || len(gqlErr.Errors) == 0 {
		return
	}
	first := gqlErr.Errors[0].Message
	fmt.Fprintln(stderr, i18n.T(locale, "warn.transport.partial", "op", op, "message", first))
}
