// Cobra-rooted flow tests for the `plan` command. Repo-side milestone
// cases live alongside project-side iteration cases; helper-level plan
// formatters are covered by `plan_test.go`. Shared helpers live in
// `testhelpers_test.go`. See `docs/design/test-structure.md` for rationale.
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

func TestPlan_RepoPreviewDaily(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssuesWithMilestone (",
			Data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{
					"id": "I_a", "number": 1, "title": "Task A", "url": "u/1",
					"updatedAt": "2026-05-04T08:00:00Z",
					"milestone": nil,
				},
			}}}},
		},
	}}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "plan", "--period", "daily")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Proposed milestone") {
		t.Errorf("expected plan.proposed prefix, got:\n%s", got)
	}
	if !strings.Contains(got, "Daily 2026-05-04") {
		t.Errorf("expected daily milestone title, got:\n%s", got)
	}
	if !strings.Contains(got, "--write") {
		t.Errorf("expected preview note pointing at --write, got:\n%s", got)
	}
}

func TestPlan_RepoReuseExistingMilestone(t *testing.T) {
	t.Parallel()

	rest := &recordingREST{}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssuesWithMilestone (",
			Data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{
					"id": "I_a", "number": 1, "title": "Task A", "url": "u/1",
					"updatedAt": "2026-05-04T08:00:00Z",
					"milestone": nil,
				},
			}}}},
		},
		{
			MatchSubstring: "query ListMilestones (",
			Data: map[string]any{"repository": map[string]any{"milestones": map[string]any{"nodes": []any{
				map[string]any{"id": "M_1", "number": 5, "title": "Daily 2026-05-04"},
			}}}},
		},
		{
			MatchSubstring: "mutation UpdateIssueMilestone (",
			Data: map[string]any{"updateIssue": map[string]any{"issue": map[string]any{
				"id": "I_a", "number": 1, "url": "u/1",
				"milestone": map[string]any{"id": "M_1", "number": 5, "title": "Daily 2026-05-04"},
			}}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return newClientsWithREST(g, rest), nil
		}
	})
	stdout, _, err := runCmd(t, d, "plan", "--period", "daily", "--write")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Reused an existing milestone") {
		t.Errorf("expected plan.reused, got:\n%s", got)
	}
	if !strings.Contains(got, "Issue bound to milestone") {
		t.Errorf("expected plan.linked, got:\n%s", got)
	}
	for _, c := range rest.calls {
		if c.method == "POST" && strings.Contains(c.path, "/milestones") {
			t.Errorf("did not expect REST milestone create when reusing, saw call: %+v", c)
		}
	}
	assertNoLeadingOrDoubleSlashInRESTPath(t, rest)
}

func TestPlan_RepoCreateNewMilestone(t *testing.T) {
	t.Parallel()

	rest := &recordingREST{responses: []restResponse{
		{
			matchMethod: "POST",
			matchPath:   "/milestones",
			data:        map[string]any{"node_id": "M_NEW", "id": 999, "number": 12, "title": "Daily 2026-05-04"},
		},
	}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssuesWithMilestone (",
			Data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{
					"id": "I_a", "number": 1, "title": "Task A", "url": "u/1",
					"updatedAt": "2026-05-04T08:00:00Z",
					"milestone": nil,
				},
			}}}},
		},
		{
			MatchSubstring: "query ListMilestones (",
			Data:           map[string]any{"repository": map[string]any{"milestones": map[string]any{"nodes": []any{}}}},
		},
		{
			MatchSubstring: "mutation UpdateIssueMilestone (",
			Data: map[string]any{"updateIssue": map[string]any{"issue": map[string]any{
				"id": "I_a", "number": 1, "url": "u/1",
				"milestone": map[string]any{"id": "M_NEW", "number": 12, "title": "Daily 2026-05-04"},
			}}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return newClientsWithREST(g, rest), nil
		}
	})
	stdout, _, err := runCmd(t, d, "plan", "--period", "daily", "--write")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "Milestone created") {
		t.Errorf("expected plan.created, got:\n%s", stdout.String())
	}
	sawCreate := false
	for _, c := range rest.calls {
		if c.method == "POST" && strings.Contains(c.path, "/milestones") {
			sawCreate = true
		}
	}
	if !sawCreate {
		t.Errorf("expected REST POST /milestones; calls=%+v", rest.calls)
	}
	assertNoLeadingOrDoubleSlashInRESTPath(t, rest)
}

// TestPlan_CreateMilestoneRESTPathFormat asserts the exact REST path format
// emitted by plan when creating a milestone. Regression guard for the
// `https://api.github.com//repos/...` HTTP 404 bug caused by a leading "/"
// in the path passed to go-gh's RESTClient (see internal/github/github.go
// RESTClient docstring).
func TestPlan_CreateMilestoneRESTPathFormat(t *testing.T) {
	t.Parallel()

	rest := &recordingREST{responses: []restResponse{
		{
			matchMethod: "POST",
			matchPath:   "milestones",
			data:        map[string]any{"node_id": "M_NEW", "id": 999, "number": 12, "title": "Daily 2026-05-04"},
		},
	}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query ListRepoIssuesWithMilestone (",
			Data: map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{
				map[string]any{
					"id": "I_a", "number": 1, "title": "Task A", "url": "u/1",
					"updatedAt": "2026-05-04T08:00:00Z",
					"milestone": nil,
				},
			}}}},
		},
		{
			MatchSubstring: "query ListMilestones (",
			Data:           map[string]any{"repository": map[string]any{"milestones": map[string]any{"nodes": []any{}}}},
		},
		{
			MatchSubstring: "mutation UpdateIssueMilestone (",
			Data: map[string]any{"updateIssue": map[string]any{"issue": map[string]any{
				"id": "I_a", "number": 1, "url": "u/1",
				"milestone": map[string]any{"id": "M_NEW", "number": 12, "title": "Daily 2026-05-04"},
			}}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return newClientsWithREST(g, rest), nil
		}
	})
	if _, _, err := runCmd(t, d, "plan", "--period", "daily", "--write"); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var sawCreate bool
	for _, c := range rest.calls {
		if c.method != "POST" || !strings.Contains(c.path, "milestones") {
			continue
		}
		sawCreate = true
		// Exact format: repos/{owner}/{name}/milestones (no leading slash).
		// testDeps wires a default repo; we check the expected suffix and the
		// no-leading-slash invariant rather than hard-coding owner/name.
		if !strings.HasSuffix(c.path, "/milestones") {
			t.Errorf("REST POST path = %q; want suffix %q", c.path, "/milestones")
		}
		if strings.HasPrefix(c.path, "/") {
			t.Errorf("REST POST path must not start with %q; got %q", "/", c.path)
		}
		if strings.Contains(c.path, "//") {
			t.Errorf("REST POST path must not contain %q; got %q", "//", c.path)
		}
		if !strings.HasPrefix(c.path, "repos/") {
			t.Errorf("REST POST path = %q; want prefix %q", c.path, "repos/")
		}
	}
	if !sawCreate {
		t.Errorf("expected REST POST .../milestones; calls=%+v", rest.calls)
	}
	assertNoLeadingOrDoubleSlashInRESTPath(t, rest)
}

func TestPlan_ProjectIterationMatched(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations": []any{
					map[string]any{"id": "IT_1", "title": "Daily 2026-05-04", "startDate": "2026-05-04", "duration": 1},
				},
				"completedIterations": []any{},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Matched an existing iteration") {
		t.Errorf("expected plan.iterationMatched.project, got:\n%s", got)
	}
}

func TestPlan_ProjectIterationFallback(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations": []any{
					// Title doesn't match Daily target, but covers now (2026-05-04)
					map[string]any{"id": "IT_C", "title": "Sprint A", "startDate": "2026-05-01", "duration": 14},
				},
				"completedIterations": []any{},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, stderr, err := runCmd(t, d, "plan", "--scope=org", "--period=daily")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := i18n.T(i18n.LocaleEN, "plan.iterationFallback.project")
	if !strings.Contains(stdout.String(), want) && !strings.Contains(stderr.String(), want) {
		t.Errorf("expected fallback note %q in stdout/stderr, stdout=%s\nstderr=%s",
			want, stdout.String(), stderr.String())
	}
}

// TestPlan_ProjectIterationUpdated covers the write half of `runPlanProject`
// with --write: when an in-range item is *not* already on the target
// iteration, the command issues an UpdateProjectV2ItemFieldValue mutation
// and prints `plan.iterationUpdated.project`. The test wires a
// captureGraphQL around the fake to assert the mutation input shape (project
// id, item id, field id, iteration id) instead of just the human output, so
// a future refactor that swaps the mutation but keeps the message can't
// silently regress.
func TestPlan_ProjectIterationUpdated(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations": []any{
					map[string]any{"id": "IT_1", "title": "Daily 2026-05-04", "startDate": "2026-05-04", "duration": 1},
				},
				"completedIterations": []any{},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		map[string]any{
			"id": "ITEM_a", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_a", "number": 11, "title": "Wire iteration", "url": "u/11",
			},
			"fieldValues": map[string]any{"nodes": []any{}},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
		{
			MatchSubstring: "mutation UpdateProjectV2ItemFieldValue (",
			Data: map[string]any{"updateProjectV2ItemFieldValue": map[string]any{"projectV2Item": map[string]any{
				"id": "ITEM_a",
			}}},
		},
	}}
	var capturedInput map[string]any
	wrap := &captureGraphQL{inner: g, capture: func(query string, vars map[string]any) {
		if !strings.Contains(query, "mutation UpdateProjectV2ItemFieldValue (") {
			return
		}
		if in, ok := vars["input"].(map[string]any); ok {
			capturedInput = in
		}
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily", "--write")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Project item iteration updated") {
		t.Errorf("expected plan.iterationUpdated.project on stdout, got:\n%s", got)
	}
	if !strings.Contains(got, "#11") {
		t.Errorf("expected updated-item label `#11`, got:\n%s", got)
	}
	if capturedInput == nil {
		t.Fatalf("expected UpdateProjectV2ItemFieldValue mutation to be invoked, captured none")
	}
	if capturedInput["projectId"] != "PVT_org" {
		t.Errorf("input.projectId = %v, want PVT_org", capturedInput["projectId"])
	}
	if capturedInput["itemId"] != "ITEM_a" {
		t.Errorf("input.itemId = %v, want ITEM_a", capturedInput["itemId"])
	}
	if capturedInput["fieldId"] != "F_IT" {
		t.Errorf("input.fieldId = %v, want F_IT", capturedInput["fieldId"])
	}
	value, _ := capturedInput["value"].(map[string]any)
	if value == nil || value["iterationId"] != "IT_1" {
		t.Errorf("input.value.iterationId mismatch, got value=%v", value)
	}
}

// TestPlan_ProjectAlreadyOnIteration covers the skip branch: when an
// in-range item already carries the resolved iteration id on the iteration
// field, `runPlanProject` must NOT call UpdateProjectV2ItemFieldValue and
// must surface `plan.iterationAlreadySet.project`. The fake registers no
// mutation response, so an accidental call would surface as `no fake
// response matched` and fail the test loudly.
func TestPlan_ProjectAlreadyOnIteration(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations": []any{
					map[string]any{"id": "IT_1", "title": "Daily 2026-05-04", "startDate": "2026-05-04", "duration": 1},
				},
				"completedIterations": []any{},
			},
		},
	}}}}
	items := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "items": map[string]any{"nodes": []any{
		map[string]any{
			"id": "ITEM_b", "updatedAt": "2026-05-04T08:00:00Z",
			"content": map[string]any{
				"__typename": "Issue", "id": "I_b", "number": 22, "title": "Already wired", "url": "u/22",
			},
			"fieldValues": map[string]any{"nodes": []any{
				map[string]any{
					"__typename":  "ProjectV2ItemFieldIterationValue",
					"iterationId": "IT_1",
					"title":       "Daily 2026-05-04",
					"startDate":   "2026-05-04",
					"duration":    1,
					"field": map[string]any{
						"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration",
					},
				},
			}},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Data: items},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily", "--write")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Skipped (already on the target iteration)") {
		t.Errorf("expected plan.iterationAlreadySet.project, got:\n%s", got)
	}
	if !strings.Contains(got, "#22") {
		t.Errorf("expected describeItem `#22`, got:\n%s", got)
	}
}

// TestPlan_ProjectNotFound covers the project-resolution failure path: when
// PaginateProjectV2Fields returns ErrProjectNotFound (e.g. project deleted
// after the initial id resolve), `runPlanProject` must surface the
// localized `error.project.notFound` on stderr and return ErrSilent so the
// CLI exits with a clean code. The mutation responses are intentionally
// absent.
func TestPlan_ProjectNotFound(t *testing.T) {
	t.Parallel()

	// First the project resolves fine, then ListProjectV2Fields returns a
	// node payload without the ProjectV2 inline fragment; the paginator
	// translates that into ErrProjectNotFound, exercising the
	// errors.Is(err, queries.ErrProjectNotFound) branch in runPlanProject.
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{
			MatchSubstring: "query ListProjectV2Fields (",
			// Wrong concrete type triggers ErrProjectNotFound inside the
			// ProjectV2-typed paginator.
			Data: map[string]any{"node": map[string]any{"__typename": "Issue"}},
		},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, stderr, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.project.notFound", "owner", "octo", "number", 7, "scope", "org")
}

// TestPlan_ProjectIterationFieldMissing exercises the
// `error.plan.iterationFieldMissing` branch: a project whose fields list
// carries no ITERATION-typed entry must surface the localized message and
// return ErrSilent without ever calling the items query.
func TestPlan_ProjectIterationFieldMissing(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2SingleSelectField", "id": "F_S", "name": "Status", "dataType": "SINGLE_SELECT",
			"options": []any{},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, stderr, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.plan.iterationFieldMissing")
}

// TestPlan_ProjectNoIterationsAvailable covers the second mid-flow guard:
// the iteration field exists but its configuration carries an empty
// iterations list (resolveTargetIteration returns nil). The command must
// emit `error.plan.noIterationsAvailable` and return ErrSilent.
func TestPlan_ProjectNoIterationsAvailable(t *testing.T) {
	t.Parallel()

	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2IterationField", "id": "F_IT", "name": "Iteration", "dataType": "ITERATION",
			"configuration": map[string]any{
				"iterations":          []any{},
				"completedIterations": []any{},
			},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, stderr, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.plan.noIterationsAvailable")
}
