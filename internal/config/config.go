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
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

// AppConfig is the parsed config file.
type AppConfig struct {
	Lang         i18n.Locale
	DefaultScope scope.Scope
	OrgProject   project.Ref
	UserProject  project.Ref
}

// ConfigError is returned when a config file is present but malformed or
// holds invalid values, so the user sees a specific message instead of a
// silent fallback.
type ConfigError struct{ i18n.Payload }

// Error satisfies the error interface.
func (e *ConfigError) Error() string { return e.Key }

// AsConfigError unwraps err into a ConfigError.
func AsConfigError(err error) (*ConfigError, bool) {
	var ce *ConfigError
	if errors.As(err, &ce) {
		return ce, true
	}
	return nil, false
}

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

type rawConfig struct {
	Lang         *string `toml:"lang"`
	DefaultScope *string `toml:"default_scope"`
	OrgProject   *string `toml:"org_project"`
	UserProject  *string `toml:"user_project"`
}

func parse(raw []byte, path string) (AppConfig, error) {
	var rc rawConfig
	if err := toml.Unmarshal(raw, &rc); err != nil {
		return AppConfig{}, newError("error.config.tomlParseFailed", "path", path, "reason", err.Error())
	}
	out := AppConfig{}
	if rc.Lang != nil {
		loc, ok := i18n.Validate(*rc.Lang)
		if !ok {
			return AppConfig{}, newError(
				"error.config.invalidLang",
				"path", path,
				"value", *rc.Lang,
				"valid", strings.Join(localeNames(), " | "),
			)
		}
		out.Lang = loc
	}
	if rc.DefaultScope != nil {
		s, ok := validateScope(*rc.DefaultScope)
		if !ok {
			return AppConfig{}, newError(
				"error.config.invalidDefaultScope",
				"path", path,
				"value", *rc.DefaultScope,
				"valid", strings.Join(scopeNames(), " | "),
			)
		}
		out.DefaultScope = s
	}
	if rc.OrgProject != nil {
		ref, err := parseProjectKey(*rc.OrgProject, "org_project", path)
		if err != nil {
			return AppConfig{}, err
		}
		out.OrgProject = ref
	}
	if rc.UserProject != nil {
		ref, err := parseProjectKey(*rc.UserProject, "user_project", path)
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

func localeNames() []string {
	return []string{string(i18n.LocaleJA), string(i18n.LocaleEN)}
}

func scopeNames() []string {
	out := make([]string, len(scope.Valid))
	for i, s := range scope.Valid {
		out[i] = string(s)
	}
	return out
}

// String renders the config for debug logs.
func (c AppConfig) String() string {
	return fmt.Sprintf(
		"AppConfig{Lang:%q DefaultScope:%q OrgProject:%v UserProject:%v}",
		c.Lang, c.DefaultScope, c.OrgProject, c.UserProject,
	)
}
