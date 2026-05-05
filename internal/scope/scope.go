// Package scope provides the 3-scope abstraction (repo / org / user).
package scope

import (
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
//
// Use errors.As(err, &target) to test for this type:
//
//	var se *scope.ScopeError
//	if errors.As(err, &se) { ... }
type ScopeError struct{ i18n.Payload }

// Error satisfies the error interface.
func (e *ScopeError) Error() string { return e.Key }

func newError(key string, args ...any) *ScopeError {
	return &ScopeError{Payload: i18n.NewPayload(key, args...)}
}

// DetectOptions configures Detect.
//
// Flag carries the raw string value of the --scope flag as parsed by cobra
// (empty string means the flag was not supplied). The previous Argv-based
// scanner was retired in favour of cobra-authoritative flag handling.
type DetectOptions struct {
	Flag         string
	HasGitRemote func() bool
	DefaultScope Scope
}

// Detect resolves the working scope.
//
// Order:
//  1. Flag (cobra-parsed --scope value)
//  2. git remote origin exists → repo
//  3. config DefaultScope
//  4. fallback → user
func Detect(opts DetectOptions) (Scope, error) {
	if opts.Flag != "" {
		return assertScope(opts.Flag)
	}
	if opts.HasGitRemote != nil && opts.HasGitRemote() {
		return Repo, nil
	}
	if opts.DefaultScope != "" {
		return opts.DefaultScope, nil
	}
	return User, nil
}

func assertScope(v string) (Scope, error) {
	for _, s := range Valid {
		if string(s) == v {
			return s, nil
		}
	}
	return "", newError("error.scope.invalid", "value", v, "valid", i18n.JoinPipe(Valid))
}
