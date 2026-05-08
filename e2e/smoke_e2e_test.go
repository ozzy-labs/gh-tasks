//go:build e2e

package e2e

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// versionRe matches a semver-ish substring like "0.2.0" or "v1.2.3-rc.1".
// gh-tasks --version prints "gh-tasks version <semver>" by cobra default.
var versionRe = regexp.MustCompile(`v?\d+\.\d+\.\d+`)

// TestE2E_SmokeVersion verifies the binary built in TestMain runs and
// emits a version-like string on --version. No GitHub interaction.
func TestE2E_SmokeVersion(t *testing.T) {
	out, stderr, code := runBin(t, "--version")
	if code != 0 {
		t.Fatalf("--version exit=%d stderr=%q stdout=%q", code, stderr, out)
	}
	if !versionRe.MatchString(out) {
		t.Errorf("--version stdout missing semver substring: %q", out)
	}
}

// TestE2E_SmokeAuth verifies the host gh CLI is authenticated and that the
// active token has `project` scope (the API call returns 403 otherwise).
// Uses the gh binary directly, not gh-tasks, so it works as a pre-flight
// even before E2E test code is written.
func TestE2E_SmokeAuth(t *testing.T) {
	if out, err := exec.Command("gh", "auth", "status").CombinedOutput(); err != nil {
		t.Fatalf("gh auth status failed: %v\n%s", err, out)
	}

	const probe = `query{viewer{projectsV2(first:1){totalCount}}}`
	out, err := exec.Command("gh", "api", "graphql", "-f", "query="+probe).CombinedOutput()
	if err != nil {
		t.Fatalf("gh api projectsV2 probe failed (project scope likely missing — try `gh auth refresh -s project`): %v\n%s", err, out)
	}
}

// TestE2E_SmokeReadOnly verifies that gh-tasks can read the two test
// Projects without writing. Uses --limit 1 to keep the GraphQL response
// small and -p / -s explicit to bypass scope auto-detection.
func TestE2E_SmokeReadOnly(t *testing.T) {
	cases := []struct {
		name  string
		scope string
		proj  string
	}{
		{"org", "org", "ozzy-labs/3"},
		{"user", "user", "ozzy-3/5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runBin(t,
				"-s", tc.scope, "-p", tc.proj,
				"list", "--limit", "1",
			)
			if code != 0 {
				t.Fatalf("list -s %s -p %s exit=%d stdout=%q stderr=%q",
					tc.scope, tc.proj, code, stdout, stderr)
			}
		})
	}
}

// TestE2E_SmokeI18nGraphQL verifies the en/ja catalogs are wired up on a
// path that actually reaches GraphQL. Uses a non-existent project number
// so the resolver hits GitHub, gets a NotFound, and surfaces the error
// through `error.project.notFound` (i18n key). Compared to the previous
// `--scope team` smoke which short-circuited at cobra-level scope
// validation, this test exercises the GraphQL transport + locale resolution
// + catalog lookup as a single unit.
func TestE2E_SmokeI18nGraphQL(t *testing.T) {
	messages := map[string]string{}
	for _, lang := range []string{"en", "ja"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			_, stderr, code := runBin(t,
				"--lang", lang,
				"-s", "user", "-p", "ozzy-3/99999",
				"list", "--limit", "1",
			)
			if code == 0 {
				t.Fatalf("expected non-zero exit on non-existent project, got 0")
			}
			if stderr == "" {
				t.Fatalf("expected localized stderr message, got empty")
			}
			messages[lang] = strings.TrimSpace(stderr)
		})
	}
	if messages["en"] != "" && messages["ja"] != "" && messages["en"] == messages["ja"] {
		t.Errorf("en and ja messages identical (i18n catalogs not switching): %q", messages["en"])
	}
}

// TestE2E_SmokeWriteRoundtrip_Org exercises a full add → done cycle on the
// org-scope test Project. This is the smoke that would have caught the
// genqlient null-list bug fixed in 2026-05 — the previous read-only smoke
// was structurally blind to mutation-time wire format issues.
func TestE2E_SmokeWriteRoundtrip_Org(t *testing.T) {
	smokeWriteRoundtrip(t, "org", "ozzy-labs/3")
}

// TestE2E_SmokeWriteRoundtrip_User exercises the same add → done cycle on
// the user-scope test Project. Org and user scopes share the underlying
// GraphQL mutations, but project_id resolution differs (Org GraphQL root
// vs. User GraphQL root), hence the explicit duplicate.
func TestE2E_SmokeWriteRoundtrip_User(t *testing.T) {
	smokeWriteRoundtrip(t, "user", "ozzy-3/5")
}

// smokeWriteRoundtrip is the per-scope body of the WriteRoundtrip smoke.
// Per the test plan §6 cleanup policy, the created draft item is left in
// Status=Done rather than physically deleted, so this test accumulates
// audit trail items at one-per-run pace. Monthly archival sweeps are
// expected to keep `ozzy-labs/3` and `ozzy-3/5` from getting too noisy.
func smokeWriteRoundtrip(t *testing.T, scope, project string) {
	t.Helper()
	id := addProjectDraft(t, scope, project, uniqueTitle("smoke roundtrip "+scope))
	markProjectItemDone(t, scope, project, id)
}
