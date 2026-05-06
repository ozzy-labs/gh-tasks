package cmd

import (
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

// Deps groups dependencies that can be overridden in tests. DefaultDeps
// populates all fields; tests should use testDeps or set fields explicitly
// before passing Deps to RootWithDeps. Commands assume non-nil callbacks
// (Now, Env, HasGitRemote, GetRemoteURL, NewClients, LoadConfig) and will
// panic on a zero-value Deps.
//
// EmbeddedSkills carries the binary-bundled skill SSOT (populated by main).
// Commands that consume skills offline (install-skills, build-skills) read
// from it; tests inject a fstest.MapFS to keep the unit tests work-tree
// independent. May be nil — only commands that opt into embedded reads
// dereference it.
type Deps struct {
	Stdout         io.Writer
	Stderr         io.Writer
	Stdin          io.Reader
	Now            func() time.Time
	Env            func(string) string
	HasGitRemote   func() bool
	GetRemoteURL   func() (string, bool)
	NewClients     func() (*github.Clients, error)
	LoadConfig     func() (config.AppConfig, error)
	EmbeddedSkills fs.FS
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
	}
}

// Resolve loads config and resolves the locale from cobra's --lang flag and
// Deps.Env. Returns a populated Resolved on success, or a *config.ConfigError
// when the config file is malformed.
//
// The cobra command is consulted for the --lang persistent flag so cobra
// remains the authoritative source of all flag values (no parallel argv
// scanning).
func (d Deps) Resolve(c *cobra.Command) (Resolved, error) {
	lang := flagString(c, "lang")
	cfg, err := d.LoadConfig()
	if err != nil {
		// Even on config load failure, resolve locale from flag + env so the
		// returned error message can still be localized by the caller. The
		// nil provider tells ResolveLocaleFor to skip the config step.
		loc := i18n.ResolveLocaleFor(langArgv(lang), d.Env, nil)
		return Resolved{Locale: loc}, err
	}
	loc := i18n.ResolveLocaleFor(langArgv(lang), d.Env, cfg)
	return Resolved{Locale: loc, Config: cfg}, nil
}

// flagString reads a string flag from c (or any ancestor that defined it as
// a persistent flag). Returns "" when the command tree does not declare the
// flag (e.g. tests that build a bare *cobra.Command without a root).
func flagString(c *cobra.Command, name string) string {
	if c == nil {
		return ""
	}
	if f := c.Flag(name); f == nil {
		return ""
	}
	v, _ := c.Flags().GetString(name)
	return v
}

// langArgv adapts the cobra-parsed --lang value into the legacy argv shape
// consumed by i18n.ResolveLocaleFor. The latter still accepts argv-only so
// the existing precedence (flag > config > env) is preserved unchanged.
func langArgv(lang string) []string {
	if lang == "" {
		return nil
	}
	return []string{"--lang=" + lang}
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
