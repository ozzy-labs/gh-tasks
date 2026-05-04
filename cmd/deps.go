package cmd

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

// Deps groups dependencies that can be overridden in tests. DefaultDeps
// populates all fields; tests should use testDeps or set fields explicitly
// before passing Deps to RootWithDeps. Commands assume non-nil callbacks
// (Now, Env, HasGitRemote, GetRemoteURL, NewClients, LoadConfig) and will
// panic on a zero-value Deps.
type Deps struct {
	Stdout       io.Writer
	Stderr       io.Writer
	Stdin        io.Reader
	Now          func() time.Time
	Env          func(string) string
	HasGitRemote func() bool
	GetRemoteURL func() (string, bool)
	NewClients   func() (*github.Clients, error)
	LoadConfig   func() (config.AppConfig, error)
	// Argv supplies the raw process argv used for legacy flag parsing in
	// commands that haven't been fully migrated to cobra flags. cobra
	// normally consumes flags in args, so this is the unparsed argv passed
	// from main when callers want to honour --scope / --project / --repo /
	// --lang etc. via the existing TS-shaped parsers in internal/{scope,
	// repo, project, period, i18n}.
	Argv []string
}

// Resolved bundles the runtime-derived values: locale, config, etc.
type Resolved struct {
	Locale i18n.Locale
	Config config.AppConfig
}

// DefaultDeps returns the production Deps (writes to os.Std{out,err}, reads
// from os.Args / os.Environ, etc.).
func DefaultDeps() Deps {
	return Deps{
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		Stdin:        os.Stdin,
		Now:          time.Now,
		Env:          os.Getenv,
		HasGitRemote: defaultHasGitRemote,
		GetRemoteURL: defaultGetRemoteURL,
		NewClients: func() (*github.Clients, error) {
			return github.NewClients(github.ClientOptions{})
		},
		LoadConfig: func() (config.AppConfig, error) {
			return config.Load(config.LoadOptions{})
		},
		Argv: os.Args,
	}
}

// Resolve loads config and resolves the locale, applying Deps.Argv +
// Deps.Env. Returns a populated Resolved on success, or a *config.ConfigError
// when the config file is malformed.
func (d Deps) Resolve() (Resolved, error) {
	cfg, err := d.LoadConfig()
	if err != nil {
		// Even on config load failure, resolve locale from argv + env so the
		// returned error message can still be localized by the caller. The
		// nil provider tells ResolveLocaleFor to skip the config step.
		loc := i18n.ResolveLocaleFor(d.Argv, d.Env, nil)
		return Resolved{Locale: loc}, err
	}
	loc := i18n.ResolveLocaleFor(d.Argv, d.Env, cfg)
	return Resolved{Locale: loc, Config: cfg}, nil
}

// T renders a localized message in the resolved locale. Convenience wrapper
// so commands don't import internal/i18n directly for simple messages.
func (r Resolved) T(key string, args ...any) string {
	return i18n.T(r.Locale, key, args...)
}

// gitRemoteCache memoizes the result of `git remote get-url origin` for the
// lifetime of a single process. Tests inject Deps.HasGitRemote /
// Deps.GetRemoteURL directly, so the cache only applies to production runs
// where the working tree's remote does not change mid-invocation.
var (
	gitRemoteCacheOnce sync.Once
	gitRemoteCacheURL  string
	gitRemoteCacheOK   bool
)

func loadGitRemoteCache() {
	gitRemoteCacheOnce.Do(func() {
		out, err := exec.Command("git", "remote", "get-url", "origin").Output()
		if err != nil {
			return
		}
		url := strings.TrimSpace(string(out))
		if url == "" {
			return
		}
		gitRemoteCacheURL = url
		gitRemoteCacheOK = true
	})
}

func defaultHasGitRemote() bool {
	loadGitRemoteCache()
	return gitRemoteCacheOK
}

func defaultGetRemoteURL() (string, bool) {
	loadGitRemoteCache()
	return gitRemoteCacheURL, gitRemoteCacheOK
}
