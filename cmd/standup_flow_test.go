// Cobra-rooted flow tests for the `standup` command, including the --mine
// viewer-matching matrix that exercises matchesViewerOnItem (cmd/standup.go).
// See `docs/design/test-structure.md` for the rationale and the
// `Test<Cmd>_<Scenario>` naming convention. Shared helpers live in
// `testhelpers_test.go`.
package cmd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

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
