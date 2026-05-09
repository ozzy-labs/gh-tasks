// Cobra-rooted flow tests for the `list` command. Each [TestList_*] case
// wires a fake GraphQL (and optionally REST) backend via [testDeps],
// invokes the real cobra root via [runCmd], and asserts on stdout/stderr/
// err. Shared helpers live in `testhelpers_test.go`. See
// `docs/design/test-structure.md` for the full rationale and the
// `Test<Cmd>_<Scenario>` naming convention.
package cmd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

func TestList_RepoEmpty(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "list")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "No open issues") {
		t.Errorf("missing empty-state message:\n%s", stdout.String())
	}
}

func TestList_RepoIssues(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssues (",
			Data: repoIssuesPayload(
				map[string]any{
					"id":        "I_1",
					"number":    42,
					"title":     "Fix login",
					"url":       "https://example.com/i/42",
					"updatedAt": "2026-05-04T08:00:00Z",
				},
			),
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "list")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "#42") || !strings.Contains(got, "Fix login") {
		t.Errorf("missing expected output:\n%s", got)
	}
}

func TestList_LimitDefault(t *testing.T) {
	t.Parallel()

	var capturedFirst int
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
	}}
	wrap := &captureGraphQL{inner: g, capture: func(_ string, vars map[string]any) {
		capturedFirst = intFromVar(vars["first"])
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	if _, _, err := runCmd(t, d, "list"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if capturedFirst != 30 {
		t.Errorf("expected default limit 30, got %d", capturedFirst)
	}
}

func TestList_LimitExplicit(t *testing.T) {
	t.Parallel()

	var capturedFirst int
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
	}}
	wrap := &captureGraphQL{inner: g, capture: func(_ string, vars map[string]any) {
		capturedFirst = intFromVar(vars["first"])
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	if _, _, err := runCmd(t, d, "list", "--limit", "5"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if capturedFirst != 5 {
		t.Errorf("expected limit 5, got %d", capturedFirst)
	}
}

// TestList_LimitZero pins the defensive default-back in cmd/list.go: an
// explicit `--limit 0` is meaningless and would otherwise propagate to
// pageStep as 0, terminating before any request. The cmd layer instead falls
// back to defaultListLimit (30) so the user gets a useful page on a fat-finger
// input. Audit follow-up #285 (C-4 --limit edge-case coverage).
func TestList_LimitZero(t *testing.T) {
	t.Parallel()

	var capturedFirst int
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
	}}
	wrap := &captureGraphQL{inner: g, capture: func(_ string, vars map[string]any) {
		capturedFirst = intFromVar(vars["first"])
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	if _, _, err := runCmd(t, d, "list", "--limit", "0"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if capturedFirst != 30 {
		t.Errorf("--limit 0 should fall back to defaultListLimit (30), got %d", capturedFirst)
	}
}

// TestList_LimitNegative pins that a negative `--limit` value also triggers
// the defensive default-back (limit <= 0 branch). Negative ints are not
// rejected at flag-parse time because pflag's IntVar accepts the full int
// range; the cmd layer normalises them instead. Audit follow-up #285 (C-4).
func TestList_LimitNegative(t *testing.T) {
	t.Parallel()

	var capturedFirst int
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
	}}
	wrap := &captureGraphQL{inner: g, capture: func(_ string, vars map[string]any) {
		capturedFirst = intFromVar(vars["first"])
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	if _, _, err := runCmd(t, d, "list", "--limit", "-5"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if capturedFirst != 30 {
		t.Errorf("--limit -5 should fall back to defaultListLimit (30), got %d", capturedFirst)
	}
}

// TestList_LimitVeryLarge pins the maxPages * maxPageSize safety valve in the
// queries paginator: even with `--limit 100000` and a server that keeps
// claiming hasNextPage=true, the cmd halts after maxPages (10) pages of
// maxPageSize (100) items, capping the rendered list at 1000 entries.
// Audit follow-up #285 (C-4).
func TestList_LimitVeryLarge(t *testing.T) {
	t.Parallel()

	const (
		pages       = 10  // matches queries.maxPages
		perPage     = 100 // matches queries.maxPageSize
		expectTotal = pages * perPage
	)
	responses := make([]testfake.FakeResponse, 0, pages)
	for p := 0; p < pages; p++ {
		nodes := make([]any, 0, perPage)
		for i := 0; i < perPage; i++ {
			n := p*perPage + i
			nodes = append(nodes, map[string]any{
				"id":        fmt.Sprintf("I_%d", n),
				"number":    n + 1,
				"title":     fmt.Sprintf("issue %d", n),
				"url":       fmt.Sprintf("https://example.com/i/%d", n+1),
				"updatedAt": "2026-05-04T08:00:00Z",
			})
		}
		cursor := fmt.Sprintf("C%d", p)
		// Always advertise hasNextPage=true so the loop only stops at
		// maxPages, not because the server told us we're done.
		responses = append(responses, testfake.FakeResponse{
			MatchSubstring: "query ListRepoIssues (",
			Data: map[string]any{
				"repository": map[string]any{
					"issues": map[string]any{
						"nodes": nodes,
						"pageInfo": map[string]any{
							"hasNextPage": true,
							"endCursor":   cursor,
						},
					},
				},
			},
		})
	}

	g := &testfake.FakeGraphQL{Responses: responses}
	var (
		callCount    int
		lastCaptured int
	)
	wrap := &captureGraphQL{inner: g, capture: func(_ string, vars map[string]any) {
		callCount++
		lastCaptured = intFromVar(vars["first"])
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "list", "--limit", "100000")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// pageStep clips per-page request size to maxPageSize=100 even when the
	// caller's remaining budget is huge.
	if lastCaptured != perPage {
		t.Errorf("expected per-page first=%d (maxPageSize), got %d", perPage, lastCaptured)
	}
	if callCount != pages {
		t.Errorf("expected exactly %d page requests (maxPages), got %d", pages, callCount)
	}
	// Each issue renders as `#N  Title\n  URL\n` — count `#` prefixes at
	// line starts as a stable proxy for "issues printed".
	got := stdout.String()
	hashLines := 0
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "#") {
			hashLines++
		}
	}
	if hashLines > expectTotal {
		t.Errorf("safety valve breach: rendered %d issues, want <= %d", hashLines, expectTotal)
	}
	if hashLines != expectTotal {
		t.Errorf("expected %d issues rendered (maxPages*maxPageSize), got %d", expectTotal, hashLines)
	}
}

// TestList_LimitDefaultInHelp guards the cobra-generated help string that
// surfaces the `defaultListLimit` constant to end users. A regression here
// (e.g. someone changing the IntFlag default without updating tests) would
// silently shift documented behaviour. Audit follow-up #285 (C-4).
func TestList_LimitDefaultInHelp(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "list", "--help")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "--limit int") {
		t.Errorf("expected --limit flag in help output, got:\n%s", got)
	}
	if !strings.Contains(got, "default 30") {
		t.Errorf("expected `default 30` in help output, got:\n%s", got)
	}
}

func TestList_OrgProjectFound(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{
			MatchSubstring: "query ListProjectV2Items (",
			Data: map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
				map[string]any{
					"id":        "ITEM_1",
					"updatedAt": "2026-05-04T08:00:00Z",
					"content": map[string]any{
						"__typename": "Issue",
						"id":         "I_42",
						"number":     42,
						"title":      "Repo issue",
						"url":        "https://example.com/i/42",
					},
					"fieldValues": map[string]any{"nodes": []any{
						map[string]any{
							"__typename": "ProjectV2ItemFieldSingleSelectValue",
							"optionId":   "OPT_TODO",
							"name":       "Todo",
							"field":      map[string]any{"__typename": "ProjectV2SingleSelectField", "id": "F_S", "name": "Status"},
						},
					}},
				},
				map[string]any{
					"id":        "ITEM_2",
					"updatedAt": "2026-05-04T08:00:00Z",
					"content": map[string]any{
						"__typename": "PullRequest",
						"id":         "PR_5",
						"number":     5,
						"title":      "Open PR",
						"url":        "https://example.com/pr/5",
					},
					"fieldValues": map[string]any{"nodes": []any{}},
				},
				map[string]any{
					"id":        "ITEM_3",
					"updatedAt": "2026-05-04T08:00:00Z",
					"content": map[string]any{
						"__typename": "DraftIssue",
						"id":         "DI_1",
						"title":      "Brainstorm idea",
					},
					"fieldValues": map[string]any{"nodes": []any{}},
				},
			}}}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "list", "--scope", "org")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"#42", "Repo issue", "[Todo]", "PR#5", "Open PR", "(draft)", "Brainstorm idea"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestList_OrgProjectNotFound(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: emptyOrgProject()},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, stderr, err := runCmd(t, d, "list", "--scope", "org")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.project.notFound", "owner", "octo", "number", 7, "scope", "org")
}

func TestList_UserProjectMissingRef(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
	})
	_, stderr, err := runCmd(t, d, "list", "--scope", "user")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	// Pin the configKey placeholder value so a future rename of the TOML
	// key surfaces here. The catalog wording around it is intentionally not
	// asserted (#284 Phase 2).
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.project.notSpecified", "scope", "user", "configKey", "user_project")
}

// ===== List error paths ====================================================

func TestList_ConfigError(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{}, &config.ConfigError{Payload: i18n.NewPayload(
				"error.config.tomlParseFailed",
				"path", "/tmp/cfg.toml", "reason", "expected '='",
			)}
		}
	})
	_, stderr, err := runCmd(t, d, "list")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.config.tomlParseFailed", "path", "/tmp/cfg.toml", "reason", "expected '='")
}

func TestList_ScopeFlagInvalid(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "list", "--scope=bogus")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for invalid scope, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.scope.invalid", "value", "bogus", "valid", i18n.JoinPipe(scope.Valid))
}

func TestList_RepoNotResolved(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return true }
		d.GetRemoteURL = func() (string, bool) { return "", false }
	})
	_, stderr, err := runCmd(t, d, "list")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for unresolved repo, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN, "error.repo.notResolved")
}

func TestList_ProjectFlagInvalid(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
	})
	_, stderr, err := runCmd(t, d, "list", "--scope=org", "--project=bogus")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for invalid project, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.project.invalidIdentifier", "value", "bogus")
}

// ===== --lang flag E2E (list) ==============================================
//
// These tests exercise the full cobra → deps.Resolve → r.T wiring rather than
// the i18n.ResolveLocaleFor unit (covered by internal/i18n/i18n_test.go). The
// goal is to pin the contract that user-supplied `--lang ja` actually flips
// command output to the ja catalog, including when env / config disagree.

// TestList_LangJaSwitchesOutput pins that `--lang ja` causes the list-empty
// placeholder to render from the ja catalog. The default-locale TestList_RepoEmpty
// asserts the en string above; this test asserts the ja counterpart so any
// regression that drops the --lang wiring fails here.
func TestList_LangJaSwitchesOutput(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "--lang", "ja", "list")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	wantJA := i18n.T(i18n.LocaleJA, "list.empty")
	if !strings.Contains(stdout.String(), wantJA) {
		t.Errorf("expected ja list.empty %q, got:\n%s", wantJA, stdout.String())
	}
	wantEN := i18n.T(i18n.LocaleEN, "list.empty")
	if strings.Contains(stdout.String(), wantEN) {
		t.Errorf("expected ja-only output, but en %q leaked through:\n%s", wantEN, stdout.String())
	}
}

// TestList_LangFlagOverridesEnvAndConfig pins the precedence contract: a
// `--lang ja` flag wins over both LANG=en* env and a config carrying
// Locale=en. ResolveLocaleFor unit tests cover this for the resolver in
// isolation; here we exercise the whole cmd-layer wiring (cobra persistent
// flag → flagString → langArgv → ResolveLocaleFor) end-to-end.
func TestList_LangFlagOverridesEnvAndConfig(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.Env = func(key string) string {
			if key == "LANG" {
				return "en_US.UTF-8"
			}
			return ""
		}
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{Locale: i18n.LocaleEN}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "--lang", "ja", "list")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	wantJA := i18n.T(i18n.LocaleJA, "list.empty")
	if !strings.Contains(stdout.String(), wantJA) {
		t.Errorf("--lang ja must win over LANG=en + config Locale=en; got:\n%s", stdout.String())
	}
}

// ===== --json / --jq path (#367 PR 1) =====================================

// TestList_JSONFieldsRepo asserts that --json restricts output to the
// requested fields and emits a JSON array shaped per the Phase 1 contract
// (id, number, title, type, updatedAt, url). Existing repo-issue payload
// builders are reused; assertions go through assertJSONFieldEquals to pin
// the exact value rather than substring-matching the raw stdout.
func TestList_JSONFieldsRepo(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssues (",
			Data: repoIssuesPayload(
				map[string]any{
					"id":        "I_1",
					"number":    42,
					"title":     "Fix login",
					"url":       "https://example.com/i/42",
					"updatedAt": "2026-05-04T08:00:00Z",
				},
			),
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "list", "--json", "id,number,title,type")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	assertJSONLength(t, stdout.String(), 1)
	assertJSONFieldEquals(t, stdout.String(), 0, "id", "I_1")
	assertJSONFieldEquals(t, stdout.String(), 0, "number", 42)
	assertJSONFieldEquals(t, stdout.String(), 0, "title", "Fix login")
	assertJSONFieldEquals(t, stdout.String(), 0, "type", "ISSUE")
	// fields not requested must not appear (otherwise the contract leaks
	// the full DTO and breaks payload-minimization).
	rows := parseJSONArray(t, stdout.String())
	for _, k := range []string{"url", "updatedAt"} {
		if _, present := rows[0][k]; present {
			t.Errorf("rows[0] should not include unrequested field %q; got: %v", k, rows[0])
		}
	}
}

// TestList_JSONEmptyArgListsFields pins the gh-style discoverability
// behaviour: `--json` with no value writes the catalog to stderr, exits
// with ErrSilentArgs, and leaves stdout empty (so a piped consumer sees
// nothing rather than a partial JSON).
func TestList_JSONEmptyArgListsFields(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{}}
	d := testDeps(g)
	stdout, stderr, err := runCmd(t, d, "list", "--json", "")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent (catalog listing), got %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty when listing fields, got: %s", stdout.String())
	}
	for _, want := range []string{"id", "number", "title", "type", "updatedAt", "url"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr must list %q in catalog, got:\n%s", want, stderr.String())
		}
	}
}

// TestList_JSONUnknownField pins the error path when the user passes an
// invalid field name: the command fails with ErrSilentArgs, prints the
// unknown-field error + the catalog to stderr, and emits no stdout.
func TestList_JSONUnknownField(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
	}}
	d := testDeps(g)
	stdout, stderr, err := runCmd(t, d, "list", "--json", "id,bogus,title")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for unknown field, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty on validation failure, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "bogus") {
		t.Errorf("stderr must name the unknown field, got:\n%s", stderr.String())
	}
}

// TestList_JQFiltersOutput pins the gojq integration: --jq applies an
// expression to the array and writes only the matching values to stdout.
// `.[].id` is the canonical "extract one field per row" jq idiom.
func TestList_JQFiltersOutput(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssues (",
			Data: repoIssuesPayload(
				map[string]any{"id": "I_a", "number": 1, "title": "a", "url": "u/1", "updatedAt": "2026-05-04T08:00:00Z"},
				map[string]any{"id": "I_b", "number": 2, "title": "b", "url": "u/2", "updatedAt": "2026-05-04T09:00:00Z"},
			),
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "list", "--json", "id", "--jq", ".[].id")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, `"I_a"`) || !strings.Contains(got, `"I_b"`) {
		t.Errorf("expected both ids in jq output, got:\n%s", got)
	}
	// No JSON array wrapper / object — gojq emits one value per line.
	if strings.HasPrefix(strings.TrimSpace(got), "[") {
		t.Errorf("jq output should be unwrapped values, got array: %s", got)
	}
}

// TestList_JQWithoutJSON pins the rule that --jq requires --json. Mixing
// text mode with a jq expression is rejected at flag-validation time.
func TestList_JQWithoutJSON(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{}}
	d := testDeps(g)
	stdout, stderr, err := runCmd(t, d, "list", "--jq", ".[].id")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for --jq without --json, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout must be empty when rejecting flag combo, got: %s", stdout.String())
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN, "error.json.jqWithoutJson")
}
