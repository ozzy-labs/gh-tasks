// Cobra-rooted flow tests for the `link` command. The helper-level tests
// for ContainsCloseLink / AppendCloseLink live in `link_test.go` (kept
// separate because those are pure-function unit tests, not flow tests).
// Shared helpers live in `testhelpers_test.go`. See
// `docs/design/test-structure.md` for rationale and the
// `Test<Cmd>_<Scenario>` naming convention.
package cmd_test

import (
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

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

// TestLink_JSONProjectBind pins the project-scope --json contract:
// linkType=projectBind plus a linkedTo object carrying the bound
// Issue's id / number / type / url. Mirrors TestLink_ProjectDualAdd
// for the GraphQL fixtures but exercises the JSON path.
func TestLink_JSONProjectBind(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
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
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return true }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "link", "12", "42", "--scope=org", "--json", "id,linkType,linkedTo")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	assertJSONLength(t, stdout.String(), 1)
	assertJSONFieldEquals(t, stdout.String(), 0, "id", "PR_1")
	assertJSONFieldEquals(t, stdout.String(), 0, "linkType", "projectBind")
	rows := parseJSONArray(t, stdout.String())
	linkedTo, _ := rows[0]["linkedTo"].(map[string]any)
	if linkedTo == nil {
		t.Fatalf("expected linkedTo object, got: %v", rows[0]["linkedTo"])
	}
	if linkedTo["id"] != "I_42" || linkedTo["url"] != "u/42" {
		t.Errorf("linkedTo missing id/url; got: %v", linkedTo)
	}
	if got, _ := linkedTo["number"].(float64); int(got) != 42 {
		t.Errorf("linkedTo.number = %v; want 42", linkedTo["number"])
	}
	if linkedTo["type"] != "ISSUE" {
		t.Errorf("linkedTo.type = %v; want ISSUE", linkedTo["type"])
	}
}

// TestLink_JSONRepoCloses pins the --json output for the repo path: a
// single-element JSON array carrying the PR row plus linkType=
// "closesAdded" and a linkedTo object with the target Issue number.
func TestLink_JSONRepoCloses(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query GetPullRequestByNumber (",
			Data: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 12, "url": "https://github.com/ozzy-labs/gh-tasks/pull/12", "body": "Initial",
			}}},
		},
		{
			MatchSubstring: "mutation UpdatePullRequest (",
			Data: map[string]any{"updatePullRequest": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 12, "url": "https://github.com/ozzy-labs/gh-tasks/pull/12",
			}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "link", "12", "42", "--json", "id,number,type,linkType,linkedTo")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	assertJSONLength(t, stdout.String(), 1)
	assertJSONFieldEquals(t, stdout.String(), 0, "id", "PR_1")
	assertJSONFieldEquals(t, stdout.String(), 0, "number", 12)
	assertJSONFieldEquals(t, stdout.String(), 0, "type", "PULL_REQUEST")
	assertJSONFieldEquals(t, stdout.String(), 0, "linkType", "closesAdded")
	// linkedTo is an object — verify the nested number field.
	rows := parseJSONArray(t, stdout.String())
	linkedTo, _ := rows[0]["linkedTo"].(map[string]any)
	if linkedTo == nil {
		t.Fatalf("expected linkedTo object, got: %v", rows[0]["linkedTo"])
	}
	if got, _ := linkedTo["number"].(float64); int(got) != 42 {
		t.Errorf("linkedTo.number = %v; want 42", linkedTo["number"])
	}
}
