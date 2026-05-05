package cmd_test

// Direct unit tests for Deps.Resolve focused on the config-load failure
// branch. Existing flow tests in root_flow_test.go cover the happy path
// (flag > config > env > default precedence). This file pins the
// fallback contract: even when LoadConfig errors, the returned Resolved
// must still carry a usable Locale derived from flag + env, so the
// caller can localize the resulting error message itself.

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

// newCmdWithLangFlag builds a minimal cobra.Command that declares the
// `--lang` persistent flag the way the root command does. Resolve looks up
// `--lang` via flagString(c, "lang"), which depends on the flag being
// declared on the command (or any ancestor); a bare *cobra.Command without
// the flag returns "" for any lookup.
func newCmdWithLangFlag(args []string) *cobra.Command {
	c := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
	c.PersistentFlags().String("lang", "", "")
	c.SetArgs(args)
	return c
}

func TestDeps_Resolve_ConfigLoadFailure_FallsBackToEnvLocale(t *testing.T) {
	t.Parallel()

	loadErr := errors.New("config: malformed yaml")
	d := cmd.Deps{
		Env: func(k string) string {
			if k == "LANG" {
				return "ja_JP.UTF-8"
			}
			return ""
		},
		LoadConfig: func() (config.AppConfig, error) {
			return config.AppConfig{}, loadErr
		},
	}
	c := newCmdWithLangFlag(nil)
	if err := c.ParseFlags(nil); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}

	r, err := d.Resolve(c)
	if !errors.Is(err, loadErr) {
		t.Fatalf("expected loadErr, got %v", err)
	}
	if r.Locale != i18n.LocaleJA {
		t.Errorf("expected LANG-derived ja, got %q", r.Locale)
	}
}

func TestDeps_Resolve_ConfigLoadFailure_FlagOverridesEnv(t *testing.T) {
	t.Parallel()

	// On the failure path the flag must still take precedence over env so
	// users can force English output even with a broken config.
	loadErr := errors.New("config: malformed yaml")
	d := cmd.Deps{
		Env: func(k string) string {
			if k == "LANG" {
				return "ja_JP.UTF-8"
			}
			return ""
		},
		LoadConfig: func() (config.AppConfig, error) {
			return config.AppConfig{}, loadErr
		},
	}
	c := newCmdWithLangFlag([]string{"--lang", "en"})
	if err := c.ParseFlags([]string{"--lang", "en"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}

	r, err := d.Resolve(c)
	if !errors.Is(err, loadErr) {
		t.Fatalf("expected loadErr, got %v", err)
	}
	if r.Locale != i18n.LocaleEN {
		t.Errorf("expected --lang=en to win, got %q", r.Locale)
	}
}

func TestDeps_Resolve_ConfigLoadFailure_DefaultsToEN(t *testing.T) {
	t.Parallel()

	// No flag, no LANG, broken config: the locale defaults to EN so the
	// caller can still emit a readable error message in the default
	// language.
	loadErr := errors.New("config: malformed yaml")
	d := cmd.Deps{
		Env:        func(string) string { return "" },
		LoadConfig: func() (config.AppConfig, error) { return config.AppConfig{}, loadErr },
	}
	c := newCmdWithLangFlag(nil)
	if err := c.ParseFlags(nil); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}

	r, err := d.Resolve(c)
	if !errors.Is(err, loadErr) {
		t.Fatalf("expected loadErr, got %v", err)
	}
	if r.Locale != i18n.LocaleEN {
		t.Errorf("expected default EN, got %q", r.Locale)
	}
}

func TestDeps_Resolve_ConfigLoadFailure_EmptyConfigOnFailure(t *testing.T) {
	t.Parallel()

	// On failure, Config must be the zero value (an absent/broken config
	// must not leak a partially-populated struct that downstream code
	// might treat as authoritative).
	loadErr := errors.New("nope")
	d := cmd.Deps{
		Env: func(string) string { return "" },
		LoadConfig: func() (config.AppConfig, error) {
			// Even if LoadConfig returned a non-zero struct alongside the
			// error, Resolve must discard it.
			return config.AppConfig{Locale: i18n.LocaleJA}, loadErr
		},
	}
	c := newCmdWithLangFlag(nil)
	if err := c.ParseFlags(nil); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}

	r, err := d.Resolve(c)
	if !errors.Is(err, loadErr) {
		t.Fatalf("expected loadErr, got %v", err)
	}
	if r.Config.Locale != "" {
		t.Errorf("Config must be zero on failure, got Locale=%q", r.Config.Locale)
	}
}
