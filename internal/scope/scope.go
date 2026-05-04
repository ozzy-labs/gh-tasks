// Package scope provides the 3-scope abstraction (repo / org / user).
package scope

import (
	"errors"
	"strings"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

// Scope is the working scope.
type Scope string

// Recognized scope values.
const (
	Repo Scope = "repo"
	Org  Scope = "org"
	User Scope = "user"
)

// Valid lists the accepted scopes in canonical order.
var Valid = []Scope{Repo, Org, User}

// ScopeError is returned when a --scope flag is missing a value or has an
// unrecognized value.
type ScopeError struct{ i18n.Payload }

// Error satisfies the error interface.
func (e *ScopeError) Error() string { return e.Key }

// AsScopeError unwraps err into a ScopeError, returning nil + false when err
// is not one.
func AsScopeError(err error) (*ScopeError, bool) {
	var se *ScopeError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}

func newError(key string, args ...any) *ScopeError {
	return &ScopeError{Payload: i18n.NewPayload(key, args...)}
}

// DetectOptions configures Detect.
type DetectOptions struct {
	Argv         []string
	HasGitRemote func() bool
	DefaultScope Scope
}

// Detect resolves the working scope.
//
// Order:
//  1. --scope repo|org|user flag
//  2. git remote origin exists → repo
//  3. config DefaultScope
//  4. fallback → user
func Detect(opts DetectOptions) (Scope, error) {
	s, present, err := ParseFlag(opts.Argv)
	if err != nil {
		return "", err
	}
	if present {
		return s, nil
	}
	if opts.HasGitRemote != nil && opts.HasGitRemote() {
		return Repo, nil
	}
	if opts.DefaultScope != "" {
		return opts.DefaultScope, nil
	}
	return User, nil
}

// ParseFlag scans argv for --scope=<value> or --scope <value>.
//
// The returned bool reports whether the --scope flag was *present* in argv,
// independent of whether its value parsed successfully. Always check err
// first; a non-nil err means the flag was present but malformed (missing
// value or unknown scope).
//
// Result combinations:
//   - ("", false, nil): flag absent.
//   - (scope, true, nil): flag present with a valid value.
//   - ("", true, err): flag present but malformed.
func ParseFlag(argv []string) (Scope, bool, error) {
	for i, arg := range argv {
		if strings.HasPrefix(arg, "--scope=") {
			s, err := assertScope(strings.TrimPrefix(arg, "--scope="))
			if err != nil {
				return "", true, err
			}
			return s, true, nil
		}
		if arg == "--scope" {
			if i+1 >= len(argv) {
				return "", true, newError("error.scope.flagMissingValue")
			}
			s, err := assertScope(argv[i+1])
			if err != nil {
				return "", true, err
			}
			return s, true, nil
		}
	}
	return "", false, nil
}

func assertScope(v string) (Scope, error) {
	for _, s := range Valid {
		if string(s) == v {
			return s, nil
		}
	}
	return "", newError("error.scope.invalid", "value", v, "valid", joinPipe(Valid))
}

func joinPipe(vals []Scope) string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = string(v)
	}
	return strings.Join(out, " | ")
}
