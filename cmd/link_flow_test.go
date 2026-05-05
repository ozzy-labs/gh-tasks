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
