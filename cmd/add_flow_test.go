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
