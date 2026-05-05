package i18n_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

func TestNewPayload(t *testing.T) {
	t.Parallel()

	t.Run("alternating-pairs-build-args-map", func(t *testing.T) {
		t.Parallel()
		p := i18n.NewPayload("error.repo.invalidIdentifier", "value", "broken-input")
		if p.Key != "error.repo.invalidIdentifier" {
			t.Errorf("Key = %q, want error.repo.invalidIdentifier", p.Key)
		}
		want := map[string]any{"value": "broken-input"}
		if diff := cmp.Diff(want, p.Args); diff != "" {
			t.Errorf("Args mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("odd-args-drops-trailing-key", func(t *testing.T) {
		t.Parallel()
		p := i18n.NewPayload("k", "value", "ok", "dangling")
		want := map[string]any{"value": "ok"}
		if diff := cmp.Diff(want, p.Args); diff != "" {
			t.Errorf("Args mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("non-string-key-dropped", func(t *testing.T) {
		t.Parallel()
		p := i18n.NewPayload("k", 42, "ignored", "value", "kept")
		want := map[string]any{"value": "kept"}
		if diff := cmp.Diff(want, p.Args); diff != "" {
			t.Errorf("Args mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("no-args-yields-empty-map", func(t *testing.T) {
		t.Parallel()
		p := i18n.NewPayload("help.usage")
		if p.Key != "help.usage" {
			t.Errorf("Key = %q, want help.usage", p.Key)
		}
		if len(p.Args) != 0 {
			t.Errorf("Args = %v, want empty", p.Args)
		}
	})
}

func TestPayload_Localize(t *testing.T) {
	t.Parallel()

	t.Run("substitutes-placeholders-in-en", func(t *testing.T) {
		t.Parallel()
		p := i18n.NewPayload("error.repo.invalidIdentifier", "value", "abc")
		got := p.Localize(i18n.LocaleEN)
		if !strings.Contains(got, "abc") {
			t.Errorf("expected substituted value in %q", got)
		}
		if strings.Contains(got, "{value}") {
			t.Errorf("placeholder must be expanded, got %q", got)
		}
	})

	t.Run("ja-locale-renders-translated-message", func(t *testing.T) {
		t.Parallel()
		p := i18n.NewPayload("error.repo.notFound", "owner", "o", "name", "r")
		gotEN := p.Localize(i18n.LocaleEN)
		gotJA := p.Localize(i18n.LocaleJA)
		if gotEN == gotJA {
			t.Errorf("expected different output for en/ja, both %q", gotEN)
		}
		if !strings.Contains(gotJA, "o/r") {
			t.Errorf("expected substituted owner/name in %q", gotJA)
		}
	})

	t.Run("unknown-locale-falls-back-to-en", func(t *testing.T) {
		t.Parallel()
		p := i18n.NewPayload("help.usage")
		want := p.Localize(i18n.LocaleEN)
		got := p.Localize(i18n.Locale("xx"))
		if got != want {
			t.Errorf("unknown locale = %q, want EN fallback %q", got, want)
		}
	})

	t.Run("unknown-key-returns-key-verbatim", func(t *testing.T) {
		t.Parallel()
		p := i18n.NewPayload("no.such.key.in.catalog")
		got := p.Localize(i18n.LocaleEN)
		if got != "no.such.key.in.catalog" {
			t.Errorf("got %q, want raw key fallback", got)
		}
	})
}

func TestPayload_Getters(t *testing.T) {
	t.Parallel()

	p := i18n.NewPayload("error.scope.invalid", "value", "foo", "valid", "repo,org,user")

	if p.I18nKey() != "error.scope.invalid" {
		t.Errorf("I18nKey = %q", p.I18nKey())
	}
	want := map[string]any{"value": "foo", "valid": "repo,org,user"}
	if diff := cmp.Diff(want, p.I18nArgs()); diff != "" {
		t.Errorf("I18nArgs mismatch (-want +got):\n%s", diff)
	}
}

func TestPayload_String(t *testing.T) {
	t.Parallel()

	p := i18n.NewPayload("k", "a", "1")
	got := p.String()
	if !strings.HasPrefix(got, "k: ") {
		t.Errorf("String() = %q, want prefix \"k: \"", got)
	}
	if !strings.Contains(got, "a:1") {
		t.Errorf("String() = %q, want to contain \"a:1\"", got)
	}
}

func TestFlat_RoundTripsThroughToArgsMap(t *testing.T) {
	t.Parallel()

	original := map[string]any{"value": "x", "url": "https://example.com"}
	flat := i18n.Flat(original)
	if len(flat) != len(original)*2 {
		t.Fatalf("Flat len = %d, want %d", len(flat), len(original)*2)
	}
	got := i18n.ToArgsMap(flat)
	if diff := cmp.Diff(original, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestFlat_EmptyMap(t *testing.T) {
	t.Parallel()

	got := i18n.Flat(nil)
	if len(got) != 0 {
		t.Errorf("Flat(nil) = %v, want empty", got)
	}
	got = i18n.Flat(map[string]any{})
	if len(got) != 0 {
		t.Errorf("Flat(empty) = %v, want empty", got)
	}
}

// TestPayload_SatisfiesLocalizedInterface pins the contract that Payload
// alone is enough to make a domain error type satisfy i18n.Localized — no
// per-type Localize override required. If this stops compiling, every domain
// error type that embeds Payload is at risk.
func TestPayload_SatisfiesLocalizedInterface(t *testing.T) {
	t.Parallel()

	type domainErr struct {
		i18n.Payload
	}
	var _ interface {
		I18nKey() string
		I18nArgs() map[string]any
		Localize(i18n.Locale) string
	} = domainErr{Payload: i18n.NewPayload("help.usage")}.Payload
}
