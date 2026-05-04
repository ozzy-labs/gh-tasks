// Package i18n loads embedded en/ja message catalogs and resolves the user's
// locale from CLI args, env vars, or config. The catalog format mirrors the TS
// implementation (flat string-keyed JSON with `{name}` placeholder syntax) so
// the en/ja JSON files can be shared 1:1 with the legacy TS source.
package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Locale is the supported output locale.
type Locale string

// Supported locales.
const (
	LocaleEN Locale = "en"
	LocaleJA Locale = "ja"
)

// Validate reports whether v is a known Locale.
func Validate(v string) (Locale, bool) {
	switch Locale(v) {
	case LocaleEN, LocaleJA:
		return Locale(v), true
	}
	return "", false
}

//go:embed en.json
var enRaw []byte

//go:embed ja.json
var jaRaw []byte

var tables = map[Locale]map[string]string{}

func init() {
	for loc, raw := range map[Locale][]byte{LocaleEN: enRaw, LocaleJA: jaRaw} {
		m := map[string]string{}
		if err := json.Unmarshal(raw, &m); err != nil {
			panic(fmt.Sprintf("i18n: malformed %s catalog: %v", loc, err))
		}
		tables[loc] = m
	}
}

// LangProvider is the small interface ResolveLocaleFor consumes. Any caller
// that can answer "what locale did the user configure?" satisfies it; this
// lets callers (e.g. internal/config.AppConfig) pass themselves in directly
// without wrapping into i18n.LocaleConfig.
//
// Implementers should return the empty Locale when the user has not
// configured one, in which case ResolveLocaleFor falls through to env vars
// and the en default.
type LangProvider interface {
	Lang() Locale
}

// LocaleConfig is the legacy struct-shaped LangProvider, kept for backward
// compatibility with callers that constructed it directly. New code should
// pass any LangProvider implementation (typically the AppConfig itself).
type LocaleConfig struct {
	Lang Locale
}

// langProviderFunc adapts a Locale value into a LangProvider. Used by
// ResolveLocale so the legacy struct-shaped API can delegate to the new
// interface-shaped implementation.
type langProviderFunc func() Locale

func (f langProviderFunc) Lang() Locale { return f() }

// EnvLookup mirrors os.Getenv but lets tests inject a fake env.
type EnvLookup func(key string) string

// ResolveLocale picks the output locale, taking the legacy LocaleConfig
// shape for backward compatibility. New code should prefer ResolveLocaleFor.
//
// Order (shared with ResolveLocaleFor):
//  1. --lang ja|en / --lang=ja|en flag (both forms)
//  2. config.Lang (from gh-tasks.toml)
//  3. LC_ALL env (ja* → ja)
//  4. LANG env (ja* → ja)
//  5. fallback → en
//
// Unknown --lang values are silently ignored and fall through to env.
func ResolveLocale(argv []string, env EnvLookup, config LocaleConfig) Locale {
	return ResolveLocaleFor(argv, env, langProviderFunc(func() Locale { return config.Lang }))
}

// ResolveLocaleFor picks the output locale.
//
// Order:
//  1. --lang ja|en / --lang=ja|en flag (both forms)
//  2. provider.Lang() (typically from gh-tasks.toml)
//  3. LC_ALL env (ja* → ja)
//  4. LANG env (ja* → ja)
//  5. fallback → en
//
// A nil provider is treated as "no configured lang" and falls through to env
// resolution. Unknown --lang values are silently ignored and fall through.
func ResolveLocaleFor(argv []string, env EnvLookup, provider LangProvider) Locale {
	if env == nil {
		env = func(string) string { return "" }
	}
	if loc, ok := parseLangFlag(argv); ok {
		return loc
	}
	if provider != nil {
		if loc := provider.Lang(); loc != "" {
			return loc
		}
	}
	if v := env("LC_ALL"); v != "" {
		if strings.HasPrefix(strings.ToLower(v), "ja") {
			return LocaleJA
		}
		return LocaleEN
	}
	if v := env("LANG"); strings.HasPrefix(strings.ToLower(v), "ja") {
		return LocaleJA
	}
	return LocaleEN
}

func parseLangFlag(argv []string) (Locale, bool) {
	for i, arg := range argv {
		if strings.HasPrefix(arg, "--lang=") {
			if loc, ok := Validate(strings.TrimPrefix(arg, "--lang=")); ok {
				return loc, true
			}
			return "", false
		}
		if arg == "--lang" {
			if i+1 < len(argv) {
				if loc, ok := Validate(argv[i+1]); ok {
					return loc, true
				}
			}
			return "", false
		}
	}
	return "", false
}

// T renders the localized message for key, substituting {name} placeholders
// with values from args. Missing translations fall back to en, then key.
//
// args is treated as alternating (key string, value any) pairs, the same
// shape ToArgsMap consumes. Pairs whose key is not a string are silently
// dropped, and an odd-length args (a trailing unpaired key) is silently
// truncated. Callers must always pass complete (string-key, value) pairs.
func T(locale Locale, key string, args ...any) string {
	msg := lookup(locale, key)
	if len(args) == 0 {
		return msg
	}
	return Substitute(msg, ToArgsMap(args))
}

// ToArgsMap converts a flat key1, value1, key2, value2, ... varargs sequence
// into a map. Pairs whose key is not a string are silently dropped (the
// value is also discarded), and an odd-length input silently drops the
// trailing unpaired key. The flat varargs shape is intentional for ergonomic
// call sites, so callers must always pass complete (string-key, value)
// pairs — non-string keys are a programmer error and produce missing
// substitutions rather than runtime errors.
func ToArgsMap(args []any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(args); i += 2 {
		k, ok := args[i].(string)
		if !ok {
			continue
		}
		m[k] = args[i+1]
	}
	return m
}

// placeholderPattern matches `{name}` tokens where `name` is one or more
// word characters (letters, digits, underscore). Used by Substitute for a
// single deterministic pass over the message.
var placeholderPattern = regexp.MustCompile(`\{(\w+)\}`)

// Substitute replaces every `{name}` token in msg with the matching value
// from args (rendered via fmt.Sprint). Tokens with no matching entry are
// left untouched.
//
// The replacement is a single left-to-right regex pass, so the result is
// deterministic regardless of Go map iteration order and `{name}` tokens
// that appear inside an already-substituted value are not re-expanded.
//
// args is keyed by string. When callers build args via T/NewPayload's
// flat varargs interface, any non-string key in that varargs is dropped at
// the ToArgsMap boundary before reaching Substitute, so its placeholder is
// left as `{name}` in the output (silent — never a runtime error).
func Substitute(msg string, args map[string]any) string {
	if len(args) == 0 {
		return msg
	}
	return placeholderPattern.ReplaceAllStringFunc(msg, func(match string) string {
		// match is `{name}`; strip the braces to get the key.
		key := match[1 : len(match)-1]
		v, ok := args[key]
		if !ok {
			return match
		}
		return fmt.Sprint(v)
	})
}

func lookup(locale Locale, key string) string {
	if msg, ok := tables[locale][key]; ok {
		return msg
	}
	if msg, ok := tables[LocaleEN][key]; ok {
		return msg
	}
	return key
}
