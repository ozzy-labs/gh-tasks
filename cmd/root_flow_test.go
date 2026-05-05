// Cobra-rooted flow tests for root-level concerns that are not tied to a
// single subcommand (locale resolution, persistent flag wiring). Shared
// helpers live in `testhelpers_test.go`. See
// `docs/design/test-structure.md` for rationale.
package cmd_test

import (
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

// TestRoot_LangResolutionPriority is a table-driven flow test of the
// flag > config > env > default precedence chain. Each row drives a list
// command with empty repo issues so the output reduces to the localized
// list.empty placeholder, which we assert is sourced from the expected
// catalog. Covers all four resolution branches in deps.Resolve.
func TestRoot_LangResolutionPriority(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		flagArgs   []string
		envLang    string
		cfgLocale  i18n.Locale
		wantLocale i18n.Locale
	}{
		{
			name:       "flag_wins_over_config_and_env",
			flagArgs:   []string{"--lang", "ja"},
			envLang:    "en_US.UTF-8",
			cfgLocale:  i18n.LocaleEN,
			wantLocale: i18n.LocaleJA,
		},
		{
			name:       "config_wins_when_no_flag",
			flagArgs:   nil,
			envLang:    "en_US.UTF-8",
			cfgLocale:  i18n.LocaleJA,
			wantLocale: i18n.LocaleJA,
		},
		{
			name:       "env_wins_when_no_flag_no_config",
			flagArgs:   nil,
			envLang:    "ja_JP.UTF-8",
			cfgLocale:  "",
			wantLocale: i18n.LocaleJA,
		},
		{
			name:       "default_en_when_nothing_set",
			flagArgs:   nil,
			envLang:    "",
			cfgLocale:  "",
			wantLocale: i18n.LocaleEN,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
				{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
			}}
			env := tc.envLang
			cfg := tc.cfgLocale
			d := testDeps(g, func(d *cmd.Deps) {
				d.Env = func(key string) string {
					if key == "LANG" {
						return env
					}
					return ""
				}
				d.LoadConfig = func() (config.AppConfig, error) {
					return config.AppConfig{Locale: cfg}, nil
				}
			})
			args := append([]string{}, tc.flagArgs...)
			args = append(args, "list")
			stdout, _, err := runCmd(t, d, args...)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			want := i18n.T(tc.wantLocale, "list.empty")
			if !strings.Contains(stdout.String(), want) {
				t.Errorf("locale %q: expected %q in output, got:\n%s", tc.wantLocale, want, stdout.String())
			}
			// Also assert the opposite catalog's string is NOT present so we
			// don't accidentally match a substring shared between locales.
			otherLocale := i18n.LocaleEN
			if tc.wantLocale == i18n.LocaleEN {
				otherLocale = i18n.LocaleJA
			}
			other := i18n.T(otherLocale, "list.empty")
			if other != want && strings.Contains(stdout.String(), other) {
				t.Errorf("locale %q: unexpected %q (other locale) leaked into output:\n%s", tc.wantLocale, other, stdout.String())
			}
		})
	}
}
