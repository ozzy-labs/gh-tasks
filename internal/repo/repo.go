// Package repo resolves a `<owner>/<name>` identifier from --repo flag or git
// remote `origin`.
package repo

import (
	"context"
	"os"
	"os/exec"
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

// RepoError is returned by [Resolve] / [ParseOwnerName] / [ExtractFromRemote]
// when an input cannot be parsed.
//
// Use errors.As(err, &target) to test for this type:
//
//	var re *repo.RepoError
//	if errors.As(err, &re) { ... }
type RepoError struct{ i18n.Payload }

// Error satisfies the error interface.
func (e *RepoError) Error() string { return e.Key }

func newError(key string, args ...any) *RepoError {
	return &RepoError{Payload: i18n.NewPayload(key, args...)}
}

// ResolveOptions configures Resolve.
//
// Flag carries the raw string value of the --repo flag as parsed by cobra
// (empty means the flag was not supplied; an explicitly empty value such as
// `--repo=` also resolves to "" and falls through to the git remote). The
// previous Argv-based scanner was retired in favour of cobra-authoritative
// flag handling.
type ResolveOptions struct {
	// Context bounds the default git remote lookup. When nil,
	// context.Background() is used. When GetRemoteURL is set, callers are
	// responsible for honouring any context themselves.
	Context      context.Context
	Flag         string
	GetRemoteURL func() (string, bool) // returns ("", false) when no remote
}

// Resolve picks a repo identifier from --repo flag or git remote origin.
//
// An empty Flag value falls through to git remote resolution rather than
// surfacing a confusing invalidIdentifier error. This matches the prior TS
// behavior where an unset --repo was treated as falsy in resolveRepo.
func Resolve(opts ResolveOptions) (Ident, error) {
	if opts.Flag != "" {
		return ParseOwnerName(opts.Flag)
	}
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	getRemote := opts.GetRemoteURL
	if getRemote == nil {
		getRemote = func() (string, bool) { return defaultGetRemoteURL(ctx) }
	}
	url, ok := getRemote()
	if !ok {
		return Ident{}, newError("error.repo.notResolved")
	}
	v, err := ExtractFromRemote(url)
	if err != nil {
		return Ident{}, err
	}
	return ParseOwnerName(v)
}

// ParseOwnerName parses an `<owner>/<name>` string.
func ParseOwnerName(value string) (Ident, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Ident{}, newError("error.repo.invalidIdentifier", "value", value)
	}
	return Ident{Owner: parts[0], Name: parts[1]}, nil
}

// ExtractFromRemote pulls owner/name out of a git remote URL.
//
// Supported forms:
//   - SSH (scp-like): git@github.com:owner/name.git
//   - SSH (URI):      ssh://git@github.com/owner/name.git
//   - HTTPS:          https://github.com/owner/name.git
//   - Trailing .git is optional; a single trailing slash is accepted.
//
// URLs whose path has more or fewer than two segments are rejected. A
// previous regex-only implementation matched the last two segments
// greedily, which silently misextracted "extra/path" from URLs like
// "https://github.com/owner/name/extra/path".
func ExtractFromRemote(url string) (string, error) {
	url = strings.TrimSpace(url)
	path := extractRemotePath(url)
	if path == "" {
		return "", newError("error.repo.cannotExtractFromRemote", "url", url)
	}
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", newError("error.repo.cannotExtractFromRemote", "url", url)
	}
	return parts[0] + "/" + parts[1], nil
}

// extractRemotePath returns the path portion of a git remote URL after
// the host/auth prefix. Returns "" when no recognized host/path
// boundary is present.
func extractRemotePath(url string) string {
	if i := strings.Index(url, "://"); i >= 0 {
		rest := url[i+3:]
		j := strings.Index(rest, "/")
		if j < 0 {
			return ""
		}
		return rest[j+1:]
	}
	// scp-like SSH form: user@host:path. Require the colon to be
	// preceded by "@" so we don't misinterpret a bare "owner:name"
	// string as a remote URL.
	if i := strings.Index(url, ":"); i >= 0 && strings.Contains(url[:i], "@") {
		return url[i+1:]
	}
	return ""
}

// defaultGetRemoteURL invokes `git remote get-url origin` bounded by ctx so
// a stuck or hanging git process cannot block the CLI indefinitely. Callers
// that already supply ResolveOptions.GetRemoteURL bypass this entirely.
//
// The child git invocation is isolated from inherited GIT_* repository-location
// variables (see [cleanGitEnv]) so the lookup honours the process cwd rather
// than whatever repo a parent git hook is operating on. Without this isolation
// a `gh tasks` call invoked from a pre-commit hook would inherit the hook's
// GIT_DIR and resolve the *hook's* origin instead of the cwd's.
func defaultGetRemoteURL(ctx context.Context) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Env = cleanGitEnv(os.Environ())
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

// cleanGitEnv returns env with GIT_* repository-location variables removed so
// a child `git` invocation falls back to cwd-based discovery.
//
// The list mirrors `chdirToTempOrSkip` in repo_test.go and is sourced from
// `git help environment`'s "repository locations" section. We remove the
// entries entirely (rather than setting them to empty strings) because git
// distinguishes an unset GIT_DIR from an empty one — the latter is treated
// as a literal empty path and aborts.
//
// Other GIT_* variables (GIT_AUTHOR_NAME, GIT_TERMINAL_PROMPT, etc.) are
// intentionally preserved; only the location-overriding ones are stripped.
func cleanGitEnv(env []string) []string {
	drop := map[string]struct{}{
		"GIT_DIR":              {},
		"GIT_WORK_TREE":        {},
		"GIT_COMMON_DIR":       {},
		"GIT_INDEX_FILE":       {},
		"GIT_OBJECT_DIRECTORY": {},
		"GIT_NAMESPACE":        {},
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		eq := strings.IndexByte(kv, '=')
		if eq > 0 {
			if _, skip := drop[kv[:eq]]; skip {
				continue
			}
		}
		out = append(out, kv)
	}
	return out
}
