package cmd

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

// Deps groups dependencies that can be overridden in tests. Any field left
// zero falls back to the production default.
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
		return Resolved{}, err
	}
	loc := i18n.ResolveLocale(d.Argv, d.Env, i18n.LocaleConfig{Lang: cfg.Lang})
	return Resolved{Locale: loc, Config: cfg}, nil
}

// T renders a localized message in the resolved locale. Convenience wrapper
// so commands don't import internal/i18n directly for simple messages.
func (r Resolved) T(key string, args ...any) string {
	return i18n.T(r.Locale, key, args...)
}

func defaultHasGitRemote() bool {
	return exec.Command("git", "remote", "get-url", "origin").Run() == nil
}

func defaultGetRemoteURL() (string, bool) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", false
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", false
	}
	return url, true
}
