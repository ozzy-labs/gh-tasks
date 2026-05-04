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

// LocaleConfig is the subset of AppConfig consumed by ResolveLocale, kept
// here to avoid an import cycle with internal/config.
type LocaleConfig struct {
	Lang Locale
}

// EnvLookup mirrors os.Getenv but lets tests inject a fake env.
type EnvLookup func(key string) string

// ResolveLocale picks the output locale.
//
// Order:
//  1. --lang ja|en / --lang=ja|en flag (both forms)
//  2. config.Lang (from gh-tasks.toml)
//  3. LC_ALL env (ja* → ja)
//  4. LANG env (ja* → ja)
//  5. fallback → en
//
// Unknown --lang values are silently ignored and fall through to env.
func ResolveLocale(argv []string, env EnvLookup, config LocaleConfig) Locale {
	if env == nil {
		env = func(string) string { return "" }
	}
	if loc, ok := parseLangFlag(argv); ok {
		return loc
	}
	if config.Lang != "" {
		return config.Lang
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
func T(locale Locale, key string, args ...any) string {
	msg := lookup(locale, key)
	if len(args) == 0 {
		return msg
	}
	return Substitute(msg, ToArgsMap(args))
}

// ToArgsMap converts a flat key1, value1, key2, value2, ... varargs sequence
// into a map. Odd-length input drops the trailing key.
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
