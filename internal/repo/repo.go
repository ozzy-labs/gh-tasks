// Package repo resolves a `<owner>/<name>` identifier from --repo flag or git
// remote `origin`.
package repo

import (
	"errors"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

// Ident is a parsed `<owner>/<name>` identifier.
type Ident struct {
	Owner string
	Name  string
}

// String renders the canonical owner/name form.
func (i Ident) String() string { return i.Owner + "/" + i.Name }

// RepoError is returned by [Resolve] / [ParseFlag] / [ParseOwnerName] /
// [ExtractFromRemote] when an input cannot be parsed.
type RepoError struct{ i18n.Payload }

// Error satisfies the error interface.
func (e *RepoError) Error() string { return e.Key }

// AsRepoError unwraps err into a RepoError.
func AsRepoError(err error) (*RepoError, bool) {
	var re *RepoError
	if errors.As(err, &re) {
		return re, true
	}
	return nil, false
}

func newError(key string, args ...any) *RepoError {
	return &RepoError{Payload: i18n.NewPayload(key, args...)}
}

// ResolveOptions configures Resolve.
type ResolveOptions struct {
	Argv         []string
	GetRemoteURL func() (string, bool) // returns ("", false) when no remote
}

// Resolve picks a repo identifier from --repo flag or git remote origin.
//
// An empty --repo= value (e.g. from an unset shell variable expansion like
// `--repo=$VAR`) falls through to git remote resolution rather than surfacing
// a confusing invalidIdentifier error. This matches the prior TS behavior
// where parseRepoFlag returning "" was treated as falsy in resolveRepo.
func Resolve(opts ResolveOptions) (Ident, error) {
	if value, ok, err := ParseFlag(opts.Argv); err != nil {
		return Ident{}, err
	} else if ok && value != "" {
		return ParseOwnerName(value)
	}
	getRemote := opts.GetRemoteURL
	if getRemote == nil {
		getRemote = defaultGetRemoteURL
	}
	url, ok := getRemote()
	if !ok {
		return Ident{}, newError("error.repo.notResolved")
	}
	value, err := ExtractFromRemote(url)
	if err != nil {
		return Ident{}, err
	}
	return ParseOwnerName(value)
}

// ParseFlag scans argv for --repo=<value> or --repo <value>. Returns
// (value, true, nil) on success, ("", false, nil) when absent, and
// (_, _, err) when present but missing a value.
func ParseFlag(argv []string) (string, bool, error) {
	for i, arg := range argv {
		if strings.HasPrefix(arg, "--repo=") {
			return strings.TrimPrefix(arg, "--repo="), true, nil
		}
		if arg == "--repo" {
			if i+1 >= len(argv) {
				return "", false, newError("error.repo.flagMissingValue")
			}
			return argv[i+1], true, nil
		}
	}
	return "", false, nil
}

// ParseOwnerName parses an `<owner>/<name>` string.
func ParseOwnerName(value string) (Ident, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Ident{}, newError("error.repo.invalidIdentifier", "value", value)
	}
	return Ident{Owner: parts[0], Name: parts[1]}, nil
}

var remotePattern = regexp.MustCompile(`[:/]([^/:]+)/([^/]+?)(?:\.git)?/?$`)

// ExtractFromRemote pulls owner/name out of a git remote URL.
//
// Supported forms:
//   - SSH: git@github.com:owner/name.git
//   - HTTPS: https://github.com/owner/name.git
//   - Trailing .git is optional.
func ExtractFromRemote(url string) (string, error) {
	url = strings.TrimSpace(url)
	m := remotePattern.FindStringSubmatch(url)
	if m == nil {
		return "", newError("error.repo.cannotExtractFromRemote", "url", url)
	}
	return m[1] + "/" + m[2], nil
}

func defaultGetRemoteURL() (string, bool) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", false
	}
	return url, true
}
