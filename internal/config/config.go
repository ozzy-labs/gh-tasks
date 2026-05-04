// Package config loads gh-tasks runtime configuration from
// `${XDG_CONFIG_HOME}/ozzylabs/gh-tasks.toml`, falling back to
// `${HOME}/.config/ozzylabs/gh-tasks.toml`.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

// AppConfig is the parsed config file.
//
// AppConfig satisfies i18n.LangProvider via the Lang() method, so callers
// (cmd/deps.go) can pass the config directly to i18n.ResolveLocaleFor
// without wrapping it into a separate i18n.LocaleConfig shim.
type AppConfig struct {
	// Locale is the user's configured output locale (from `lang =` in
	// gh-tasks.toml). Empty when not set; callers should fall through to env
	// vars / the en default.
	Locale       i18n.Locale
	DefaultScope scope.Scope
	OrgProject   project.Ref
	UserProject  project.Ref
}

// Lang implements i18n.LangProvider.
func (c AppConfig) Lang() i18n.Locale { return c.Locale }

// ConfigError is returned when a config file is present but malformed or
// holds invalid values, so the user sees a specific message instead of a
// silent fallback.
//
// Use errors.As(err, &target) to test for this type:
//
//	var ce *config.ConfigError
//	if errors.As(err, &ce) { ... }
type ConfigError struct{ i18n.Payload }

// Error satisfies the error interface.
func (e *ConfigError) Error() string { return e.Key }

func newError(key string, args ...any) *ConfigError {
	return &ConfigError{Payload: i18n.NewPayload(key, args...)}
}

// LoadOptions configures Load.
type LoadOptions struct {
	Getenv   func(string) string
	Path     string                            // overrides the resolved path; tests use this
	ReadFile func(path string) ([]byte, error) // tests inject a fake reader
}

// ResolvePath resolves the config path per XDG Base Directory.
func ResolvePath(getenv func(string) string) string {
	if getenv == nil {
		getenv = os.Getenv
	}
	xdg := getenv("XDG_CONFIG_HOME")
	base := xdg
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "ozzylabs", "gh-tasks.toml")
}

// Load reads the config file. Returns a zero AppConfig when the file is
// absent (the file is optional). Returns ConfigError when the file exists
// but cannot be read or parsed.
func Load(opts LoadOptions) (AppConfig, error) {
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	path := opts.Path
	if path == "" {
		path = ResolvePath(getenv)
	}
	reader := opts.ReadFile
	if reader == nil {
		reader = os.ReadFile
	}

	raw, err := reader(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return AppConfig{}, nil
		}
		return AppConfig{}, newError("error.config.readFailed", "path", path, "reason", err.Error())
	}
	return parse(raw, path)
}

func parse(raw []byte, path string) (AppConfig, error) {
	var doc map[string]any
	if err := toml.Unmarshal(raw, &doc); err != nil {
		return AppConfig{}, newError("error.config.tomlParseFailed", "path", path, "reason", err.Error())
	}
	out := AppConfig{}
	if v, present := doc["lang"]; present {
		s, ok := v.(string)
		if !ok {
			return AppConfig{}, newError(
				"error.config.invalidLang",
				"path", path,
				"value", fmt.Sprint(v),
				"valid", i18n.JoinPipe(i18n.Locales),
			)
		}
		loc, ok := i18n.Validate(s)
		if !ok {
			return AppConfig{}, newError(
				"error.config.invalidLang",
				"path", path,
				"value", s,
				"valid", i18n.JoinPipe(i18n.Locales),
			)
		}
		out.Locale = loc
	}
	if v, present := doc["default_scope"]; present {
		s, ok := v.(string)
		if !ok {
			return AppConfig{}, newError(
				"error.config.invalidDefaultScope",
				"path", path,
				"value", fmt.Sprint(v),
				"valid", i18n.JoinPipe(scope.Valid),
			)
		}
		sc, ok := validateScope(s)
		if !ok {
			return AppConfig{}, newError(
				"error.config.invalidDefaultScope",
				"path", path,
				"value", s,
				"valid", i18n.JoinPipe(scope.Valid),
			)
		}
		out.DefaultScope = sc
	}
	if v, present := doc["org_project"]; present {
		s, ok := v.(string)
		if !ok {
			return AppConfig{}, newError(
				"error.config.invalidProjectRef",
				"key", "org_project", "path", path, "value", fmt.Sprint(v),
			)
		}
		ref, err := parseProjectKey(s, "org_project", path)
		if err != nil {
			return AppConfig{}, err
		}
		out.OrgProject = ref
	}
	if v, present := doc["user_project"]; present {
		s, ok := v.(string)
		if !ok {
			return AppConfig{}, newError(
				"error.config.invalidProjectRef",
				"key", "user_project", "path", path, "value", fmt.Sprint(v),
			)
		}
		ref, err := parseProjectKey(s, "user_project", path)
		if err != nil {
			return AppConfig{}, err
		}
		out.UserProject = ref
	}
	return out, nil
}

func parseProjectKey(value, key, path string) (project.Ref, error) {
	ref, ok := project.ParseIdentifier(value)
	if !ok {
		return project.Ref{}, newError(
			"error.config.invalidProjectRef",
			"key", key, "path", path, "value", value,
		)
	}
	return ref, nil
}

func validateScope(v string) (scope.Scope, bool) {
	for _, s := range scope.Valid {
		if string(s) == v {
			return s, true
		}
	}
	return "", false
}

// String renders the config for debug logs.
func (c AppConfig) String() string {
	return fmt.Sprintf(
		"AppConfig{Lang:%q DefaultScope:%q OrgProject:%v UserProject:%v}",
		c.Locale, c.DefaultScope, c.OrgProject, c.UserProject,
	)
}
