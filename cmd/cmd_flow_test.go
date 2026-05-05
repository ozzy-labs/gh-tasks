package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
)

// recordingREST captures REST calls and replays canned responses keyed by
// `<METHOD> <path-substring>`. Unmatched calls return a marshalable empty body.
type recordingREST struct {
	calls     []restCall
	responses []restResponse
}

type restCall struct {
	method string
	path   string
	body   any
}

type restResponse struct {
	matchMethod string
	matchPath   string
	data        any
	err         error
}

func (r *recordingREST) Do(_ context.Context, method, path string, body, out any) error {
	r.calls = append(r.calls, restCall{method: method, path: path, body: body})
	for _, resp := range r.responses {
		if resp.matchMethod != "" && resp.matchMethod != method {
			continue
		}
		if resp.matchPath != "" && !strings.Contains(path, resp.matchPath) {
			continue
		}
		if resp.err != nil {
			return resp.err
		}
		if out == nil || resp.data == nil {
			return nil
		}
		buf, err := json.Marshal(resp.data)
		if err != nil {
			return fmt.Errorf("marshal rest fake: %w", err)
		}
		return json.Unmarshal(buf, out)
	}
	return nil
}

// newClientsWithREST is a counterpart to newClients that lets a test inject a
// recordingREST in addition to the GraphQL fake.
func newClientsWithREST(g *fakeGraphQL, r *recordingREST) *github.Clients {
	return &github.Clients{Host: "github.com", GraphQL: g, REST: r}
}

// runCmd is a small helper that bootstraps the cobra root with the supplied
// deps + argv, captures stdout/stderr into the bytes.Buffers wired into
// d.Stdout / d.Stderr, and returns the resulting err. It also wires
// SetOut/SetErr on the root command itself, since cmd handlers prefer
// c.OutOrStdout()/c.ErrOrStderr() over deps.Stdout/deps.Stderr.
//
// As a convenience, when d.Argv is empty, runCmd populates it with a
// process-shaped argv (`["gh-tasks", args...]`) so the legacy flag parsers in
// internal/{scope,repo,project,period,i18n} see the flags consistently with
// what cobra parses. Tests that need a different argv (e.g. a malformed
// --scope value) should set d.Argv before calling runCmd.
func runCmd(t *testing.T, d cmd.Deps, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	stdout = d.Stdout.(*bytes.Buffer)
	stderr = d.Stderr.(*bytes.Buffer)
	if len(d.Argv) == 0 {
		argv := make([]string, 0, len(args)+1)
		argv = append(argv, "gh-tasks")
		argv = append(argv, args...)
		d.Argv = argv
	}
	root := cmd.RootWithDeps(d)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err = root.Execute()
	return stdout, stderr, err
}

// repoIssuesPayload constructs the `repository.issues.nodes` shape consumed by
// ListRepoIssues across many tests.
func repoIssuesPayload(nodes ...map[string]any) map[string]any {
	return map[string]any{
		"repository": map[string]any{
			"issues": map[string]any{"nodes": append([]any{}, asAnySlice(nodes)...)},
		},
	}
}

func asAnySlice(in []map[string]any) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// emptyOrgProject builds a `nil` projectV2 envelope under organization (used
// to drive the not-found path in scope=org tests).
func emptyOrgProject() map[string]any {
	return map[string]any{"organization": map[string]any{"projectV2": nil}}
}

func orgProject(id string) map[string]any {
	return map[string]any{"organization": map[string]any{"projectV2": map[string]any{
		"id": id, "number": 7, "title": "Org Project",
	}}}
}

func userProject(id string) map[string]any {
	return map[string]any{"user": map[string]any{"projectV2": map[string]any{
		"id": id, "number": 9, "title": "User Project",
	}}}
}

// ===== List ================================================================

func TestList_LimitDefault(t *testing.T) {
	t.Parallel()

	var capturedFirst int
	g := &fakeGraphQL{}
	g.responses = []fakeResponse{
		{
			matchSubstring: "query ListRepoIssues (",
			data:           repoIssuesPayload(),
		},
	}
	// Wrap the fake so we can capture the variables.
	wrap := &captureGraphQL{inner: g, capture: func(_ string, vars map[string]any) {
		capturedFirst = intFromVar(vars["first"])
	}}
	d := cmd.Deps{
		Stdout:       new(bytes.Buffer),
		Stderr:       new(bytes.Buffer),
		Now:          func() time.Time { return time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC) },
		Env:          func(string) string { return "" },
		HasGitRemote: func() bool { return true },
		GetRemoteURL: func() (string, bool) { return "git@github.com:ozzy-labs/gh-tasks.git", true },
		NewClients: func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		},
		LoadConfig: func() (config.AppConfig, error) { return config.AppConfig{}, nil },
		Argv:       []string{},
	}
	if _, _, err := runCmd(t, d, "list"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if capturedFirst != 30 {
		t.Errorf("expected default limit 30, got %d", capturedFirst)
	}
}

// intFromVar extracts an integer from a variable captured by [captureGraphQL].
// Genqlient-generated calls pass variables through JSON, so numeric values
// are unmarshalled to float64; hand-written call sites still pass ints
// directly. This helper accepts both.
func intFromVar(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case float64:
		return int(x)
	}
	return 0
}

func TestList_LimitExplicit(t *testing.T) {
	t.Parallel()

	var capturedFirst int
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query ListRepoIssues (", data: repoIssuesPayload()},
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

func TestList_OrgProjectFound(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetOrgProjectV2 (", data: orgProject("PVT_org")},
		{
			matchSubstring: "query ListProjectV2Items(",
			data: map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{
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
							"field":      map[string]any{"id": "F_S", "name": "Status"},
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

	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetOrgProjectV2 (", data: emptyOrgProject()},
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
	if !strings.Contains(stderr.String(), "Project not found") {
		t.Errorf("expected localized notFound on stderr, got:\n%s", stderr.String())
	}
}

func TestList_UserProjectMissingRef(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
	})
	_, stderr, err := runCmd(t, d, "list", "--scope", "user")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), "user_project") {
		t.Errorf("expected configKey hint in stderr, got:\n%s", stderr.String())
	}
}

// ===== Today ===============================================================

func TestToday_RepoEmpty(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query ListRepoIssues (", data: repoIssuesPayload()},
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

func TestToday_OrgScope(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetOrgProjectV2 (", data: orgProject("PVT_org")},
		{
			matchSubstring: "query ListProjectV2Items(",
			data: map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{
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

	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetUserProjectV2 (", data: userProject("PVT_user")},
		{
			matchSubstring: "query ListProjectV2Items(",
			data:           map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{}}}},
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

func TestStandup_SinceFlag(t *testing.T) {
	t.Parallel()

	emptyRepoIssues := map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{}}}}
	emptyPRs := map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": []any{}}}}
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query ListClosedIssues (", data: emptyRepoIssues},
		{matchSubstring: "query ListMergedPRs (", data: emptyPRs},
		{matchSubstring: "query ListRepoIssues (", data: emptyRepoIssues},
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
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetViewerLogin", data: map[string]any{"viewer": map[string]any{"login": "alice"}}},
		{matchSubstring: "query ListClosedIssues (", data: closed},
		{matchSubstring: "query ListMergedPRs (", data: merged},
		{matchSubstring: "query ListRepoIssues (", data: open},
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

	items := map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{
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
					"field":      map[string]any{"id": "F_S", "name": "Status"},
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
					"field":      map[string]any{"id": "F_S", "name": "Status"},
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
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetViewerLogin", data: map[string]any{"viewer": map[string]any{"login": "alice"}}},
		{matchSubstring: "query GetOrgProjectV2 (", data: orgProject("PVT_org")},
		{matchSubstring: "query ListProjectV2Items(", data: items},
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

func TestReview_PeriodDaily(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query ListClosedIssues (",
			data:           map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{}}}},
		},
		{
			matchSubstring: "query ListMergedPRs (",
			data:           map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": []any{}}}},
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
	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query ListClosedIssues (",
			data:           map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{}}}},
		},
		{
			matchSubstring: "query ListMergedPRs (",
			data:           map[string]any{"repository": map[string]any{"pullRequests": map[string]any{"nodes": []any{}}}},
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

	items := map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{
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
					"field":      map[string]any{"id": "F_S", "name": "Status"},
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
					"field":      map[string]any{"id": "F_S", "name": "Status"},
				},
			}},
		},
	}}}}
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetOrgProjectV2 (", data: orgProject("PVT_org")},
		{matchSubstring: "query ListProjectV2Items(", data: items},
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

	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetUserProjectV2 (", data: userProject("PVT_user")},
		{matchSubstring: "query ListProjectV2Items(", data: map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{}}}}},
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

	g := &fakeGraphQL{}
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
	if !strings.Contains(stderr.String(), "/tmp/cfg.toml") {
		t.Errorf("expected localized config path on stderr, got:\n%s", stderr.String())
	}
}

func TestList_ScopeFlagInvalid(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.Argv = []string{"gh-tasks", "list", "--scope=bogus"}
	})
	_, stderr, err := runCmd(t, d, "list", "--scope=bogus")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for invalid scope, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Invalid --scope value") {
		t.Errorf("expected scope.invalid message, got:\n%s", stderr.String())
	}
}

func TestList_RepoNotResolved(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return true }
		d.GetRemoteURL = func() (string, bool) { return "", false }
	})
	_, stderr, err := runCmd(t, d, "list")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for unresolved repo, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Could not resolve a repository") {
		t.Errorf("expected repo.notResolved message, got:\n%s", stderr.String())
	}
}

func TestList_ProjectFlagInvalid(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.Argv = []string{"gh-tasks", "list", "--scope=org", "--project=bogus"}
	})
	_, stderr, err := runCmd(t, d, "list", "--scope=org", "--project=bogus")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for invalid project, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Invalid --project value") {
		t.Errorf("expected project.invalidIdentifier message, got:\n%s", stderr.String())
	}
}

func TestReview_PeriodFlagInvalid(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "review", "--period", "yearly")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for invalid period, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Invalid --period value") {
		t.Errorf("expected period.invalid message, got:\n%s", stderr.String())
	}
}

// ===== Add =================================================================

func TestAdd_RepoCreatesIssue(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query GetRepositoryID (",
			data:           map[string]any{"repository": map[string]any{"id": "R_1"}},
		},
		{
			matchSubstring: "mutation CreateIssue(",
			data: map[string]any{"createIssue": map[string]any{"issue": map[string]any{
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
	inner := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetRepositoryID (", data: map[string]any{"repository": map[string]any{"id": "R_1"}}},
		{
			matchSubstring: "mutation CreateIssue(",
			data: map[string]any{"createIssue": map[string]any{"issue": map[string]any{
				"id": "I_new", "number": 124, "url": "u/124",
			}}},
		},
	}}
	wrap := &captureGraphQL{inner: inner, capture: func(query string, vars map[string]any) {
		if strings.Contains(query, "mutation CreateIssue(") {
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

	g := &fakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "add")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Title is required") {
		t.Errorf("expected titleRequired message, got:\n%s", stderr.String())
	}
}

func TestAdd_ProjectDraftItem(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetUserProjectV2 (", data: userProject("PVT_user")},
		{
			matchSubstring: "mutation AddProjectV2DraftIssue(",
			data:           map[string]any{"addProjectV2DraftIssue": map[string]any{"projectItem": map[string]any{"id": "DI_new"}}},
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

	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetRepositoryID (", data: map[string]any{"repository": nil}},
	}}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "add", "Title")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Repository not found") {
		t.Errorf("expected repo.notFound message, got:\n%s", stderr.String())
	}
}

// ===== Done ================================================================

func TestDone_RepoCloses(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query GetIssueByNumber (",
			data: map[string]any{"repository": map[string]any{"issue": map[string]any{
				"id": "I_open", "number": 7, "url": "u/7", "state": "OPEN",
			}}},
		},
		{
			matchSubstring: "mutation CloseIssue(",
			data: map[string]any{"closeIssue": map[string]any{"issue": map[string]any{
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

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query GetIssueByNumber (",
			data: map[string]any{"repository": map[string]any{"issue": map[string]any{
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

	fields := map[string]any{"node": map[string]any{"fields": map[string]any{"nodes": []any{
		map[string]any{
			"id": "F_STATUS", "name": "Status", "dataType": "SINGLE_SELECT",
			"options": []any{
				map[string]any{"id": "OPT_TODO", "name": "Todo"},
				map[string]any{"id": "OPT_DONE", "name": "Done"},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{
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
					"field":      map[string]any{"id": "F_STATUS", "name": "Status"},
				},
			}},
		},
	}}}}
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetUserProjectV2 (", data: userProject("PVT_user")},
		{matchSubstring: "query ListProjectV2Fields(", data: fields},
		{matchSubstring: "query ListProjectV2Items(", data: items},
		{
			matchSubstring: "mutation UpdateProjectV2ItemFieldValue(",
			data:           map[string]any{"updateProjectV2ItemFieldValue": map[string]any{"projectV2Item": map[string]any{"id": "ITEM_X"}}},
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

	fields := map[string]any{"node": map[string]any{"fields": map[string]any{"nodes": []any{
		map[string]any{
			"id": "F_STATUS", "name": "Status", "dataType": "SINGLE_SELECT",
			"options": []any{
				map[string]any{"id": "OPT_TODO", "name": "Todo"},
				map[string]any{"id": "OPT_DONE", "name": "Done"},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{
		map[string]any{
			"id": "ITEM_X", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{"__typename": "Issue", "id": "I_xx", "number": 1, "title": "x", "url": "u/x"},
			"fieldValues": map[string]any{"nodes": []any{
				map[string]any{
					"__typename": "ProjectV2ItemFieldSingleSelectValue",
					"optionId":   "OPT_DONE",
					"name":       "Done",
					"field":      map[string]any{"id": "F_STATUS", "name": "Status"},
				},
			}},
		},
	}}}}
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetUserProjectV2 (", data: userProject("PVT_user")},
		{matchSubstring: "query ListProjectV2Fields(", data: fields},
		{matchSubstring: "query ListProjectV2Items(", data: items},
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

	fields := map[string]any{"node": map[string]any{"fields": map[string]any{"nodes": []any{
		map[string]any{"id": "F_OTHER", "name": "Priority", "dataType": "SINGLE_SELECT", "options": []any{}},
	}}}}
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetUserProjectV2 (", data: userProject("PVT_user")},
		{matchSubstring: "query ListProjectV2Fields(", data: fields},
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
	if !strings.Contains(stderr.String(), "no `Status` field") {
		t.Errorf("expected statusFieldMissing message, got:\n%s", stderr.String())
	}
}

func TestDone_ProjectMissingDoneOption(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"fields": map[string]any{"nodes": []any{
		map[string]any{
			"id": "F_S", "name": "Status", "dataType": "SINGLE_SELECT",
			"options": []any{map[string]any{"id": "OPT_TODO", "name": "Todo"}},
		},
	}}}}
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetUserProjectV2 (", data: userProject("PVT_user")},
		{matchSubstring: "query ListProjectV2Fields(", data: fields},
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
	if !strings.Contains(stderr.String(), "no `Done` option") {
		t.Errorf("expected doneOptionMissing message, got:\n%s", stderr.String())
	}
}

// ===== Link ================================================================

func TestLink_RepoAppendsClosesLink(t *testing.T) {
	t.Parallel()

	var seenBody string
	inner := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query GetPullRequestByNumber (",
			data: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 12, "url": "https://github.com/ozzy-labs/gh-tasks/pull/12", "body": "Initial summary",
			}}},
		},
		{
			matchSubstring: "mutation UpdatePullRequest(",
			data: map[string]any{"updatePullRequest": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 12, "url": "https://github.com/ozzy-labs/gh-tasks/pull/12",
			}}},
		},
	}}
	wrap := &captureGraphQL{inner: inner, capture: func(query string, vars map[string]any) {
		if strings.Contains(query, "mutation UpdatePullRequest(") {
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

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query GetPullRequestByNumber (",
			data: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
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
	inner := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetOrgProjectV2 (", data: orgProject("PVT_org")},
		{
			matchSubstring: "query GetPullRequestByNumber (",
			data: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 12, "url": "u/12", "body": "",
			}}},
		},
		{
			matchSubstring: "query GetIssueByNumber (",
			data: map[string]any{"repository": map[string]any{"issue": map[string]any{
				"id": "I_42", "number": 42, "url": "u/42", "state": "OPEN",
			}}},
		},
		{
			matchSubstring: "mutation AddProjectV2ItemById(",
			data:           map[string]any{"addProjectV2ItemById": map[string]any{"item": map[string]any{"id": "PI_pr"}}},
		},
		{
			matchSubstring: "mutation AddProjectV2ItemById(",
			data:           map[string]any{"addProjectV2ItemById": map[string]any{"item": map[string]any{"id": "PI_iss"}}},
		},
	}}
	wrap := &captureGraphQL{inner: inner, capture: func(query string, _ map[string]any) {
		if strings.Contains(query, "mutation AddProjectV2ItemById(") {
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

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query ListRepoIssuesWithMilestone (",
			data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
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
	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query ListRepoIssuesWithMilestone (",
			data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{
					"id": "I_a", "number": 1, "title": "Task A", "url": "u/1",
					"updatedAt": "2026-05-04T08:00:00Z",
					"milestone": nil,
				},
			}}}},
		},
		{
			matchSubstring: "query ListMilestones (",
			data: map[string]any{"repository": map[string]any{"milestones": map[string]any{"nodes": []any{
				map[string]any{"id": "M_1", "number": 5, "title": "Daily 2026-05-04"},
			}}}},
		},
		{
			matchSubstring: "mutation UpdateIssueMilestone(",
			data: map[string]any{"updateIssue": map[string]any{"issue": map[string]any{
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
	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query ListRepoIssuesWithMilestone (",
			data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{
					"id": "I_a", "number": 1, "title": "Task A", "url": "u/1",
					"updatedAt": "2026-05-04T08:00:00Z",
					"milestone": nil,
				},
			}}}},
		},
		{
			matchSubstring: "query ListMilestones (",
			data:           map[string]any{"repository": map[string]any{"milestones": map[string]any{"nodes": []any{}}}},
		},
		{
			matchSubstring: "mutation UpdateIssueMilestone(",
			data: map[string]any{"updateIssue": map[string]any{"issue": map[string]any{
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

	fields := map[string]any{"node": map[string]any{"fields": map[string]any{"nodes": []any{
		map[string]any{
			"id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations": []any{
					map[string]any{"id": "IT_1", "title": "Daily 2026-05-04", "startDate": "2026-05-04", "duration": 1},
				},
				"completedIterations": []any{},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{}}}}
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetOrgProjectV2 (", data: orgProject("PVT_org")},
		{matchSubstring: "query ListProjectV2Fields(", data: fields},
		{matchSubstring: "query ListProjectV2Items(", data: items},
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

	fields := map[string]any{"node": map[string]any{"fields": map[string]any{"nodes": []any{
		map[string]any{
			"id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations": []any{
					// Title doesn't match Daily target, but covers now (2026-05-04)
					map[string]any{"id": "IT_C", "title": "Sprint A", "startDate": "2026-05-01", "duration": 14},
				},
				"completedIterations": []any{},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{}}}}
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetOrgProjectV2 (", data: orgProject("PVT_org")},
		{matchSubstring: "query ListProjectV2Fields(", data: fields},
		{matchSubstring: "query ListProjectV2Items(", data: items},
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
	if !strings.Contains(stdout.String(), "falling back") && !strings.Contains(stderr.String(), "falling back") {
		t.Errorf("expected fallback note in stdout/stderr, stdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
}

// ===== Triage ==============================================================

func TestTriage_RepoUnlabelledOnly(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query ListRepoIssuesWithLabels (",
			data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
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

	items := map[string]any{"node": map[string]any{"items": map[string]any{"nodes": []any{
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
					"field":      map[string]any{"id": "F_S", "name": "Status"},
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
					"field":      map[string]any{"id": "F_S", "name": "Status"},
				},
			}},
		},
	}}}}
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetUserProjectV2 (", data: userProject("PVT_user")},
		{matchSubstring: "query ListProjectV2Items(", data: items},
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

	g := &fakeGraphQL{}
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

	g := &fakeGraphQL{}
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

	g := &fakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--template", "user")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), "--title is required") {
		t.Errorf("expected titleRequired error, got:\n%s", stderr.String())
	}
}

func TestProjectsInit_TemplateRequired(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--title", "Foo")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Either --template") {
		t.Errorf("expected templateRequired error, got:\n%s", stderr.String())
	}
}

func TestProjectsInit_TemplateConflict(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "x", "/tmp/some.yaml")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), "cannot be combined") {
		t.Errorf("expected templateConflict error, got:\n%s", stderr.String())
	}
}

func TestProjectsInit_OwnerNotFound(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query GetOwnerID (", data: map[string]any{"repositoryOwner": nil}},
	}}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "x", "--owner", "ghost")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Could not resolve owner") {
		t.Errorf("expected ownerNotFound error, got:\n%s", stderr.String())
	}
}

// ===== Helpers =============================================================

// captureGraphQL wraps a fakeGraphQL to peek at queries / vars without
// disturbing the canned-response replay.
type captureGraphQL struct {
	inner   *fakeGraphQL
	capture func(query string, vars map[string]any)
}

func (c *captureGraphQL) Do(ctx context.Context, query string, vars map[string]any, out any) error {
	if c.capture != nil {
		c.capture(query, vars)
	}
	return c.inner.Do(ctx, query, vars, out)
}
