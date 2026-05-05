// Cobra-rooted flow tests for the `review` command. See
// `docs/design/test-structure.md` for rationale and the `Test<Cmd>_<Scenario>`
// naming convention. Shared helpers live in `testhelpers_test.go`.
package cmd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/period"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

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
