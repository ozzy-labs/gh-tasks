// Package project parses Projects v2 references and resolves them per scope.
package project

import (
	"strconv"
	"strings"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

// Ref is a Projects v2 reference: owner login + project number.
//
// `<owner>/<number>` mirrors GitHub's URL convention
// (`/users/<owner>/projects/<number>` and `/orgs/<owner>/projects/<number>`).
type Ref struct {
	Owner  string
	Number int
}

// String renders the canonical owner/number form.
func (r Ref) String() string { return r.Owner + "/" + strconv.Itoa(r.Number) }

// IsZero reports whether r is the zero value.
func (r Ref) IsZero() bool { return r == Ref{} }

// ProjectError is returned when a project flag or config value is malformed,
// or when a scope is asked for a project it cannot have (e.g. repo).
//
// Use errors.As(err, &target) to test for this type:
//
//	var pe *project.ProjectError
//	if errors.As(err, &pe) { ... }
type ProjectError struct{ i18n.Payload }

// Error satisfies the error interface.
func (e *ProjectError) Error() string { return e.Key }

func newError(key string, args ...any) *ProjectError {
	return &ProjectError{Payload: i18n.NewPayload(key, args...)}
}

// ParseIdentifier parses an `<owner>/<number>` string. Returns (Ref{}, false)
// for any malformed input — callers decide whether that's an error.
func ParseIdentifier(value string) (Ref, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Ref{}, false
	}
	slash := strings.Index(value, "/")
	if slash <= 0 || slash == len(value)-1 {
		return Ref{}, false
	}
	owner := value[:slash]
	numberStr := value[slash+1:]
	n, err := strconv.Atoi(numberStr)
	if err != nil || n <= 0 || strconv.Itoa(n) != numberStr {
		return Ref{}, false
	}
	return Ref{Owner: owner, Number: n}, true
}

// ResolveOptions configures Resolve.
//
// Flag carries the raw string value of the --project flag as parsed by cobra
// (empty means the flag was not supplied). The previous Argv-based scanner
// was retired in favour of cobra-authoritative flag handling.
type ResolveOptions struct {
	Scope       scope.Scope
	Flag        string
	OrgProject  Ref // from config
	UserProject Ref // from config
}

// Resolve picks a Projects v2 reference for the given scope.
//
// Order:
//  1. Flag (cobra-parsed --project value)
//  2. config (OrgProject when scope=org, UserProject when scope=user)
//  3. ProjectError (callers should report the missing setting)
//
// scope=repo always errors — Projects v2 is not used in repo scope.
func Resolve(opts ResolveOptions) (Ref, error) {
	if opts.Scope == scope.Repo {
		return Ref{}, newError("error.project.repoScope")
	}
	if opts.Flag != "" {
		ref, ok := ParseIdentifier(opts.Flag)
		if !ok {
			return Ref{}, newError("error.project.invalidIdentifier", "value", opts.Flag)
		}
		return ref, nil
	}
	switch opts.Scope {
	case scope.Org:
		if !opts.OrgProject.IsZero() {
			return opts.OrgProject, nil
		}
		return Ref{}, newError("error.project.notSpecified", "scope", string(opts.Scope), "configKey", "org_project")
	case scope.User:
		if !opts.UserProject.IsZero() {
			return opts.UserProject, nil
		}
		return Ref{}, newError("error.project.notSpecified", "scope", string(opts.Scope), "configKey", "user_project")
	}
	return Ref{}, newError("error.project.notSpecified", "scope", string(opts.Scope), "configKey", "user_project")
}
