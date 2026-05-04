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
)

// fakeGraphQL implements github.GraphQLClient for tests. Each call is matched
// against responses keyed by query substring, in registration order. To
// disambiguate prefix-overlapping operations (e.g. ListRepoIssues vs
// ListRepoIssuesWithLabels), matchSubstring values use the
// `query <Name>(` form.
type fakeGraphQL struct {
	responses []fakeResponse
	idx       int
}

type fakeResponse struct {
	matchSubstring string
	data           any
	err            error
}

func (f *fakeGraphQL) Do(_ context.Context, query string, _ map[string]any, out any) error {
	for i := f.idx; i < len(f.responses); i++ {
		r := f.responses[i]
		if !strings.Contains(query, r.matchSubstring) {
			continue
		}
		f.idx = i + 1
		if r.err != nil {
			return r.err
		}
		buf, err := json.Marshal(r.data)
		if err != nil {
			return fmt.Errorf("marshal fake response: %w", err)
		}
		return json.Unmarshal(buf, out)
	}
	return fmt.Errorf("no fake response matched query: %q", query)
}

type fakeREST struct{}

func (fakeREST) Do(context.Context, string, string, any, any) error { return nil }

func newClients(g *fakeGraphQL) *github.Clients {
	return &github.Clients{Host: "github.com", GraphQL: g, REST: fakeREST{}}
}

func testDeps(g *fakeGraphQL, opts ...func(*cmd.Deps)) cmd.Deps {
	d := cmd.Deps{
		Stdout:       new(bytes.Buffer),
		Stderr:       new(bytes.Buffer),
		Now:          func() time.Time { return time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC) },
		Env:          func(string) string { return "" },
		HasGitRemote: func() bool { return true },
		GetRemoteURL: func() (string, bool) { return "git@github.com:ozzy-labs/gh-tasks.git", true },
		NewClients:   func() (*github.Clients, error) { return newClients(g), nil },
		LoadConfig:   func() (config.AppConfig, error) { return config.AppConfig{}, nil },
	}
	for _, opt := range opts {
		opt(&d)
	}
	return d
}

func TestList_RepoEmpty(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query ListRepoIssues (",
			data: map[string]any{
				"repository": map[string]any{
					"issues": map[string]any{"nodes": []any{}},
				},
			},
		},
	}}
	out := new(bytes.Buffer)
	deps := testDeps(g, func(d *cmd.Deps) { d.Stdout = out })
	root := cmd.RootWithDeps(deps)
	root.SetArgs([]string{"list"})
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), "No open issues") {
		t.Errorf("missing empty-state message:\n%s", out.String())
	}
}

func TestList_RepoIssues(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query ListRepoIssues (",
			data: map[string]any{
				"repository": map[string]any{
					"issues": map[string]any{
						"nodes": []any{
							map[string]any{
								"id":        "I_1",
								"number":    42,
								"title":     "Fix login",
								"url":       "https://example.com/i/42",
								"updatedAt": "2026-05-04T08:00:00Z",
							},
						},
					},
				},
			},
		},
	}}
	out := new(bytes.Buffer)
	deps := testDeps(g, func(d *cmd.Deps) { d.Stdout = out })
	root := cmd.RootWithDeps(deps)
	root.SetArgs([]string{"list"})
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "#42") || !strings.Contains(got, "Fix login") {
		t.Errorf("missing expected output:\n%s", got)
	}
}

func TestToday_FiltersByUTC(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query ListRepoIssues (",
			data: map[string]any{
				"repository": map[string]any{
					"issues": map[string]any{
						"nodes": []any{
							map[string]any{"id": "I_old", "number": 1, "title": "Yesterday", "url": "u1", "updatedAt": "2026-05-03T08:00:00Z"},
							map[string]any{"id": "I_today", "number": 2, "title": "Today", "url": "u2", "updatedAt": "2026-05-04T08:00:00Z"},
							map[string]any{"id": "I_tomorrow", "number": 3, "title": "Tomorrow", "url": "u3", "updatedAt": "2026-05-05T08:00:00Z"},
						},
					},
				},
			},
		},
	}}
	out := new(bytes.Buffer)
	deps := testDeps(g, func(d *cmd.Deps) { d.Stdout = out })
	root := cmd.RootWithDeps(deps)
	root.SetArgs([]string{"today"})
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Today") {
		t.Errorf("expected Today entry, got:\n%s", got)
	}
	if strings.Contains(got, "Yesterday") || strings.Contains(got, "Tomorrow") {
		t.Errorf("expected only today's entries, got:\n%s", got)
	}
}

func TestReview_RepoMarkdown(t *testing.T) {
	t.Parallel()

	g := &fakeGraphQL{responses: []fakeResponse{
		{
			matchSubstring: "query ListClosedIssues (",
			data: map[string]any{
				"repository": map[string]any{
					"issues": map[string]any{
						"nodes": []any{
							map[string]any{"id": "I_x", "number": 7, "title": "Fix bug", "url": "u", "closedAt": "2026-05-04T09:00:00Z"},
						},
					},
				},
			},
		},
		{
			matchSubstring: "query ListMergedPRs (",
			data: map[string]any{
				"repository": map[string]any{
					"pullRequests": map[string]any{"nodes": []any{}},
				},
			},
		},
	}}
	out := new(bytes.Buffer)
	deps := testDeps(g, func(d *cmd.Deps) { d.Stdout = out })
	root := cmd.RootWithDeps(deps)
	root.SetArgs([]string{"review", "--period", "weekly"})
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Closed Issues") || !strings.Contains(got, "#7") {
		t.Errorf("expected closed issues section, got:\n%s", got)
	}
}

func TestStandup_RepoStructure(t *testing.T) {
	t.Parallel()

	emptyRepoIssues := map[string]any{
		"repository": map[string]any{"issues": map[string]any{"nodes": []any{}}},
	}
	emptyPRs := map[string]any{
		"repository": map[string]any{"pullRequests": map[string]any{"nodes": []any{}}},
	}
	g := &fakeGraphQL{responses: []fakeResponse{
		{matchSubstring: "query ListClosedIssues (", data: emptyRepoIssues},
		{matchSubstring: "query ListMergedPRs (", data: emptyPRs},
		{matchSubstring: "query ListRepoIssues (", data: emptyRepoIssues},
	}}
	out := new(bytes.Buffer)
	deps := testDeps(g, func(d *cmd.Deps) { d.Stdout = out })
	root := cmd.RootWithDeps(deps)
	root.SetArgs([]string{"standup"})
	root.SetOut(out)
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	for _, want := range []string{"# Standup", "## Yesterday", "## Today", "## Blockers"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// silentNoErr asserts an error is the silent sentinel; primarily for type
// linting in this test file.
var _ = errors.New
