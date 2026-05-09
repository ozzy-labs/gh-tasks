// Cobra-rooted flow tests for the `add` command. See
// `docs/design/test-structure.md` for rationale and the `Test<Cmd>_<Scenario>`
// naming convention. Shared helpers live in `testhelpers_test.go`.
package cmd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

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

func TestAdd_OrgProjectDraftItem(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{
			MatchSubstring: "mutation AddProjectV2DraftIssue (",
			Data:           map[string]any{"addProjectV2DraftIssue": map[string]any{"projectItem": map[string]any{"id": "DI_org"}}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "add", "Org idea", "--scope=org")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Draft item added to project") {
		t.Errorf("expected add.created.project prefix, got:\n%s", got)
	}
	if !strings.Contains(got, "DI_org") {
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

// TestAdd_JSONRepoCreated pins the --json output for the repo path: a
// single-element JSON array carrying the created Issue's id / number /
// title / type / updatedAt / url. After PR 7 of #376 the CreateIssue
// mutation now returns updatedAt, so the contract carries a real RFC
// 3339 timestamp instead of null.
func TestAdd_JSONRepoCreated(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query GetRepositoryID (",
			Data:           map[string]any{"repository": map[string]any{"id": "R_1"}},
		},
		{
			MatchSubstring: "mutation CreateIssue (",
			Data: map[string]any{"createIssue": map[string]any{"issue": map[string]any{
				"id": "I_new", "number": 123, "title": "Fix login",
				"url":       "https://github.com/ozzy-labs/gh-tasks/issues/123",
				"updatedAt": "2026-05-04T08:00:00Z",
			}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "add", "Fix login", "--json", "id,number,title,type,updatedAt,url")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	assertJSONLength(t, stdout.String(), 1)
	assertJSONFieldEquals(t, stdout.String(), 0, "id", "I_new")
	assertJSONFieldEquals(t, stdout.String(), 0, "number", 123)
	assertJSONFieldEquals(t, stdout.String(), 0, "title", "Fix login")
	assertJSONFieldEquals(t, stdout.String(), 0, "type", "ISSUE")
	assertJSONFieldEquals(t, stdout.String(), 0, "updatedAt", "2026-05-04T08:00:00Z")
	assertJSONFieldEquals(t, stdout.String(), 0, "url", "https://github.com/ozzy-labs/gh-tasks/issues/123")
}

// TestAdd_JSONProjectDraft pins the --json output for the project draft
// path: type is "DRAFT_ISSUE", number is 0, url is "" (zero values
// rather than null because the JSON marshal of int / string defaults to
// those types).
func TestAdd_JSONProjectDraft(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{
			MatchSubstring: "mutation AddProjectV2DraftIssue (",
			Data: map[string]any{"addProjectV2DraftIssue": map[string]any{
				"projectItem": map[string]any{"id": "DI_org"},
			}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "add", "Org idea", "--scope=org", "--json", "id,type,number,url")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	assertJSONLength(t, stdout.String(), 1)
	assertJSONFieldEquals(t, stdout.String(), 0, "id", "DI_org")
	assertJSONFieldEquals(t, stdout.String(), 0, "type", "DRAFT_ISSUE")
	assertJSONFieldEquals(t, stdout.String(), 0, "number", 0)
	assertJSONFieldEquals(t, stdout.String(), 0, "url", "")
}
