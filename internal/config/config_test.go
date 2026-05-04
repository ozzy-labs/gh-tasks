package config_test

import (
	"io/fs"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

func TestLoad_AbsentReturnsZero(t *testing.T) {
	t.Parallel()
	got, err := config.Load(config.LoadOptions{
		Path:     "/nonexistent",
		ReadFile: func(string) ([]byte, error) { return nil, fs.ErrNotExist },
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != (config.AppConfig{}) {
		t.Errorf("got %v, want zero", got)
	}
}

func TestLoad_AllFieldsParsed(t *testing.T) {
	t.Parallel()
	body := []byte(`
lang = "ja"
default_scope = "org"
org_project = "ozzy-labs/3"
user_project = "ozzy-3/1"
`)
	got, err := config.Load(config.LoadOptions{
		Path:     "/test/gh-tasks.toml",
		ReadFile: func(string) ([]byte, error) { return body, nil },
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := config.AppConfig{
		Locale:       i18n.LocaleJA,
		DefaultScope: scope.Org,
		OrgProject:   project.Ref{Owner: "ozzy-labs", Number: 3},
		UserProject:  project.Ref{Owner: "ozzy-3", Number: 1},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Load mismatch (-want +got):\n%s", diff)
	}
}

func TestLoad_InvalidLang(t *testing.T) {
	t.Parallel()
	body := []byte(`lang = "fr"`)
	_, err := config.Load(config.LoadOptions{
		Path:     "/test.toml",
		ReadFile: func(string) ([]byte, error) { return body, nil },
	})
	if err == nil {
		t.Fatal("want error")
	}
	ce, ok := config.AsConfigError(err)
	if !ok {
		t.Fatalf("not a ConfigError: %T", err)
	}
	if ce.I18nKey() != "error.config.invalidLang" {
		t.Errorf("key=%q", ce.I18nKey())
	}
}

func TestLoad_InvalidProjectRef(t *testing.T) {
	t.Parallel()
	body := []byte(`org_project = "no-slash"`)
	_, err := config.Load(config.LoadOptions{
		Path:     "/test.toml",
		ReadFile: func(string) ([]byte, error) { return body, nil },
	})
	ce, ok := config.AsConfigError(err)
	if !ok {
		t.Fatalf("got %T", err)
	}
	if ce.I18nKey() != "error.config.invalidProjectRef" {
		t.Errorf("key=%q", ce.I18nKey())
	}
}

func TestLoad_MalformedTOML(t *testing.T) {
	t.Parallel()
	body := []byte(`not = valid toml = double-equals`)
	_, err := config.Load(config.LoadOptions{
		Path:     "/test.toml",
		ReadFile: func(string) ([]byte, error) { return body, nil },
	})
	ce, ok := config.AsConfigError(err)
	if !ok {
		t.Fatalf("got %T", err)
	}
	if ce.I18nKey() != "error.config.tomlParseFailed" {
		t.Errorf("key=%q", ce.I18nKey())
	}
}

func TestResolvePath_PrefersXDG(t *testing.T) {
	t.Parallel()
	got := config.ResolvePath(func(k string) string {
		if k == "XDG_CONFIG_HOME" {
			return "/xdg"
		}
		return ""
	})
	want := "/xdg/ozzylabs/gh-tasks.toml"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestLoad_DefaultScopeInvalid(t *testing.T) {
	t.Parallel()
	body := []byte(`default_scope = "team"`)
	_, err := config.Load(config.LoadOptions{
		Path:     "/test.toml",
		ReadFile: func(string) ([]byte, error) { return body, nil },
	})
	ce, ok := config.AsConfigError(err)
	if !ok {
		t.Fatalf("got %T", err)
	}
	if ce.I18nKey() != "error.config.invalidDefaultScope" {
		t.Errorf("key=%q", ce.I18nKey())
	}
}

func TestLoad_LangNonString(t *testing.T) {
	t.Parallel()
	body := []byte(`lang = 5`)
	_, err := config.Load(config.LoadOptions{
		Path:     "/test.toml",
		ReadFile: func(string) ([]byte, error) { return body, nil },
	})
	ce, ok := config.AsConfigError(err)
	if !ok {
		t.Fatalf("got %T", err)
	}
	if ce.I18nKey() != "error.config.invalidLang" {
		t.Errorf("key=%q want error.config.invalidLang", ce.I18nKey())
	}
}

func TestLoad_OrgProjectNonString(t *testing.T) {
	t.Parallel()
	body := []byte(`org_project = 5`)
	_, err := config.Load(config.LoadOptions{
		Path:     "/test.toml",
		ReadFile: func(string) ([]byte, error) { return body, nil },
	})
	ce, ok := config.AsConfigError(err)
	if !ok {
		t.Fatalf("got %T", err)
	}
	if ce.I18nKey() != "error.config.invalidProjectRef" {
		t.Errorf("key=%q want error.config.invalidProjectRef", ce.I18nKey())
	}
}

func TestLoad_DefaultScopeNonString(t *testing.T) {
	t.Parallel()
	body := []byte(`default_scope = 5`)
	_, err := config.Load(config.LoadOptions{
		Path:     "/test.toml",
		ReadFile: func(string) ([]byte, error) { return body, nil },
	})
	ce, ok := config.AsConfigError(err)
	if !ok {
		t.Fatalf("got %T", err)
	}
	if ce.I18nKey() != "error.config.invalidDefaultScope" {
		t.Errorf("key=%q want error.config.invalidDefaultScope", ce.I18nKey())
	}
}

func TestLoad_UnknownKeyIgnored(t *testing.T) {
	t.Parallel()
	body := []byte(`
unknown_key = "hello"
lang = "ja"
default_scope = "org"
`)
	got, err := config.Load(config.LoadOptions{
		Path:     "/test.toml",
		ReadFile: func(string) ([]byte, error) { return body, nil },
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := config.AppConfig{
		Locale:       i18n.LocaleJA,
		DefaultScope: scope.Org,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Load mismatch (-want +got):\n%s", diff)
	}
}

func TestResolvePath_EmptyXDGTreatedAsUnset(t *testing.T) {
	// Cannot use t.Parallel() because t.Setenv mutates process env.
	t.Setenv("HOME", "/home/test")
	// On linux, os.UserHomeDir returns $HOME, so we get a deterministic path.
	got := config.ResolvePath(func(k string) string {
		if k == "XDG_CONFIG_HOME" {
			return ""
		}
		return ""
	})
	want := "/home/test/.config/ozzylabs/gh-tasks.toml"
	if got != want {
		t.Errorf("got %q want %q (empty XDG_CONFIG_HOME should fall back to HOME)", got, want)
	}
}
