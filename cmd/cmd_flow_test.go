// Cobra-rooted flow tests: each [TestCmd_*] case wires a fake GraphQL (and
// optionally REST) backend via [testDeps], invokes the real cobra root via
// [runCmd], and asserts on stdout/stderr/err. Shared helpers live in
// `testhelpers_test.go`. See `docs/design/test-structure.md` for the full
// rationale and the `Test<Cmd>_<Scenario>` naming convention.
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
	"github.com/ozzy-labs/gh-tasks/internal/period"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

// ===== List ================================================================

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

// ===== Today ===============================================================

func TestToday_RepoEmpty(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "today")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "No issues updated today") {
		t.Errorf("expected today.empty message, got:\n%s", stdout.String())
	}
}

// TestToday_FiltersByUTC pins the date-boundary filter contract: only issues
// whose updatedAt falls on the deps.Now UTC date are emitted.
func TestToday_FiltersByUTC(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssues (",
			Data: repoIssuesPayload(
				map[string]any{"id": "I_old", "number": 1, "title": "Yesterday", "url": "u1", "updatedAt": "2026-05-03T08:00:00Z"},
				map[string]any{"id": "I_today", "number": 2, "title": "Today", "url": "u2", "updatedAt": "2026-05-04T08:00:00Z"},
				map[string]any{"id": "I_tomorrow", "number": 3, "title": "Tomorrow", "url": "u3", "updatedAt": "2026-05-05T08:00:00Z"},
			),
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "today")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Today") {
		t.Errorf("expected Today entry, got:\n%s", got)
	}
	if strings.Contains(got, "Yesterday") || strings.Contains(got, "Tomorrow") {
		t.Errorf("expected only today's entries, got:\n%s", got)
	}
}

func TestToday_OrgScope(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{
			MatchSubstring: "query ListProjectV2Items (",
			Data: map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
				map[string]any{
					"id":        "ITEM_T",
					"updatedAt": "2026-05-04T08:00:00Z",
					"content": map[string]any{
						"__typename": "Issue",
						"id":         "I_99",
						"number":     99,
						"title":      "Today item",
						"url":        "https://example.com/99",
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
	stdout, _, err := runCmd(t, d, "today", "--scope", "org")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "Today item") || !strings.Contains(stdout.String(), "#99") {
		t.Errorf("expected today item line, got:\n%s", stdout.String())
	}
}

func TestToday_UserScopeEmpty(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{
			MatchSubstring: "query ListProjectV2Items (",
			Data:           map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{}}}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "today", "--scope", "user")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "No project items updated today") {
		t.Errorf("expected today.empty.project, got:\n%s", stdout.String())
	}
}

// ===== Standup =============================================================

// TestStandup_RepoStructure pins the markdown skeleton emitted in repo scope:
// the `# Standup` heading followed by `## Yesterday`, `## Today`, and
// `## Blockers` sections always render, even with no underlying activity.
func TestStandup_RepoStructure(t *testing.T) {
	t.Parallel()

	emptyRepoIssues := map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{}}}}
	emptyPRs := map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": []any{}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListClosedIssues (", Data: emptyRepoIssues},
		{MatchSubstring: "query ListMergedPRs (", Data: emptyPRs},
		{MatchSubstring: "query ListRepoIssues (", Data: emptyRepoIssues},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "standup")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"# Standup", "## Yesterday", "## Today", "## Blockers"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestStandup_SinceFlag(t *testing.T) {
	t.Parallel()

	emptyRepoIssues := map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{}}}}
	emptyPRs := map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": []any{}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListClosedIssues (", Data: emptyRepoIssues},
		{MatchSubstring: "query ListMergedPRs (", Data: emptyPRs},
		{MatchSubstring: "query ListRepoIssues (", Data: emptyRepoIssues},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "standup", "--since", "2026-05-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "since 2026-05-01T00:00:00Z") {
		t.Errorf("expected explicit since timestamp, got:\n%s", stdout.String())
	}
}

// TestStandup_SinceInvalidFailsFast pins the fail-fast contract for
// --since. The legacy implementation silently fell back to "24h ago" on a
// parse failure; the parsed-flag value should now classify as arg
// validation (exit 2) so the user notices their mistake instead of
// reading results scoped to a different window than they intended.
func TestStandup_SinceInvalidFailsFast(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "standup", "--since", "not-a-timestamp")
	if !errors.Is(err, cmd.ErrSilentArgs) {
		t.Fatalf("expected ErrSilentArgs for unparseable --since, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.standup.invalidSince", "value", "not-a-timestamp")
}

// TestStandup_MineViewerLoginUnresolvedFailsFast pins the fail-fast
// contract for --mine when the GraphQL response omits viewer.login (e.g.
// the active token has no associated user). Previously matchesViewer
// silently treated an empty viewer as "match everything", so the user
// got an unfiltered standup labelled as if --mine had taken effect.
func TestStandup_MineViewerLoginUnresolvedFailsFast(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerLogin", Data: map[string]any{"viewer": nil}},
	}}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "standup", "--since", "2026-05-04T00:00:00Z", "--mine")
	if !errors.Is(err, cmd.ErrSilentRuntime) {
		t.Fatalf("expected ErrSilentRuntime when viewer login is unresolved, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.standup.viewerLoginUnresolved")
}

func TestStandup_MineFiltersAndAnnotates(t *testing.T) {
	t.Parallel()

	closed := map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
		map[string]any{ // mine -- author=alice
			"id": "I_a", "number": 10, "title": "Mine closed", "url": "u/10",
			"closedAt":  "2026-05-04T08:00:00Z",
			"author":    map[string]any{"__typename": "User", "login": "alice"},
			"assignees": map[string]any{"nodes": []any{}},
		},
		map[string]any{ // not mine -- author=bob, no assignees
			"id": "I_b", "number": 11, "title": "Other closed", "url": "u/11",
			"closedAt":  "2026-05-04T08:00:00Z",
			"author":    map[string]any{"__typename": "User", "login": "bob"},
			"assignees": map[string]any{"nodes": []any{}},
		},
	}}}}
	merged := map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": []any{
		map[string]any{ // mine via assignee
			"id": "P_1", "number": 21, "title": "Assigned to me", "url": "p/21",
			"mergedAt":  "2026-05-04T09:00:00Z",
			"author":    map[string]any{"__typename": "User", "login": "carol"},
			"assignees": map[string]any{"nodes": []any{map[string]any{"login": "alice"}}},
		},
	}}}}
	open := map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
		map[string]any{ // mine
			"id": "I_o", "number": 33, "title": "WIP", "url": "u/33",
			"updatedAt": "2026-05-04T11:00:00Z",
			"author":    map[string]any{"__typename": "User", "login": "alice"},
			"assignees": map[string]any{"nodes": []any{}},
		},
		map[string]any{ // not mine
			"id": "I_x", "number": 34, "title": "Their WIP", "url": "u/34",
			"updatedAt": "2026-05-04T11:00:00Z",
			"author":    map[string]any{"__typename": "User", "login": "bob"},
			"assignees": map[string]any{"nodes": []any{}},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerLogin", Data: map[string]any{"viewer": map[string]any{"login": "alice"}}},
		{MatchSubstring: "query ListClosedIssues (", Data: closed},
		{MatchSubstring: "query ListMergedPRs (", Data: merged},
		{MatchSubstring: "query ListRepoIssues (", Data: open},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "standup", "--since", "2026-05-04T00:00:00Z", "--mine")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "(@alice)") {
		t.Errorf("expected viewer annotation in heading, got:\n%s", got)
	}
	for _, want := range []string{"Mine closed", "Assigned to me", "WIP"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing matched item %q in:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"Other closed", "Their WIP"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("unexpected non-matching item %q in:\n%s", unwanted, got)
		}
	}
}

func TestStandup_OrgScope_DoneSplit_DraftExcludedUnderMine(t *testing.T) {
	t.Parallel()

	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		map[string]any{ // done, alice
			"id":        "ITEM_DONE",
			"updatedAt": "2026-05-04T09:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_d", "number": 1, "title": "Done item", "url": "u/1",
				"author":    map[string]any{"__typename": "User", "login": "alice"},
				"assignees": map[string]any{"nodes": []any{}},
			},
			"fieldValues": map[string]any{"nodes": []any{
				map[string]any{
					"__typename": "ProjectV2ItemFieldSingleSelectValue",
					"optionId":   "OPT_DONE",
					"name":       "Done",
					"field":      map[string]any{"__typename": "ProjectV2SingleSelectField", "id": "F_S", "name": "Status"},
				},
			}},
		},
		map[string]any{ // in-progress, alice
			"id":        "ITEM_INPROG",
			"updatedAt": "2026-05-04T10:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_p", "number": 2, "title": "Active item", "url": "u/2",
				"author":    map[string]any{"__typename": "User", "login": "alice"},
				"assignees": map[string]any{"nodes": []any{}},
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
		map[string]any{ // draft -- excluded under --mine
			"id":        "ITEM_DRAFT",
			"updatedAt": "2026-05-04T10:00:00Z",
			"content": map[string]any{
				"__typename": "DraftIssue", "id": "DI_1", "title": "Drafted thought",
			},
			"fieldValues": map[string]any{"nodes": []any{}},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerLogin", Data: map[string]any{"viewer": map[string]any{"login": "alice"}}},
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "standup", "--since", "2026-05-04T00:00:00Z", "--scope", "org", "--mine")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	// Done item appears under Yesterday section, in-progress under Today.
	yesterdayIdx := strings.Index(got, "## Yesterday")
	todayIdx := strings.Index(got, "## Today")
	blockersIdx := strings.Index(got, "## Blockers")
	if yesterdayIdx < 0 || todayIdx < 0 || blockersIdx < 0 || yesterdayIdx >= todayIdx || todayIdx >= blockersIdx {
		t.Fatalf("expected Yesterday -> Today -> Blockers ordering, got:\n%s", got)
	}
	doneSegment := got[yesterdayIdx:todayIdx]
	progressSegment := got[todayIdx:blockersIdx]
	if !strings.Contains(doneSegment, "Done item") {
		t.Errorf("expected Done item under Yesterday, got:\n%s", doneSegment)
	}
	if !strings.Contains(progressSegment, "Active item") {
		t.Errorf("expected Active item under Today, got:\n%s", progressSegment)
	}
	if strings.Contains(got, "Drafted thought") {
		t.Errorf("DraftIssue must be excluded under --mine, got:\n%s", got)
	}
}

// ===== Review ==============================================================

// TestReview_RepoMarkdown pins the closed-issues markdown rendering for the
// default repo scope: the `## Closed Issues` section materialises and lists
// each closed issue by `#<num>`.
func TestReview_RepoMarkdown(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListClosedIssues (",
			Data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{"id": "I_x", "number": 7, "title": "Fix bug", "url": "u", "closedAt": "2026-05-04T09:00:00Z"},
			}}}},
		},
		{
			MatchSubstring: "query ListMergedPRs (",
			Data:           map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": []any{}}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "review", "--period", "weekly")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Closed Issues") || !strings.Contains(got, "#7") {
		t.Errorf("expected closed issues section, got:\n%s", got)
	}
}

func TestReview_PeriodDaily(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListClosedIssues (",
			Data:           map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{}}}},
		},
		{
			MatchSubstring: "query ListMergedPRs (",
			Data:           map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": []any{}}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "review", "--period", "daily")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "(daily)") {
		t.Errorf("expected daily heading, got:\n%s", got)
	}
	if !strings.Contains(got, "None") {
		t.Errorf("expected None placeholder when empty, got:\n%s", got)
	}
}

func TestReview_PeriodSprintDefaultsWeekly(t *testing.T) {
	t.Parallel()

	// Bare `review` (no --period) inherits cobra default "weekly".
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListClosedIssues (",
			Data:           map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{}}}},
		},
		{
			MatchSubstring: "query ListMergedPRs (",
			Data:           map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": []any{}}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "review")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "(weekly)") {
		t.Errorf("expected default weekly heading, got:\n%s", stdout.String())
	}
}

func TestReview_OrgProjectDoneFilter(t *testing.T) {
	t.Parallel()

	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		map[string]any{
			"id": "ITEM_D", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_done", "number": 100, "title": "Wrapped up", "url": "u/100",
			},
			"fieldValues": map[string]any{"nodes": []any{
				map[string]any{
					"__typename": "ProjectV2ItemFieldSingleSelectValue",
					"optionId":   "OPT_DONE",
					"name":       "Done",
					"field":      map[string]any{"__typename": "ProjectV2SingleSelectField", "id": "F_S", "name": "Status"},
				},
			}},
		},
		map[string]any{
			"id": "ITEM_T", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_todo", "number": 101, "title": "Still open", "url": "u/101",
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
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "review", "--scope", "org", "--period", "daily")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Wrapped up") {
		t.Errorf("expected Done-filtered item, got:\n%s", got)
	}
	if strings.Contains(got, "Still open") {
		t.Errorf("non-Done item leaked into review:\n%s", got)
	}
}

func TestReview_UserProjectEmptyPlaceholder(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Items (", Data: map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{}}}}},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "review", "--scope", "user", "--period", "weekly")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "No project items completed in this range") {
		t.Errorf("expected review.empty.project placeholder, got:\n%s", stdout.String())
	}
}

// ===== Error paths =========================================================

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

func TestReview_PeriodFlagInvalid(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "review", "--period", "yearly")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for invalid period, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.period.invalid", "value", "yearly", "valid", i18n.JoinPipe(period.Periods))
}

// ===== Add =================================================================

func TestAdd_RepoCreatesIssue(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query GetRepositoryID (",
			Data:           map[string]any{"repository": map[string]any{"id": "R_1"}},
		},
		{
			MatchSubstring: "mutation CreateIssue (",
			Data: map[string]any{"createIssue": map[string]any{"issue": map[string]any{
				"id": "I_new", "number": 123, "url": "https://github.com/ozzy-labs/gh-tasks/issues/123",
			}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "add", "Fix login")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Issue created") {
		t.Errorf("expected add.created.repo prefix, got:\n%s", got)
	}
	if !strings.Contains(got, "issues/123") {
		t.Errorf("expected created URL in output, got:\n%s", got)
	}
}

func TestAdd_RepoBodyFlagPropagates(t *testing.T) {
	t.Parallel()

	var seenInput map[string]any
	inner := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetRepositoryID (", Data: map[string]any{"repository": map[string]any{"id": "R_1"}}},
		{
			MatchSubstring: "mutation CreateIssue (",
			Data: map[string]any{"createIssue": map[string]any{"issue": map[string]any{
				"id": "I_new", "number": 124, "url": "u/124",
			}}},
		},
	}}
	wrap := &captureGraphQL{inner: inner, capture: func(query string, vars map[string]any) {
		if strings.Contains(query, "mutation CreateIssue (") {
			if v, ok := vars["input"].(map[string]any); ok {
				seenInput = v
			}
		}
	}}
	d := testDeps(inner, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	if _, _, err := runCmd(t, d, "add", "Fix login", "--body", "details here"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if seenInput["body"] != "details here" {
		t.Errorf("expected body propagated to mutation, saw %#v", seenInput)
	}
}

func TestAdd_TitleRequired(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "add")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN, "error.add.titleRequired")
}

func TestAdd_ProjectDraftItem(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{
			MatchSubstring: "mutation AddProjectV2DraftIssue (",
			Data:           map[string]any{"addProjectV2DraftIssue": map[string]any{"projectItem": map[string]any{"id": "DI_new"}}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "add", "Idea", "--scope=user")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Draft item added to project") {
		t.Errorf("expected add.created.project prefix, got:\n%s", got)
	}
	if !strings.Contains(got, "DI_new") {
		t.Errorf("expected created draft id, got:\n%s", got)
	}
}

func TestAdd_RepoMissingRepository(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetRepositoryID (", Data: map[string]any{"repository": nil}},
	}}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "add", "Title")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.repo.notFound", "owner", "ozzy-labs", "name", "gh-tasks")
}

// ===== Done ================================================================

func TestDone_RepoCloses(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query GetIssueByNumber (",
			Data: map[string]any{"repository": map[string]any{"issue": map[string]any{
				"id": "I_open", "number": 7, "url": "u/7", "state": "OPEN",
			}}},
		},
		{
			MatchSubstring: "mutation CloseIssue (",
			Data: map[string]any{"closeIssue": map[string]any{"issue": map[string]any{
				"id": "I_open", "number": 7, "url": "u/7", "state": "CLOSED",
			}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "done", "7")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "Issue closed") {
		t.Errorf("expected done.closed prefix, got:\n%s", stdout.String())
	}
}

func TestDone_RepoIdempotentAlreadyClosed(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query GetIssueByNumber (",
			Data: map[string]any{"repository": map[string]any{"issue": map[string]any{
				"id": "I_done", "number": 9, "url": "u/9", "state": "CLOSED",
			}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "done", "9")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "Issue is already closed") {
		t.Errorf("expected done.alreadyClosed, got:\n%s", stdout.String())
	}
}

func TestDone_ProjectStatusUpdate(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2SingleSelectField", "id": "F_STATUS", "name": "Status", "dataType": "SINGLE_SELECT",
			"options": []any{
				map[string]any{"id": "OPT_TODO", "name": "Todo"},
				map[string]any{"id": "OPT_DONE", "name": "Done"},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		map[string]any{
			"id": "ITEM_X", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_xx", "number": 1, "title": "x", "url": "u/x",
			},
			"fieldValues": map[string]any{"nodes": []any{
				map[string]any{
					"__typename": "ProjectV2ItemFieldSingleSelectValue",
					"optionId":   "OPT_TODO",
					"name":       "Todo",
					"field":      map[string]any{"__typename": "ProjectV2SingleSelectField", "id": "F_STATUS", "name": "Status"},
				},
			}},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
		{
			MatchSubstring: "mutation UpdateProjectV2ItemFieldValue (",
			Data:           map[string]any{"updateProjectV2ItemFieldValue": map[string]any{"projectV2Item": map[string]any{"id": "ITEM_X"}}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "done", "ITEM_X", "--scope=user")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "Project item Status set to Done") {
		t.Errorf("expected done.statusUpdated.project, got:\n%s", stdout.String())
	}
}

func TestDone_ProjectIdempotentAlreadyDone(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2SingleSelectField", "id": "F_STATUS", "name": "Status", "dataType": "SINGLE_SELECT",
			"options": []any{
				map[string]any{"id": "OPT_TODO", "name": "Todo"},
				map[string]any{"id": "OPT_DONE", "name": "Done"},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		map[string]any{
			"id": "ITEM_X", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{"__typename": "Issue", "id": "I_xx", "number": 1, "title": "x", "url": "u/x"},
			"fieldValues": map[string]any{"nodes": []any{
				map[string]any{
					"__typename": "ProjectV2ItemFieldSingleSelectValue",
					"optionId":   "OPT_DONE",
					"name":       "Done",
					"field":      map[string]any{"__typename": "ProjectV2SingleSelectField", "id": "F_STATUS", "name": "Status"},
				},
			}},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "done", "ITEM_X", "--scope=user")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "already Done") {
		t.Errorf("expected done.alreadyDone.project, got:\n%s", stdout.String())
	}
}

func TestDone_ProjectMissingStatusField(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{"__typename": "ProjectV2SingleSelectField", "id": "F_OTHER", "name": "Priority", "dataType": "SINGLE_SELECT", "options": []any{}},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, stderr, err := runCmd(t, d, "done", "ITEM_X", "--scope=user")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN, "error.done.statusFieldMissing")
}

func TestDone_ProjectMissingDoneOption(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2SingleSelectField", "id": "F_S", "name": "Status", "dataType": "SINGLE_SELECT",
			"options": []any{map[string]any{"id": "OPT_TODO", "name": "Todo"}},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, stderr, err := runCmd(t, d, "done", "ITEM_X", "--scope=user")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN, "error.done.doneOptionMissing")
}

func TestDone_ProjectItemNotFound(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2SingleSelectField", "id": "F_STATUS", "name": "Status", "dataType": "SINGLE_SELECT",
			"options": []any{
				map[string]any{"id": "OPT_TODO", "name": "Todo"},
				map[string]any{"id": "OPT_DONE", "name": "Done"},
			},
		},
	}}}}
	// Linear search returns a single non-matching item (well under doneItemsLimit=100),
	// so we expect the `error.projectItem.notFound` branch — not `error.done.searchLimit`.
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		map[string]any{
			"id": "ITEM_OTHER", "updatedAt": "2026-05-04T08:00:00Z",
			"content":     map[string]any{"__typename": "Issue", "id": "I_yy", "number": 2, "title": "y", "url": "u/y"},
			"fieldValues": map[string]any{"nodes": []any{}},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, stderr, err := runCmd(t, d, "done", "ITEM_X", "--scope=user")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projectItem.notFound", "id", "ITEM_X")
}

// ===== Link ================================================================

func TestLink_RepoAppendsClosesLink(t *testing.T) {
	t.Parallel()

	var seenBody string
	inner := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query GetPullRequestByNumber (",
			Data: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 12, "url": "https://github.com/ozzy-labs/gh-tasks/pull/12", "body": "Initial summary",
			}}},
		},
		{
			MatchSubstring: "mutation UpdatePullRequest (",
			Data: map[string]any{"updatePullRequest": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 12, "url": "https://github.com/ozzy-labs/gh-tasks/pull/12",
			}}},
		},
	}}
	wrap := &captureGraphQL{inner: inner, capture: func(query string, vars map[string]any) {
		if strings.Contains(query, "mutation UpdatePullRequest (") {
			if input, ok := vars["input"].(map[string]any); ok {
				if b, ok := input["body"].(string); ok {
					seenBody = b
				}
			}
		}
	}}
	d := testDeps(inner, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "link", "12", "42")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(seenBody, "Closes #42") {
		t.Errorf("expected appended Closes #42, got body=%q", seenBody)
	}
	if !strings.HasPrefix(seenBody, "Initial summary") {
		t.Errorf("expected original body preserved, got body=%q", seenBody)
	}
	if !strings.Contains(stdout.String(), "Appended Closes link") {
		t.Errorf("expected link.added prefix, got:\n%s", stdout.String())
	}
}

func TestLink_RepoIdempotentAlreadyLinked(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query GetPullRequestByNumber (",
			Data: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 12, "url": "u/12", "body": "Closes #42 already",
			}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "link", "12", "42")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "Already linked") {
		t.Errorf("expected link.alreadyLinked prefix, got:\n%s", stdout.String())
	}
}

func TestLink_ProjectDualAdd(t *testing.T) {
	t.Parallel()

	calls := 0
	inner := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{
			MatchSubstring: "query GetPullRequestByNumber (",
			Data: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 12, "url": "u/12", "body": "",
			}}},
		},
		{
			MatchSubstring: "query GetIssueByNumber (",
			Data: map[string]any{"repository": map[string]any{"issue": map[string]any{
				"id": "I_42", "number": 42, "url": "u/42", "state": "OPEN",
			}}},
		},
		{
			MatchSubstring: "mutation AddProjectV2ItemById (",
			Data:           map[string]any{"addProjectV2ItemById": map[string]any{"item": map[string]any{"id": "PI_pr"}}},
		},
		{
			MatchSubstring: "mutation AddProjectV2ItemById (",
			Data:           map[string]any{"addProjectV2ItemById": map[string]any{"item": map[string]any{"id": "PI_iss"}}},
		},
	}}
	wrap := &captureGraphQL{inner: inner, capture: func(query string, _ map[string]any) {
		if strings.Contains(query, "mutation AddProjectV2ItemById (") {
			calls++
		}
	}}
	d := testDeps(inner, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return true } // repo resolution still works via remote URL
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "link", "12", "42", "--scope=org")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 AddProjectV2ItemById calls, got %d", calls)
	}
	if !strings.Contains(stdout.String(), "Linked PR and task") {
		t.Errorf("expected link.added.project prefix, got:\n%s", stdout.String())
	}
}

// ===== Plan ================================================================

func TestPlan_RepoDryRunDaily(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssuesWithMilestone (",
			Data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{
					"id": "I_a", "number": 1, "title": "Task A", "url": "u/1",
					"updatedAt": "2026-05-04T08:00:00Z",
					"milestone": nil,
				},
			}}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "plan", "--period", "daily", "--dry-run")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Proposed milestone") {
		t.Errorf("expected plan.proposed prefix, got:\n%s", got)
	}
	if !strings.Contains(got, "Daily 2026-05-04") {
		t.Errorf("expected daily milestone title, got:\n%s", got)
	}
	if !strings.Contains(got, "--dry-run") {
		t.Errorf("expected dry-run note, got:\n%s", got)
	}
}

func TestPlan_RepoReuseExistingMilestone(t *testing.T) {
	t.Parallel()

	rest := &recordingREST{}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssuesWithMilestone (",
			Data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{
					"id": "I_a", "number": 1, "title": "Task A", "url": "u/1",
					"updatedAt": "2026-05-04T08:00:00Z",
					"milestone": nil,
				},
			}}}},
		},
		{
			MatchSubstring: "query ListMilestones (",
			Data: map[string]any{"repository": map[string]any{"milestones": map[string]any{"nodes": []any{
				map[string]any{"id": "M_1", "number": 5, "title": "Daily 2026-05-04"},
			}}}},
		},
		{
			MatchSubstring: "mutation UpdateIssueMilestone (",
			Data: map[string]any{"updateIssue": map[string]any{"issue": map[string]any{
				"id": "I_a", "number": 1, "url": "u/1",
				"milestone": map[string]any{"id": "M_1", "number": 5, "title": "Daily 2026-05-04"},
			}}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return newClientsWithREST(g, rest), nil
		}
	})
	stdout, _, err := runCmd(t, d, "plan", "--period", "daily")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Reused an existing milestone") {
		t.Errorf("expected plan.reused, got:\n%s", got)
	}
	if !strings.Contains(got, "Issue bound to milestone") {
		t.Errorf("expected plan.linked, got:\n%s", got)
	}
	for _, c := range rest.calls {
		if c.method == "POST" && strings.Contains(c.path, "/milestones") {
			t.Errorf("did not expect REST milestone create when reusing, saw call: %+v", c)
		}
	}
}

func TestPlan_RepoCreateNewMilestone(t *testing.T) {
	t.Parallel()

	rest := &recordingREST{responses: []restResponse{
		{
			matchMethod: "POST",
			matchPath:   "/milestones",
			data:        map[string]any{"node_id": "M_NEW", "id": 999, "number": 12, "title": "Daily 2026-05-04"},
		},
	}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssuesWithMilestone (",
			Data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{
					"id": "I_a", "number": 1, "title": "Task A", "url": "u/1",
					"updatedAt": "2026-05-04T08:00:00Z",
					"milestone": nil,
				},
			}}}},
		},
		{
			MatchSubstring: "query ListMilestones (",
			Data:           map[string]any{"repository": map[string]any{"milestones": map[string]any{"nodes": []any{}}}},
		},
		{
			MatchSubstring: "mutation UpdateIssueMilestone (",
			Data: map[string]any{"updateIssue": map[string]any{"issue": map[string]any{
				"id": "I_a", "number": 1, "url": "u/1",
				"milestone": map[string]any{"id": "M_NEW", "number": 12, "title": "Daily 2026-05-04"},
			}}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return newClientsWithREST(g, rest), nil
		}
	})
	stdout, _, err := runCmd(t, d, "plan", "--period", "daily")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "Milestone created") {
		t.Errorf("expected plan.created, got:\n%s", stdout.String())
	}
	sawCreate := false
	for _, c := range rest.calls {
		if c.method == "POST" && strings.Contains(c.path, "/milestones") {
			sawCreate = true
		}
	}
	if !sawCreate {
		t.Errorf("expected REST POST /milestones; calls=%+v", rest.calls)
	}
}

func TestPlan_ProjectIterationMatched(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations": []any{
					map[string]any{"id": "IT_1", "title": "Daily 2026-05-04", "startDate": "2026-05-04", "duration": 1},
				},
				"completedIterations": []any{},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily", "--dry-run")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Matched an existing iteration") {
		t.Errorf("expected plan.iterationMatched.project, got:\n%s", got)
	}
}

func TestPlan_ProjectIterationFallback(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations": []any{
					// Title doesn't match Daily target, but covers now (2026-05-04)
					map[string]any{"id": "IT_C", "title": "Sprint A", "startDate": "2026-05-01", "duration": 14},
				},
				"completedIterations": []any{},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, stderr, err := runCmd(t, d, "plan", "--scope=org", "--period=daily", "--dry-run")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := i18n.T(i18n.LocaleEN, "plan.iterationFallback.project")
	if !strings.Contains(stdout.String(), want) && !strings.Contains(stderr.String(), want) {
		t.Errorf("expected fallback note %q in stdout/stderr, stdout=%s\nstderr=%s",
			want, stdout.String(), stderr.String())
	}
}

// TestPlan_ProjectIterationUpdated covers the write half of `runPlanProject`
// without --dry-run: when an in-range item is *not* already on the target
// iteration, the command issues an UpdateProjectV2ItemFieldValue mutation
// and prints `plan.iterationUpdated.project`. The test wires a
// captureGraphQL around the fake to assert the mutation input shape (project
// id, item id, field id, iteration id) instead of just the human output, so
// a future refactor that swaps the mutation but keeps the message can't
// silently regress.
func TestPlan_ProjectIterationUpdated(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations": []any{
					map[string]any{"id": "IT_1", "title": "Daily 2026-05-04", "startDate": "2026-05-04", "duration": 1},
				},
				"completedIterations": []any{},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		map[string]any{
			"id": "ITEM_a", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_a", "number": 11, "title": "Wire iteration", "url": "u/11",
			},
			"fieldValues": map[string]any{"nodes": []any{}},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
		{
			MatchSubstring: "mutation UpdateProjectV2ItemFieldValue (",
			Data: map[string]any{"updateProjectV2ItemFieldValue": map[string]any{"projectV2Item": map[string]any{
				"id": "ITEM_a",
			}}},
		},
	}}
	var capturedInput map[string]any
	wrap := &captureGraphQL{inner: g, capture: func(query string, vars map[string]any) {
		if !strings.Contains(query, "mutation UpdateProjectV2ItemFieldValue (") {
			return
		}
		if in, ok := vars["input"].(map[string]any); ok {
			capturedInput = in
		}
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Project item iteration updated") {
		t.Errorf("expected plan.iterationUpdated.project on stdout, got:\n%s", got)
	}
	if !strings.Contains(got, "#11") {
		t.Errorf("expected updated-item label `#11`, got:\n%s", got)
	}
	if capturedInput == nil {
		t.Fatalf("expected UpdateProjectV2ItemFieldValue mutation to be invoked, captured none")
	}
	if capturedInput["projectId"] != "PVT_org" {
		t.Errorf("input.projectId = %v, want PVT_org", capturedInput["projectId"])
	}
	if capturedInput["itemId"] != "ITEM_a" {
		t.Errorf("input.itemId = %v, want ITEM_a", capturedInput["itemId"])
	}
	if capturedInput["fieldId"] != "F_IT" {
		t.Errorf("input.fieldId = %v, want F_IT", capturedInput["fieldId"])
	}
	value, _ := capturedInput["value"].(map[string]any)
	if value == nil || value["iterationId"] != "IT_1" {
		t.Errorf("input.value.iterationId mismatch, got value=%v", value)
	}
}

// TestPlan_ProjectAlreadyOnIteration covers the skip branch: when an
// in-range item already carries the resolved iteration id on the iteration
// field, `runPlanProject` must NOT call UpdateProjectV2ItemFieldValue and
// must surface `plan.iterationAlreadySet.project`. The fake registers no
// mutation response, so an accidental call would surface as `no fake
// response matched` and fail the test loudly.
func TestPlan_ProjectAlreadyOnIteration(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations": []any{
					map[string]any{"id": "IT_1", "title": "Daily 2026-05-04", "startDate": "2026-05-04", "duration": 1},
				},
				"completedIterations": []any{},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		map[string]any{
			"id": "ITEM_b", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_b", "number": 22, "title": "Already wired", "url": "u/22",
			},
			"fieldValues": map[string]any{"nodes": []any{
				map[string]any{
					"__typename":  "ProjectV2ItemFieldIterationValue",
					"iterationId": "IT_1",
					"title":       "Daily 2026-05-04",
					"startDate":   "2026-05-04",
					"duration":    1,
					"field": map[string]any{
						"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration",
					},
				},
			}},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Skipped (already on the target iteration)") {
		t.Errorf("expected plan.iterationAlreadySet.project, got:\n%s", got)
	}
	if !strings.Contains(got, "#22") {
		t.Errorf("expected describeItem `#22`, got:\n%s", got)
	}
}

// TestPlan_ProjectNotFound covers the project-resolution failure path: when
// PaginateProjectV2Fields returns ErrProjectNotFound (e.g. project deleted
// after the initial id resolve), `runPlanProject` must surface the
// localized `error.project.notFound` on stderr and return ErrSilent so the
// CLI exits with a clean code. The mutation responses are intentionally
// absent.
func TestPlan_ProjectNotFound(t *testing.T) {
	t.Parallel()

	// First the project resolves fine, then ListProjectV2Fields returns a
	// node payload without the ProjectV2 inline fragment; the paginator
	// translates that into ErrProjectNotFound, exercising the
	// errors.Is(err, queries.ErrProjectNotFound) branch in runPlanProject.
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{
			MatchSubstring: "query ListProjectV2Fields (",
			// Wrong concrete type triggers ErrProjectNotFound inside the
			// ProjectV2-typed paginator.
			Data: map[string]any{"node": map[string]any{"__typename": "Issue"}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, stderr, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.project.notFound", "owner", "octo", "number", 7, "scope", "org")
}

// TestPlan_ProjectIterationFieldMissing exercises the
// `error.plan.iterationFieldMissing` branch: a project whose fields list
// carries no ITERATION-typed entry must surface the localized message and
// return ErrSilent without ever calling the items query.
func TestPlan_ProjectIterationFieldMissing(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2SingleSelectField", "id": "F_S", "name": "Status", "dataType": "SINGLE_SELECT",
			"options": []any{},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, stderr, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.plan.iterationFieldMissing")
}

// TestPlan_ProjectNoIterationsAvailable covers the second mid-flow guard:
// the iteration field exists but its configuration carries an empty
// iterations list (resolveTargetIteration returns nil). The command must
// emit `error.plan.noIterationsAvailable` and return ErrSilent.
func TestPlan_ProjectNoIterationsAvailable(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations":          []any{},
				"completedIterations": []any{},
			},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, stderr, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.plan.noIterationsAvailable")
}

// ===== Triage ==============================================================

func TestTriage_RepoUnlabelledOnly(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssuesWithLabels (",
			Data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{
					"id": "I_u", "number": 1, "title": "Untriaged", "url": "u/1", "updatedAt": "2026-05-04T08:00:00Z",
					"labels": map[string]any{"nodes": []any{}},
				},
				map[string]any{
					"id": "I_l", "number": 2, "title": "Already triaged", "url": "u/2", "updatedAt": "2026-05-04T08:00:00Z",
					"labels": map[string]any{"nodes": []any{map[string]any{"name": "bug"}}},
				},
			}}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "triage")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Untriaged") || !strings.Contains(got, "#1") {
		t.Errorf("expected unlabelled hit, got:\n%s", got)
	}
	if strings.Contains(got, "Already triaged") {
		t.Errorf("expected labelled issue filtered out, got:\n%s", got)
	}
}

func TestTriage_ProjectStatusTriageFilter(t *testing.T) {
	t.Parallel()

	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		map[string]any{
			"id": "ITEM_T", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_t", "number": 1, "title": "Needs triage", "url": "u/1",
			},
			"fieldValues": map[string]any{"nodes": []any{
				map[string]any{
					"__typename": "ProjectV2ItemFieldSingleSelectValue",
					"optionId":   "OPT_TRIAGE",
					"name":       "Triage",
					"field":      map[string]any{"__typename": "ProjectV2SingleSelectField", "id": "F_S", "name": "Status"},
				},
			}},
		},
		map[string]any{
			"id": "ITEM_OK", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_ok", "number": 2, "title": "On track", "url": "u/2",
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
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "triage", "--scope=user")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Needs triage") {
		t.Errorf("expected Triage-status item, got:\n%s", got)
	}
	if strings.Contains(got, "On track") {
		t.Errorf("non-triage item leaked: %s", got)
	}
}

// ===== Projects ============================================================

func TestProjectsInit_DryRunHeader(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "My Todo", "--dry-run")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "(--dry-run)") || !strings.Contains(got, "My Todo") {
		t.Errorf("expected dry-run header with title, got:\n%s", got)
	}
	// User template fields: Status (single_select with Triage/Todo/Done) +
	// Iteration. Verify both surface.
	if !strings.Contains(got, "Status") || !strings.Contains(got, "Iteration") {
		t.Errorf("expected user-template fields in output, got:\n%s", got)
	}
}

func TestProjectsInit_DryRunOrgTemplate(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "projects", "init", "--template", "org", "--title", "Team", "--dry-run")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Status", "Iteration", "Project"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected org-template field %q in output, got:\n%s", want, got)
		}
	}
}

func TestProjectsInit_TitleRequired(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--template", "user")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projectsInit.titleRequired")
}

func TestProjectsInit_TemplateRequired(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--title", "Foo")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projectsInit.templateRequired")
}

func TestProjectsInit_TemplateConflict(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "x", "/tmp/some.yaml")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projectsInit.templateConflict")
}

func TestProjectsInit_OwnerNotFound(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOwnerID (", Data: map[string]any{"repositoryOwner": nil}},
	}}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "x", "--owner", "ghost")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projectsInit.ownerNotFound", "owner", "ghost")
}

// TestProjectsInit_CreatesNewFields drives the mutation-success half of
// runProjectsInit end-to-end with a captured GraphQL fake, asserting:
//   - GetViewerID is consulted for --owner @me
//   - CreateProjectV2 receives the title + viewer-resolved owner id
//   - CreateProjectV2Field is invoked once per template entry
//   - The 3 genqlient subtype payloads (Common / SingleSelect / Iteration)
//     all flow through projectV2FieldDescriptor without losing name/dataType
//
// The user-template ships 2 fields (Status[single_select], Iteration); we
// canon a CreateProjectV2Field response per call to stage one Common, one
// SingleSelectField, and (via the second mutation) one IterationField --
// that combination gives projectV2FieldDescriptor full type-switch coverage.
func TestProjectsInit_CreatesNewFields(t *testing.T) {
	t.Parallel()

	mutationInputs := []map[string]any{}
	inner := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerID", Data: map[string]any{"viewer": map[string]any{
			"id": "U_viewer", "login": "me",
		}}},
		{MatchSubstring: "mutation CreateProjectV2 (", Data: map[string]any{
			"createProjectV2": map[string]any{"projectV2": map[string]any{
				"id": "PVT_new", "number": 1, "title": "My Todo",
				"url": "https://example.test/p/1",
			}},
		}},
		// PaginateProjectV2Fields runs once after the project is created;
		// returning an empty connection ensures every template field hits
		// the create path (no `existingNames` skip).
		{MatchSubstring: "query ListProjectV2Fields (", Data: map[string]any{
			"node": map[string]any{
				"__typename": "ProjectV2",
				"fields": map[string]any{
					"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					"nodes":    []any{},
				},
			},
		}},
		// First field create: Status (SINGLE_SELECT). Stage the response
		// as a SingleSelectField so the type-switch hits that arm.
		{MatchSubstring: "mutation CreateProjectV2Field (", Data: map[string]any{
			"createProjectV2Field": map[string]any{"projectV2Field": map[string]any{
				"__typename": "ProjectV2SingleSelectField",
				"id":         "F_status", "name": "Status", "dataType": "SINGLE_SELECT",
			}},
		}},
		// Second field create: Iteration. Stage as IterationField.
		{MatchSubstring: "mutation CreateProjectV2Field (", Data: map[string]any{
			"createProjectV2Field": map[string]any{"projectV2Field": map[string]any{
				"__typename": "ProjectV2IterationField",
				"id":         "F_iter", "name": "Iteration", "dataType": "ITERATION",
			}},
		}},
	}}
	wrap := &captureGraphQL{inner: inner, capture: func(query string, vars map[string]any) {
		if strings.Contains(query, "mutation CreateProjectV2 (") ||
			strings.Contains(query, "mutation CreateProjectV2Field (") {
			if input, ok := vars["input"].(map[string]any); ok {
				// Defensive copy so subsequent mutations don't mutate
				// the captured snapshot through the shared map.
				snapshot := map[string]any{}
				for k, v := range input {
					snapshot[k] = v
				}
				snapshot["__op__"] = query
				mutationInputs = append(mutationInputs, snapshot)
			}
		}
	}}
	d := testDeps(inner, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "My Todo")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Project created") || !strings.Contains(got, "https://example.test/p/1") {
		t.Errorf("expected projectsInit.created with URL, got:\n%s", got)
	}
	if !strings.Contains(got, "Status") || !strings.Contains(got, "Iteration") {
		t.Errorf("expected each created field to surface in stdout, got:\n%s", got)
	}
	// CreateProjectV2 + 2x CreateProjectV2Field = 3 capture entries.
	if len(mutationInputs) != 3 {
		t.Fatalf("captured %d mutation inputs, want 3: %#v", len(mutationInputs), mutationInputs)
	}
	if !strings.Contains(mutationInputs[0]["__op__"].(string), "mutation CreateProjectV2 (") {
		t.Errorf("first mutation should be CreateProjectV2, got %q", mutationInputs[0]["__op__"])
	}
	if mutationInputs[0]["ownerId"] != "U_viewer" {
		t.Errorf("CreateProjectV2.ownerId = %v, want U_viewer", mutationInputs[0]["ownerId"])
	}
	if mutationInputs[0]["title"] != "My Todo" {
		t.Errorf("CreateProjectV2.title = %v, want My Todo", mutationInputs[0]["title"])
	}
	// Field mutations: first is Status (SINGLE_SELECT, with options).
	if mutationInputs[1]["projectId"] != "PVT_new" || mutationInputs[1]["name"] != "Status" {
		t.Errorf("Status mutation projectId/name mismatch: %#v", mutationInputs[1])
	}
	if mutationInputs[1]["dataType"] != "SINGLE_SELECT" {
		t.Errorf("Status dataType = %v, want SINGLE_SELECT", mutationInputs[1]["dataType"])
	}
	if opts, ok := mutationInputs[1]["singleSelectOptions"].([]any); !ok || len(opts) == 0 {
		t.Errorf("Status mutation should carry singleSelectOptions, got %#v", mutationInputs[1]["singleSelectOptions"])
	}
	// Second: Iteration (no options).
	if mutationInputs[2]["name"] != "Iteration" || mutationInputs[2]["dataType"] != "ITERATION" {
		t.Errorf("Iteration mutation = %#v", mutationInputs[2])
	}
}

// TestProjectsInit_SkipsExistingFields verifies the existingNames branch:
// when PaginateProjectV2Fields surfaces a field whose name (case-insensitive)
// matches one in the template, runProjectsInit emits projectsInit.fieldSkipped
// and does *not* issue CreateProjectV2Field for that entry.
func TestProjectsInit_SkipsExistingFields(t *testing.T) {
	t.Parallel()

	createdFieldNames := []string{}
	inner := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerID", Data: map[string]any{"viewer": map[string]any{
			"id": "U_viewer", "login": "me",
		}}},
		{MatchSubstring: "mutation CreateProjectV2 (", Data: map[string]any{
			"createProjectV2": map[string]any{"projectV2": map[string]any{
				"id": "PVT_skip", "number": 2, "title": "Reuse",
				"url": "https://example.test/p/2",
			}},
		}},
		// Pre-populate "status" (lowercase) to exercise the case-insensitive
		// skip — the template field is "Status".
		{MatchSubstring: "query ListProjectV2Fields (", Data: map[string]any{
			"node": map[string]any{
				"__typename": "ProjectV2",
				"fields": map[string]any{
					"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					"nodes": []any{
						map[string]any{
							"__typename": "ProjectV2SingleSelectField",
							"id":         "F_existing_status", "name": "status", "dataType": "SINGLE_SELECT",
							"options": []any{},
						},
					},
				},
			},
		}},
		// Only Iteration should hit CreateProjectV2Field after the skip.
		{MatchSubstring: "mutation CreateProjectV2Field (", Data: map[string]any{
			"createProjectV2Field": map[string]any{"projectV2Field": map[string]any{
				"__typename": "ProjectV2IterationField",
				"id":         "F_iter", "name": "Iteration", "dataType": "ITERATION",
			}},
		}},
	}}
	wrap := &captureGraphQL{inner: inner, capture: func(query string, vars map[string]any) {
		if strings.Contains(query, "mutation CreateProjectV2Field (") {
			if input, ok := vars["input"].(map[string]any); ok {
				if name, ok := input["name"].(string); ok {
					createdFieldNames = append(createdFieldNames, name)
				}
			}
		}
	}}
	d := testDeps(inner, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "Reuse")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "skipped") || !strings.Contains(got, "Status") {
		t.Errorf("expected projectsInit.fieldSkipped for Status, got:\n%s", got)
	}
	if !strings.Contains(got, "Iteration") {
		t.Errorf("expected Iteration to still be created, got:\n%s", got)
	}
	if len(createdFieldNames) != 1 || createdFieldNames[0] != "Iteration" {
		t.Errorf("expected exactly one CreateProjectV2Field call for Iteration, saw %v", createdFieldNames)
	}
}

// TestProjectsInit_TemplateNotFound exercises the YAML-path-missing branch
// of loadTemplateRaw via the runProjectsInit error funnel: the user passes
// a yaml path that doesn't exist, so the command emits
// `error.projectsInit.yamlRead` and exits with ErrSilent before any
// GraphQL calls are issued.
func TestProjectsInit_TemplateNotFound(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{} // any GraphQL call would fall through with "no fake response matched"
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--title", "x", "/path/does/not/exist.yaml")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	// reason is OS-dependent ("no such file or directory" on linux, etc.)
	// so we can't assert on the full rendered string. Instead we render
	// only the {path} placeholder and assert the prefix-up-to-{reason}
	// portion appears, which still detects key/wording mismatches.
	rendered := i18n.T(i18n.LocaleEN, "error.projectsInit.yamlRead",
		"path", "/path/does/not/exist.yaml")
	prefix := strings.SplitN(rendered, "{reason}", 2)[0]
	if !strings.Contains(stderr.String(), prefix) {
		t.Errorf("expected yamlRead prefix %q in stderr, got:\n%s", prefix, stderr.String())
	}
}

// ===== projects (group) + init-templates wiring ============================
//
// These tests pin the cobra wiring of the `projects` subcommand group and
// the small bundled-template printer. They do NOT exercise GitHub API
// surface — `runProjectsInit` already has full mutation/skip/error coverage
// in the TestProjectsInit_* block above. Their job is to lock the
// command-tree shape (groups, available subcommands, flag rejection) and
// the literal stdout that operators copy-paste into their own YAML files
// when bootstrapping a board.

// TestProjectsCmd_NoArgs pins that running `gh tasks projects` without any
// subcommand emits the i18n `error.projects.subcommandRequired` notice on
// stderr and exits with [cmd.ErrSilent] (via the underlying
// [cmd.ErrSilentArgs]). Stdout must stay empty because the parent agent
// pipes the output of `projects init-templates` into a YAML file — any
// stray group-level chatter would corrupt that pipe.
func TestProjectsCmd_NoArgs(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, stderr, err := runCmd(t, d, "projects")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for no-subcommand projects, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projects.subcommandRequired")
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout when no subcommand is given, got:\n%s", stdout.String())
	}
}

// TestProjectsCmd_HelpFlag pins that `gh tasks projects --help` produces
// cobra's standard help layout and lists both `init` and `init-templates`
// under "Available Commands:". This guards against accidental subcommand
// removal / rename — the group wiring inside `newProjectsCmd` is exercised
// only when `AddCommand` is consulted by cobra during help rendering.
func TestProjectsCmd_HelpFlag(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "projects", "--help")
	if err != nil {
		t.Fatalf("Execute --help: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Available Commands:", "init", "init-templates"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in projects --help output, got:\n%s", want, got)
		}
	}
}

// TestProjectsInitTemplates_PrintsUserYaml pins that the `# --template
// user` section of `init-templates` carries the bundled user-scope YAML
// (name + fields + Status + Iteration). The literal copy that operators
// pipe into their own YAML files is part of the public CLI contract, so
// the assertion locks the structural markers (top-level keys + scope
// title + the two required fields) rather than the entire string body.
func TestProjectsInitTemplates_PrintsUserYaml(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "projects", "init-templates")
	if err != nil {
		t.Fatalf("Execute init-templates: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"# --template user",
		"name: gh-tasks user scope",
		"fields:",
		"- name: Status",
		"type: single_select",
		"- name: Iteration",
		"type: iteration",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("user template section missing %q, got:\n%s", want, got)
		}
	}
}

// TestProjectsInitTemplates_PrintsOrgYaml pins the `# --template org`
// section, which extends the user template with `Repository` (built-in
// field type) and a free-form `Project` single_select. Operators rely on
// these two extra fields to coordinate cross-repo work in a team Project,
// so any drift in the bundled YAML body is a customer-visible change.
func TestProjectsInitTemplates_PrintsOrgYaml(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "projects", "init-templates")
	if err != nil {
		t.Fatalf("Execute init-templates: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"# --template org",
		"name: gh-tasks org scope",
		"- name: Repository",
		"type: repository",
		"- name: Project",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("org template section missing %q, got:\n%s", want, got)
		}
	}
	// The org section must follow the user section (the printer emits
	// user first, then a blank line, then org). Verify the ordering so a
	// future refactor that swaps them can't slip through.
	userIdx := strings.Index(got, "# --template user")
	orgIdx := strings.Index(got, "# --template org")
	if userIdx < 0 || orgIdx < 0 || userIdx >= orgIdx {
		t.Errorf("expected user section before org section, indices user=%d org=%d:\n%s",
			userIdx, orgIdx, got)
	}
}

// TestProjectsInitTemplates_InvalidTemplate pins that `init-templates`
// rejects unknown flags such as `--template foo`. The command is a small
// stdout printer that takes no flags by design (it emits both bundled
// templates unconditionally), so cobra's unknown-flag error is the
// expected behaviour. This guards against a future refactor that
// silently absorbs / ignores unrecognised flags.
func TestProjectsInitTemplates_InvalidTemplate(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init-templates", "--template", "foo")
	if err == nil {
		t.Fatalf("expected error for --template flag on init-templates, got nil")
	}
	// cobra surfaces unknown flags via "unknown flag" in the error / stderr.
	combined := err.Error() + "\n" + stderr.String()
	if !strings.Contains(combined, "unknown flag") {
		t.Errorf("expected 'unknown flag' in error/stderr, got err=%v stderr=%s",
			err, stderr.String())
	}
}

// ===== --lang flag E2E ======================================================
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

// ===== --mine viewer matching ===============================================
//
// matchesViewerOnItem (cmd/standup.go) is exercised via the org-scope standup
// path. Each case constructs a single project item and checks whether the
// item leaks past the --mine filter when viewer.login is "alice".

// standupOrgMineDeps builds the testDeps shape used by all --mine matrix
// tests below: org-scope, no git remote, OrgProject pre-set in config.
func standupOrgMineDeps(t *testing.T, g *testfake.FakeGraphQL) cmd.Deps {
	t.Helper()
	return testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
}

// standupOrgMineItem builds the canonical "in-progress Issue at since+1h"
// project-item shape used by the --mine matrix tests. Caller customizes
// the author / assignees fields per case.
func standupOrgMineItem(id, title, authorLogin string, assignees []string) map[string]any {
	assigneeNodes := make([]any, 0, len(assignees))
	for _, a := range assignees {
		assigneeNodes = append(assigneeNodes, map[string]any{"login": a})
	}
	var author any
	if authorLogin != "" {
		author = map[string]any{"__typename": "User", "login": authorLogin}
	}
	return map[string]any{
		"id":        id,
		"updatedAt": "2026-05-04T10:00:00Z",
		"content": map[string]any{
			"__typename": "Issue", "id": "I_" + id, "number": 1, "title": title, "url": "u/" + id,
			"author":    author,
			"assignees": map[string]any{"nodes": assigneeNodes},
		},
		"fieldValues": map[string]any{"nodes": []any{
			map[string]any{
				"__typename": "ProjectV2ItemFieldSingleSelectValue",
				"optionId":   "OPT_TODO",
				"name":       "Todo",
				"field":      map[string]any{"__typename": "ProjectV2SingleSelectField", "id": "F_S", "name": "Status"},
			},
		}},
	}
}

// TestStandup_MineMatchesAuthor pins the author-only branch of
// matchesViewerOnItem: viewer.login matches content.author and assignees is
// empty.
func TestStandup_MineMatchesAuthor(t *testing.T) {
	t.Parallel()

	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		standupOrgMineItem("AUTH", "Authored item", "alice", nil),
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerLogin", Data: map[string]any{"viewer": map[string]any{"login": "alice"}}},
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := standupOrgMineDeps(t, g)
	stdout, _, err := runCmd(t, d, "standup", "--since", "2026-05-04T00:00:00Z", "--scope", "org", "--mine")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "Authored item") {
		t.Errorf("author-match must surface item, got:\n%s", stdout.String())
	}
}

// TestStandup_MineMatchesAssignee pins the assignee-membership branch: the
// viewer is one of the assignees while the author is somebody else. This
// is the branch that was uncovered before (matchesViewerOnItem 55.6%).
func TestStandup_MineMatchesAssignee(t *testing.T) {
	t.Parallel()

	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		standupOrgMineItem("ASSN", "Assigned to alice", "bob", []string{"carol", "alice"}),
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerLogin", Data: map[string]any{"viewer": map[string]any{"login": "alice"}}},
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := standupOrgMineDeps(t, g)
	stdout, _, err := runCmd(t, d, "standup", "--since", "2026-05-04T00:00:00Z", "--scope", "org", "--mine")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "Assigned to alice") {
		t.Errorf("assignee-match must surface item, got:\n%s", stdout.String())
	}
}

// TestStandup_MineMatchesNeither pins the exclude-when-no-match branch:
// the viewer is neither the author nor in the assignees list, so the item
// must drop out. The today-section placeholder (standup.empty.project)
// signals successful filtering.
func TestStandup_MineMatchesNeither(t *testing.T) {
	t.Parallel()

	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		standupOrgMineItem("OTHR", "Their item", "bob", []string{"carol"}),
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerLogin", Data: map[string]any{"viewer": map[string]any{"login": "alice"}}},
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := standupOrgMineDeps(t, g)
	stdout, _, err := runCmd(t, d, "standup", "--since", "2026-05-04T00:00:00Z", "--scope", "org", "--mine")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if strings.Contains(got, "Their item") {
		t.Errorf("non-matching item must be filtered out under --mine, got:\n%s", got)
	}
	// Both Yesterday and Today sections should fall back to the empty
	// placeholder, since the only candidate item was filtered.
	emptyPlaceholder := i18n.T(i18n.LocaleEN, "standup.empty.project")
	if !strings.Contains(got, emptyPlaceholder) {
		t.Errorf("expected empty.project placeholder after filtering, got:\n%s", got)
	}
}

// TestStandup_MineMatchesAuthorAndAssignee pins the dedup contract: when the
// viewer is both author and an assignee, the item appears exactly once
// (matchesViewerOnItem returns true on the author check before falling
// through to the assignee loop).
func TestStandup_MineMatchesAuthorAndAssignee(t *testing.T) {
	t.Parallel()

	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		standupOrgMineItem("DUAL", "Self assigned", "alice", []string{"alice", "bob"}),
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerLogin", Data: map[string]any{"viewer": map[string]any{"login": "alice"}}},
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := standupOrgMineDeps(t, g)
	stdout, _, err := runCmd(t, d, "standup", "--since", "2026-05-04T00:00:00Z", "--scope", "org", "--mine")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if n := strings.Count(got, "Self assigned"); n != 1 {
		t.Errorf("expected exactly one occurrence of item title, got %d in:\n%s", n, got)
	}
}
