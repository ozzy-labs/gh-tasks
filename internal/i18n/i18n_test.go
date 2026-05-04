package i18n_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

func TestResolveLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		argv   []string
		env    map[string]string
		config i18n.LocaleConfig
		want   i18n.Locale
	}{
		{name: "default-en", want: i18n.LocaleEN},
		{name: "lang-flag-equals-ja", argv: []string{"--lang=ja"}, want: i18n.LocaleJA},
		{name: "lang-flag-space-ja", argv: []string{"--lang", "ja"}, want: i18n.LocaleJA},
		{name: "lang-flag-equals-en", argv: []string{"--lang=en"}, want: i18n.LocaleEN},
		{name: "lang-flag-unknown-falls-through-to-env", argv: []string{"--lang=fr"}, env: map[string]string{"LANG": "ja_JP.UTF-8"}, want: i18n.LocaleJA},
		{name: "config-ja", config: i18n.LocaleConfig{Lang: i18n.LocaleJA}, want: i18n.LocaleJA},
		{name: "config-overridden-by-flag", argv: []string{"--lang=en"}, config: i18n.LocaleConfig{Lang: i18n.LocaleJA}, want: i18n.LocaleEN},
		{name: "lc-all-ja", env: map[string]string{"LC_ALL": "ja_JP.UTF-8"}, want: i18n.LocaleJA},
		{name: "lc-all-en", env: map[string]string{"LC_ALL": "en_US.UTF-8"}, want: i18n.LocaleEN},
		{name: "lc-all-outranks-lang", env: map[string]string{"LC_ALL": "en_US.UTF-8", "LANG": "ja_JP.UTF-8"}, want: i18n.LocaleEN},
		{name: "lang-ja", env: map[string]string{"LANG": "ja_JP.UTF-8"}, want: i18n.LocaleJA},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := i18n.ResolveLocale(tc.argv, lookupFn(tc.env), tc.config)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ResolveLocale mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestT(t *testing.T) {
	t.Parallel()

	t.Run("known-key-en", func(t *testing.T) {
		t.Parallel()
		got := i18n.T(i18n.LocaleEN, "list.empty")
		if got == "list.empty" {
			t.Fatalf("got raw key, expected translation")
		}
	})

	t.Run("known-key-ja", func(t *testing.T) {
		t.Parallel()
		got := i18n.T(i18n.LocaleJA, "list.empty")
		if got == "list.empty" {
			t.Fatalf("got raw key, expected translation")
		}
	})

	t.Run("missing-key-returns-key", func(t *testing.T) {
		t.Parallel()
		got := i18n.T(i18n.LocaleEN, "no.such.key")
		if got != "no.such.key" {
			t.Errorf("want fallback to key, got %q", got)
		}
	})

	t.Run("placeholder-substitution", func(t *testing.T) {
		t.Parallel()
		got := i18n.T(i18n.LocaleEN, "error.repo.invalidIdentifier", "value", "broken-input")
		if !contains(got, "broken-input") {
			t.Errorf("expected substituted value in %q", got)
		}
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{"en": true, "ja": true, "fr": false, "": false, "EN": false}
	for in, ok := range cases {
		_, gotOK := i18n.Validate(in)
		if gotOK != ok {
			t.Errorf("Validate(%q) ok=%v, want %v", in, gotOK, ok)
		}
	}
}

func lookupFn(env map[string]string) i18n.EnvLookup {
	return func(k string) string { return env[k] }
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
