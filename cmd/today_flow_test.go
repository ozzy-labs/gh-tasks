// Cobra-rooted flow tests for the `today` command. See
// `docs/design/test-structure.md` for rationale and the `Test<Cmd>_<Scenario>`
// naming convention. Shared helpers live in `testhelpers_test.go`.
package cmd_test

import (
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

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

// TestToday_JSONFields pins that --json on `today` reuses the shared item
// catalog (id / number / title / type / updatedAt / url) and applies the
// same UTC-day filter as the text path. Only the today-dated entry must
// surface in the JSON array; the yesterday / tomorrow rows from the fake
// payload are filtered out before the DTO is constructed.
func TestToday_JSONFields(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssues (",
			Data: repoIssuesPayload(
				map[string]any{"id": "I_old", "number": 1, "title": "Yesterday", "url": "u1", "updatedAt": "2026-05-03T08:00:00Z"},
				map[string]any{"id": "I_today", "number": 2, "title": "Today", "url": "u2", "updatedAt": "2026-05-04T08:00:00Z"},
			),
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "today", "--json", "id,number,title,type")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	assertJSONLength(t, stdout.String(), 1)
	assertJSONFieldEquals(t, stdout.String(), 0, "id", "I_today")
	assertJSONFieldEquals(t, stdout.String(), 0, "number", 2)
	assertJSONFieldEquals(t, stdout.String(), 0, "title", "Today")
	assertJSONFieldEquals(t, stdout.String(), 0, "type", "ISSUE")
}
