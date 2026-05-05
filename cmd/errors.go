package cmd

import (
	"errors"
	"fmt"

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
