// Cobra-rooted flow tests for the `triage` command. Shared helpers live
// in `testhelpers_test.go`. See `docs/design/test-structure.md` for
// rationale and the `Test<Cmd>_<Scenario>` naming convention.
package cmd_test

import (
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

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
