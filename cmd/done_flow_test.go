// Cobra-rooted flow tests for the `done` command. See
// `docs/design/test-structure.md` for rationale and the `Test<Cmd>_<Scenario>`
// naming convention. Shared helpers live in `testhelpers_test.go`.
package cmd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

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

// TestDone_ProjectSearchLimitHint exercises the `len(itemList) >= doneItemsLimit`
// branch in runDoneProject (cmd/done.go:158): when the linear search fills the
// page-size cap of 100 items and the target id is not among them, the user
// gets the searchLimit hint instead of plain notFound, so they understand the
// id might exist beyond the bounded scan.
func TestDone_ProjectSearchLimitHint(t *testing.T) {
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
	nodes := make([]any, 100)
	for i := range nodes {
		nodes[i] = map[string]any{
			"id": fmt.Sprintf("ITEM_%03d", i), "updatedAt": "2026-05-04T08:00:00Z",
			"content":     map[string]any{"__typename": "Issue", "id": fmt.Sprintf("I_%03d", i), "number": i + 1, "title": "x", "url": "u/x"},
			"fieldValues": map[string]any{"nodes": []any{}},
		}
	}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{
		"nodes":    nodes,
		"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
	}}}
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
	_, stderr, err := runCmd(t, d, "done", "ITEM_MISSING", "--scope=user")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.done.searchLimit", "id", "ITEM_MISSING", "limit", 100)
}

// TestDone_JSONRepoClosed pins the --json output for the repo path: a
// single-element JSON array carrying id / number / state ("CLOSED") /
// type / url. Title is null because GetIssueByNumber does not return
// title (operations.graphql gap; tracked as PR 7 of #376).
func TestDone_JSONRepoClosed(t *testing.T) {
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
	stdout, _, err := runCmd(t, d, "done", "7", "--json", "id,number,state,type,url,title")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	assertJSONLength(t, stdout.String(), 1)
	assertJSONFieldEquals(t, stdout.String(), 0, "id", "I_open")
	assertJSONFieldEquals(t, stdout.String(), 0, "number", 7)
	assertJSONFieldEquals(t, stdout.String(), 0, "state", "CLOSED")
	assertJSONFieldEquals(t, stdout.String(), 0, "type", "ISSUE")
	assertJSONFieldEquals(t, stdout.String(), 0, "title", nil)
}
