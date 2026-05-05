package i18n_test

import (
	"strings"
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
		{name: "lang-flag-no-value-falls-through-to-env", argv: []string{"--lang"}, env: map[string]string{"LANG": "ja_JP.UTF-8"}, want: i18n.LocaleJA},
		{name: "lang-flag-separate-unknown-falls-through-to-env", argv: []string{"--lang", "xx"}, env: map[string]string{"LANG": "ja_JP.UTF-8"}, want: i18n.LocaleJA},
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
		if !strings.Contains(got, "broken-input") {
			t.Errorf("expected substituted value in %q", got)
		}
	})

	t.Run("no-args-leaves-placeholder-intact", func(t *testing.T) {
		t.Parallel()
		// When called without args, T must not run substitution at all, so
		// any `{name}` placeholders in the catalog string must survive
		// verbatim in the output.
		got := i18n.T(i18n.LocaleEN, "error.repo.invalidIdentifier")
		if !strings.Contains(got, "{value}") {
			t.Errorf("expected literal {value} placeholder in %q", got)
		}
	})
}

type stubLangProvider struct{ loc i18n.Locale }

func (s stubLangProvider) Lang() i18n.Locale { return s.loc }

func TestResolveLocaleFor(t *testing.T) {
	t.Parallel()

	t.Run("provider-empty-falls-through-to-env", func(t *testing.T) {
		t.Parallel()
		got := i18n.ResolveLocaleFor(nil, lookupFn(map[string]string{"LANG": "ja_JP.UTF-8"}), stubLangProvider{})
		if got != i18n.LocaleJA {
			t.Errorf("got %q, want ja", got)
		}
	})

	t.Run("provider-set-wins-over-env", func(t *testing.T) {
		t.Parallel()
		got := i18n.ResolveLocaleFor(nil, lookupFn(map[string]string{"LANG": "ja_JP.UTF-8"}), stubLangProvider{loc: i18n.LocaleEN})
		if got != i18n.LocaleEN {
			t.Errorf("got %q, want en", got)
		}
	})

	t.Run("flag-wins-over-provider", func(t *testing.T) {
		t.Parallel()
		got := i18n.ResolveLocaleFor([]string{"--lang=en"}, nil, stubLangProvider{loc: i18n.LocaleJA})
		if got != i18n.LocaleEN {
			t.Errorf("got %q, want en", got)
		}
	})

	t.Run("nil-provider-falls-through", func(t *testing.T) {
		t.Parallel()
		got := i18n.ResolveLocaleFor(nil, lookupFn(map[string]string{"LANG": "ja_JP.UTF-8"}), nil)
		if got != i18n.LocaleJA {
			t.Errorf("got %q, want ja", got)
		}
	})
}

func TestSubstitute(t *testing.T) {
	t.Parallel()

	t.Run("nested-placeholder-not-re-expanded", func(t *testing.T) {
		t.Parallel()
		// If the implementation iterated the map and called strings.ReplaceAll
		// per entry, expanding `outer` first would leave `{inner}` to be
		// substituted on the next pass — non-deterministic across map order.
		// The single-pass regex implementation must leave the literal `{inner}`
		// from the value untouched.
		msg := "{outer}"
		args := map[string]any{
			"outer": "value-{inner}",
			"inner": "REPLACED",
		}
		got := i18n.Substitute(msg, args)
		want := "value-{inner}"
		if got != want {
			t.Errorf("Substitute deterministic mismatch: got %q, want %q", got, want)
		}
	})

	t.Run("multiple-placeholders-stable-across-runs", func(t *testing.T) {
		t.Parallel()
		msg := "{a}-{b}-{c}-{a}"
		args := map[string]any{"a": "1", "b": "2", "c": "3"}
		// Run many iterations; map iteration order randomization should not
		// affect the result.
		want := "1-2-3-1"
		for i := 0; i < 100; i++ {
			if got := i18n.Substitute(msg, args); got != want {
				t.Fatalf("iteration %d: got %q, want %q", i, got, want)
			}
		}
	})

	t.Run("unknown-placeholder-left-intact", func(t *testing.T) {
		t.Parallel()
		got := i18n.Substitute("hello {name}", map[string]any{"other": "x"})
		if got != "hello {name}" {
			t.Errorf("unknown placeholder should be untouched, got %q", got)
		}
	})

	t.Run("empty-args-returns-msg-unchanged", func(t *testing.T) {
		t.Parallel()
		got := i18n.Substitute("hello {name}", nil)
		if got != "hello {name}" {
			t.Errorf("empty args should return msg unchanged, got %q", got)
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

func TestKeys(t *testing.T) {
	t.Parallel()

	en := i18n.Keys(i18n.LocaleEN)
	ja := i18n.Keys(i18n.LocaleJA)

	if len(en) == 0 {
		t.Fatal("Keys(en) is empty; embedded en.json failed to load")
	}
	if len(ja) == 0 {
		t.Fatal("Keys(ja) is empty; embedded ja.json failed to load")
	}

	// Mutating the returned map must not corrupt the catalog. Re-call and
	// verify length is unchanged.
	beforeLen := len(en)
	en["__sentinel__"] = struct{}{}
	again := i18n.Keys(i18n.LocaleEN)
	if len(again) != beforeLen {
		t.Errorf("Keys returned a shared map: mutating the result changed catalog (len before=%d after=%d)",
			beforeLen, len(again))
	}

	// Unknown locale must return a non-nil empty map (callers iterate it).
	out := i18n.Keys(i18n.Locale("zz"))
	if out == nil {
		t.Error("Keys(unknown) returned nil; want empty non-nil map")
	}
	if len(out) != 0 {
		t.Errorf("Keys(unknown) len=%d, want 0", len(out))
	}
}

func lookupFn(env map[string]string) i18n.EnvLookup {
	return func(k string) string { return env[k] }
}

// TestJoinPipe pins the contract of [i18n.JoinPipe], the generic helper
// used for "list of valid values" rendering in error messages
// (e.g. error.locale.invalid renders Locales as "ja | en"). Callers
// pass any named string slice (Locale, scope.Scope, period.Period) so
// the test exercises both raw string and named-string-type inputs.
func TestJoinPipe(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if got := i18n.JoinPipe([]string{}); got != "" {
			t.Errorf("empty: got %q, want \"\"", got)
		}
	})

	t.Run("single", func(t *testing.T) {
		t.Parallel()
		if got := i18n.JoinPipe([]string{"only"}); got != "only" {
			t.Errorf("single: got %q, want %q", got, "only")
		}
	})

	t.Run("multiple", func(t *testing.T) {
		t.Parallel()
		if got := i18n.JoinPipe([]string{"a", "b", "c"}); got != "a | b | c" {
			t.Errorf("multiple: got %q, want %q", got, "a | b | c")
		}
	})

	t.Run("named-string-type", func(t *testing.T) {
		t.Parallel()
		// Locale is `~string`, exercising the generic constraint.
		if got := i18n.JoinPipe(i18n.Locales); got != "ja | en" {
			t.Errorf("Locales: got %q, want %q", got, "ja | en")
		}
	})
}

// TestT_EmptyLocale pins the empty-Locale fallback in [i18n.lookup]: when a
// caller passes Locale("") (e.g. an unresolved zero value before
// ResolveLocale runs), T must still emit a usable English message rather
// than the raw key, because callers print the result directly to stderr.
func TestT_EmptyLocale(t *testing.T) {
	t.Parallel()

	got := i18n.T(i18n.Locale(""), "list.empty")
	if got == "list.empty" {
		t.Fatalf("empty locale fell through to raw key; expected en fallback")
	}
	enGot := i18n.T(i18n.LocaleEN, "list.empty")
	if got != enGot {
		t.Errorf("empty-locale rendering should match en fallback: got %q, en %q", got, enGot)
	}
}
