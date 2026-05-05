// Transport-error / context-cancellation flow tests.
//
// The companion tests in cmd_flow_test.go exercise happy paths and domain
// "not found" branches. This file pins the *unhappy* surface where the
// paginator (or mutation) bubbles up a raw error from the GraphQL transport
// layer — HTTP 5xx, network failure, an interrupted request — and the cmd
// handler wraps it with `fmt.Errorf("<verb>: %w", err)` before returning.
//
// Two concerns are pinned:
//
//  1. Transport errors propagate as wrapped, non-silent errors so cobra
//     surfaces them via its default Error: prefix (current behaviour, audit
//     finding C-2/C-3 of #264). They must NOT silently classify as
//     ErrSilent — that would suppress the user-visible explanation.
//  2. errors.Is preserves the underlying transport error sentinel (or
//     context.Canceled), so callers can match on it without parsing the
//     human message. This is what lets `main` exit non-zero via cobra.CheckErr
//     and what makes the wrap reversible for downstream consumers.
//
// All cases use the existing `testfake.FakeResponse{Err: ...}` channel of the
// testfake.FakeGraphQL fake; the context-cancellation case additionally wraps the
// fake with `ctxAwareGraphQL` so the paginator's first `Do` observes the
// cancellation directly.
package cmd_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

// ===== Transport error injection ===========================================

// TestList_RepoGraphQL5xx pins the wrap behaviour when the GraphQL transport
// surfaces an HTTP 5xx as a generic error (not the queries.ErrRepoNotFound
// sentinel). `runListRepo` must wrap it with the `list repo issues:` prefix
// so log-greppers and operators can identify which API call failed, while
// keeping the original error reachable via errors.Is.
func TestList_RepoGraphQL5xx(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 502 Bad Gateway")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "list")
	if err == nil {
		t.Fatalf("expected transport error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list repo issues") {
		t.Errorf("expected wrap prefix `list repo issues`, got %q", err.Error())
	}
	// Transport errors must NOT classify as silent — they're raw cobra Errors
	// so the user sees the wrapped explanation. Pinning the negation prevents
	// a future refactor from accidentally swallowing them under ErrSilent*.
	if errors.Is(err, cmd.ErrSilent) {
		t.Errorf("transport error must NOT classify as ErrSilent, got %v", err)
	}
}

// TestStandup_RepoNetworkError pins the same wrap contract for
// `runStandupRepo`. The first paginator call (PaginateClosedIssues) is the
// one we fail; the cmd must surface `list closed issues:` as the wrap
// prefix, matching the literal in standup.go.
func TestStandup_RepoNetworkError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("net: connection refused")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListClosedIssues (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "standup")
	if err == nil {
		t.Fatalf("expected transport error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list closed issues") {
		t.Errorf("expected wrap prefix `list closed issues`, got %q", err.Error())
	}
	if errors.Is(err, cmd.ErrSilent) {
		t.Errorf("transport error must NOT classify as ErrSilent, got %v", err)
	}
}

// TestDone_RepoUpdateMutationError pins the wrap behaviour when the
// CloseIssue mutation fails after the issue lookup succeeded. The wrap
// prefix `close issue:` matches the literal in done.go.
func TestDone_RepoUpdateMutationError(t *testing.T) {
	t.Parallel()

	mutationErr := errors.New("graphql: HTTP 500 Internal Server Error")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query GetIssueByNumber (",
			Data: map[string]any{"repository": map[string]any{"issue": map[string]any{
				"id": "I_open", "number": 7, "url": "u/7", "state": "OPEN",
			}}},
		},
		{MatchSubstring: "mutation CloseIssue (", Err: mutationErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "done", "7")
	if err == nil {
		t.Fatalf("expected mutation error to surface, got nil")
	}
	if !errors.Is(err, mutationErr) {
		t.Errorf("expected errors.Is(err, mutationErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "close issue") {
		t.Errorf("expected wrap prefix `close issue`, got %q", err.Error())
	}
	if errors.Is(err, cmd.ErrSilent) {
		t.Errorf("mutation error must NOT classify as ErrSilent, got %v", err)
	}
}

// TestReview_RepoListErrorPropagates pins the wrap behaviour for
// `runReviewRepo` when the closed-issues listing fails. The wrap prefix
// `list closed issues:` matches the literal in review.go (which mirrors
// standup but lives in a separate handler, so a regression in either is
// distinguishable).
func TestReview_RepoListErrorPropagates(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 503 Service Unavailable")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListClosedIssues (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "review", "--period", "weekly")
	if err == nil {
		t.Fatalf("expected transport error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list closed issues") {
		t.Errorf("expected wrap prefix `list closed issues`, got %q", err.Error())
	}
	if errors.Is(err, cmd.ErrSilent) {
		t.Errorf("transport error must NOT classify as ErrSilent, got %v", err)
	}
}

// TestPlan_RepoMilestonesError pins the wrap behaviour for the second
// paginator call in `runPlanRepo` (PaginateMilestones). The first
// paginator (issues with milestone) succeeds with an in-range candidate so
// the plan reaches the milestones lookup; that lookup then fails. The wrap
// prefix `list milestones:` matches the literal in plan.go.
func TestPlan_RepoMilestonesError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 500 Internal Server Error")
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
		{MatchSubstring: "query ListMilestones (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "plan", "--period", "daily")
	if err == nil {
		t.Fatalf("expected milestones error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list milestones") {
		t.Errorf("expected wrap prefix `list milestones`, got %q", err.Error())
	}
	if errors.Is(err, cmd.ErrSilent) {
		t.Errorf("milestones error must NOT classify as ErrSilent, got %v", err)
	}
}

// TestPlan_ProjectFieldsError pins the wrap behaviour for the project-side
// fields paginator (PaginateProjectV2Fields) inside `runPlanProject`. This
// covers the org/user scope branch that the repo tests above cannot reach.
// The wrap prefix `list project fields:` matches the literal in plan.go.
func TestPlan_ProjectFieldsError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 502 Bad Gateway")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Fields (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, _, err := runCmd(t, d, "plan", "--scope=org", "--period", "daily")
	if err == nil {
		t.Fatalf("expected fields error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list project fields") {
		t.Errorf("expected wrap prefix `list project fields`, got %q", err.Error())
	}
}

// TestList_ProjectItemsError pins the wrap behaviour for the project-side
// items paginator inside `runListProject`. The org project resolution
// succeeds, then `PaginateProjectV2Items` fails — `runListProject` must
// surface `list project items:` as the wrap prefix.
func TestList_ProjectItemsError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 504 Gateway Timeout")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Items (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, _, err := runCmd(t, d, "list", "--scope=org")
	if err == nil {
		t.Fatalf("expected items error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list project items") {
		t.Errorf("expected wrap prefix `list project items`, got %q", err.Error())
	}
}

// TestStandup_RepoMergedPRsError pins the wrap behaviour for the second
// paginator in `runStandupRepo` (PaginateMergedPRs). Closed-issues listing
// succeeds (returns an empty page), and then merged-PRs listing fails. The
// wrap prefix `list merged PRs:` matches the literal in standup.go.
func TestStandup_RepoMergedPRsError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 502 Bad Gateway")
	emptyRepoIssues := map[string]any{"repository": map[string]any{"issues": map[string]any{"nodes": []any{}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListClosedIssues (", Data: emptyRepoIssues},
		{MatchSubstring: "query ListMergedPRs (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "standup")
	if err == nil {
		t.Fatalf("expected merged PRs error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list merged PRs") {
		t.Errorf("expected wrap prefix `list merged PRs`, got %q", err.Error())
	}
}

// TestPlan_RepoCreateMilestoneError pins the wrap behaviour for the REST
// POST /milestones call inside `runPlanRepo` when no matching milestone
// exists yet. The wrap prefix `create milestone:` matches the literal in
// plan.go; this also exercises the recordingREST error-injection path that
// the happy-path tests don't reach.
func TestPlan_RepoCreateMilestoneError(t *testing.T) {
	t.Parallel()

	restErr := errors.New("REST: HTTP 500 Internal Server Error")
	rest := &recordingREST{responses: []restResponse{
		{matchMethod: "POST", matchPath: "/milestones", err: restErr},
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
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return newClientsWithREST(g, rest), nil
		}
	})
	_, _, err := runCmd(t, d, "plan", "--period", "daily")
	if err == nil {
		t.Fatalf("expected REST create-milestone error to surface, got nil")
	}
	if !errors.Is(err, restErr) {
		t.Errorf("expected errors.Is(err, restErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "create milestone") {
		t.Errorf("expected wrap prefix `create milestone`, got %q", err.Error())
	}
}

// TestDone_ProjectUpdateFieldValueError pins the wrap behaviour for the
// UpdateProjectV2ItemFieldValue mutation in `runDoneProject`. Fields and
// items lookups succeed and the target item is found in non-Done state;
// the mutation then fails — the wrap prefix `update item field value:`
// matches the literal in done.go.
func TestDone_ProjectUpdateFieldValueError(t *testing.T) {
	t.Parallel()

	mutationErr := errors.New("graphql: HTTP 500 Internal Server Error")
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
		{MatchSubstring: "mutation UpdateProjectV2ItemFieldValue (", Err: mutationErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, _, err := runCmd(t, d, "done", "ITEM_X", "--scope=user")
	if err == nil {
		t.Fatalf("expected update-field-value error to surface, got nil")
	}
	if !errors.Is(err, mutationErr) {
		t.Errorf("expected errors.Is(err, mutationErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "update item field value") {
		t.Errorf("expected wrap prefix `update item field value`, got %q", err.Error())
	}
}

// TestStandup_ProjectMineViewerLoginError pins the wrap behaviour for
// `runStandupProject` when `--mine` triggers a GetViewerLogin call that
// fails at the transport layer. The wrap prefix `get viewer login:`
// matches the literal in standup.go (project branch).
func TestStandup_ProjectMineViewerLoginError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 401 Unauthorized")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerLogin", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, _, err := runCmd(t, d, "standup", "--scope=org", "--mine")
	if err == nil {
		t.Fatalf("expected viewer-login error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "get viewer login") {
		t.Errorf("expected wrap prefix `get viewer login`, got %q", err.Error())
	}
}

// TestPlan_RepoUpdateIssueMilestoneError pins the wrap behaviour for the
// UpdateIssueMilestone mutation in `runPlanRepo`. ListMilestones returns a
// matching milestone (so the create REST call is skipped), then the
// per-issue mutation fails — the wrap prefix `update issue milestone`
// matches the literal in plan.go.
func TestPlan_RepoUpdateIssueMilestoneError(t *testing.T) {
	t.Parallel()

	mutationErr := errors.New("graphql: HTTP 502 Bad Gateway")
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
		{MatchSubstring: "mutation UpdateIssueMilestone (", Err: mutationErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return newClientsWithREST(g, rest), nil
		}
	})
	_, _, err := runCmd(t, d, "plan", "--period", "daily")
	if err == nil {
		t.Fatalf("expected update-milestone error to surface, got nil")
	}
	if !errors.Is(err, mutationErr) {
		t.Errorf("expected errors.Is(err, mutationErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "update issue milestone") {
		t.Errorf("expected wrap prefix `update issue milestone`, got %q", err.Error())
	}
}

// TestLink_ProjectGetIssueError pins the wrap behaviour for the issue
// lookup inside `runLinkProject`. The PR lookup succeeds, then
// GetIssueByNumber fails — the wrap prefix `get issue:` matches the
// literal in link.go.
func TestLink_ProjectGetIssueError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 502 Bad Gateway")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{
			MatchSubstring: "query GetPullRequestByNumber (",
			Data: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 10, "url": "u/pr/10", "body": "",
			}}},
		},
		{MatchSubstring: "query GetIssueByNumber (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, _, err := runCmd(t, d, "link", "10", "20", "--scope=user")
	if err == nil {
		t.Fatalf("expected get-issue error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "get issue") {
		t.Errorf("expected wrap prefix `get issue`, got %q", err.Error())
	}
}

// TestLink_ProjectAddItemError pins the wrap behaviour for the
// AddProjectV2ItemById mutation in `runLinkProject`. PR + issue lookups
// succeed; the first mutation (add PR) fails — the wrap prefix `add PR to
// project:` matches the literal in link.go.
func TestLink_ProjectAddItemError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 500 Internal Server Error")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{
			MatchSubstring: "query GetPullRequestByNumber (",
			Data: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 10, "url": "u/pr/10", "body": "",
			}}},
		},
		{
			MatchSubstring: "query GetIssueByNumber (",
			Data: map[string]any{"repository": map[string]any{"issue": map[string]any{
				"id": "I_1", "number": 20, "url": "u/i/20", "state": "OPEN",
			}}},
		},
		{MatchSubstring: "mutation AddProjectV2ItemById (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, _, err := runCmd(t, d, "link", "10", "20", "--scope=user")
	if err == nil {
		t.Fatalf("expected add-item mutation error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "add PR to project") {
		t.Errorf("expected wrap prefix `add PR to project`, got %q", err.Error())
	}
}

// TestDone_ProjectFieldsError pins the wrap behaviour for the project-side
// fields paginator inside `runDoneProject`. The project resolution
// succeeds, then `PaginateProjectV2Fields` fails — the wrap prefix `list
// project fields:` matches the literal in done.go.
func TestDone_ProjectFieldsError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 502 Bad Gateway")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Fields (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, _, err := runCmd(t, d, "done", "ITEM_X", "--scope=user")
	if err == nil {
		t.Fatalf("expected fields error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list project fields") {
		t.Errorf("expected wrap prefix `list project fields`, got %q", err.Error())
	}
}

// TestDone_ProjectItemsError pins the wrap behaviour for the project-side
// items paginator inside `runDoneProject`. The fields paginator succeeds
// (returning a Status field with a Done option), then the items paginator
// fails — the wrap prefix `list project items:` matches the literal in
// done.go.
func TestDone_ProjectItemsError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 504 Gateway Timeout")
	fields := map[string]any{"node": map[string]any{"__typename": "ProjectV2", "fields": map[string]any{"nodes": []any{
		map[string]any{
			"__typename": "ProjectV2SingleSelectField", "id": "F_S", "name": "Status", "dataType": "SINGLE_SELECT",
			"options": []any{
				map[string]any{"id": "OPT_TODO", "name": "Todo"},
				map[string]any{"id": "OPT_DONE", "name": "Done"},
			},
		},
	}}}}
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Fields (", Data: fields},
		{MatchSubstring: "query ListProjectV2Items (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, _, err := runCmd(t, d, "done", "ITEM_X", "--scope=user")
	if err == nil {
		t.Fatalf("expected items error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list project items") {
		t.Errorf("expected wrap prefix `list project items`, got %q", err.Error())
	}
}

// TestStandup_ProjectItemsError pins the wrap behaviour for
// `runStandupProject` when the items paginator fails. The wrap prefix
// `list project items:` matches the literal in standup.go.
func TestStandup_ProjectItemsError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 503 Service Unavailable")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Items (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, _, err := runCmd(t, d, "standup", "--scope=org")
	if err == nil {
		t.Fatalf("expected standup project error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list project items") {
		t.Errorf("expected wrap prefix `list project items`, got %q", err.Error())
	}
}

// TestAdd_RepoCreateIssueError pins the wrap behaviour for the
// CreateIssue mutation in `runAddRepo`. The wrap prefix `create issue:`
// matches the literal in add.go.
func TestAdd_RepoCreateIssueError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 500 Internal Server Error")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query GetRepositoryID (",
			Data:           map[string]any{"repository": map[string]any{"id": "R_1"}},
		},
		{MatchSubstring: "mutation CreateIssue (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "add", "Title")
	if err == nil {
		t.Fatalf("expected create-issue error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "create issue") {
		t.Errorf("expected wrap prefix `create issue`, got %q", err.Error())
	}
}

// TestAdd_ProjectDraftIssueError pins the wrap behaviour for the
// AddProjectV2DraftIssue mutation in `runAddProject`. The wrap prefix `add
// project draft issue:` matches the literal in add.go.
func TestAdd_ProjectDraftIssueError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 502 Bad Gateway")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "mutation AddProjectV2DraftIssue (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, _, err := runCmd(t, d, "add", "Idea", "--scope=user")
	if err == nil {
		t.Fatalf("expected add-draft error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "add project draft issue") {
		t.Errorf("expected wrap prefix `add project draft issue`, got %q", err.Error())
	}
}

// TestToday_ProjectItemsError pins the wrap behaviour for `runTodayProject`
// when the project-items paginator fails. The wrap prefix `list project
// items:` matches the literal in today.go.
func TestToday_ProjectItemsError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 502 Bad Gateway")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Items (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, _, err := runCmd(t, d, "today", "--scope=user")
	if err == nil {
		t.Fatalf("expected today project error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list project items") {
		t.Errorf("expected wrap prefix `list project items`, got %q", err.Error())
	}
}

// TestTriage_ProjectItemsError pins the wrap behaviour for
// `runTriageProject` when the project-items paginator fails. The wrap
// prefix `list project items:` matches the literal in triage.go.
func TestTriage_ProjectItemsError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 503 Service Unavailable")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOrgProjectV2 (", Data: orgProject("PVT_org")},
		{MatchSubstring: "query ListProjectV2Items (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{OrgProject: project.Ref{Owner: "octo", Number: 7}}, nil
		}
	})
	_, _, err := runCmd(t, d, "triage", "--scope=org")
	if err == nil {
		t.Fatalf("expected triage project error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list project items") {
		t.Errorf("expected wrap prefix `list project items`, got %q", err.Error())
	}
}

// TestReview_ProjectItemsError pins the wrap behaviour for
// `runReviewProject` when the project-items paginator fails. The wrap
// prefix `list project items:` matches the literal in review.go.
func TestReview_ProjectItemsError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 504 Gateway Timeout")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetUserProjectV2 (", Data: userProject("PVT_user")},
		{MatchSubstring: "query ListProjectV2Items (", Err: transportErr},
	}}
	d := testDeps(g, func(d *cmd.Deps) {
		d.HasGitRemote = func() bool { return false }
		d.LoadConfig = func() (config.AppConfig, error) {
			return config.AppConfig{UserProject: project.Ref{Owner: "ozzy", Number: 9}}, nil
		}
	})
	_, _, err := runCmd(t, d, "review", "--scope=user", "--period", "weekly")
	if err == nil {
		t.Fatalf("expected review project error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list project items") {
		t.Errorf("expected wrap prefix `list project items`, got %q", err.Error())
	}
}

// TestLink_RepoGetPullRequestError pins the wrap behaviour for the
// pull-request lookup query in `runLinkRepo`. The wrap prefix `get pull
// request:` matches the literal in link.go.
func TestLink_RepoGetPullRequestError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 502 Bad Gateway")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetPullRequestByNumber (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "link", "10", "20")
	if err == nil {
		t.Fatalf("expected get-PR error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "get pull request") {
		t.Errorf("expected wrap prefix `get pull request`, got %q", err.Error())
	}
}

// TestLink_RepoUpdatePullRequestError pins the wrap behaviour when the
// PR-body mutation in `runLinkRepo` fails after the PR lookup succeeds. The
// wrap prefix `update pull request body:` matches the literal in link.go.
func TestLink_RepoUpdatePullRequestError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 500 Internal Server Error")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{
			MatchSubstring: "query GetPullRequestByNumber (",
			Data: map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"id": "PR_1", "number": 10, "url": "u/pr/10", "body": "",
			}}},
		},
		{MatchSubstring: "mutation UpdatePullRequest (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "link", "10", "20")
	if err == nil {
		t.Fatalf("expected update-PR error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "update pull request body") {
		t.Errorf("expected wrap prefix `update pull request body`, got %q", err.Error())
	}
}

// TestStandup_MineViewerLoginTransportError pins the wrap behaviour when
// `--mine` triggers a GetViewerLogin call that fails at the transport
// layer. The wrap prefix `get viewer login:` matches the literal in
// standup.go.
func TestStandup_MineViewerLoginTransportError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 401 Unauthorized")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerLogin", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "standup", "--mine")
	if err == nil {
		t.Fatalf("expected viewer-login error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "get viewer login") {
		t.Errorf("expected wrap prefix `get viewer login`, got %q", err.Error())
	}
}

// TestToday_RepoListError pins the wrap behaviour for `runTodayRepo`. The
// wrap prefix `list repo issues:` matches the literal in today.go.
func TestToday_RepoListError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 503 Service Unavailable")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "today")
	if err == nil {
		t.Fatalf("expected today error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list repo issues") {
		t.Errorf("expected wrap prefix `list repo issues`, got %q", err.Error())
	}
}

// TestTriage_RepoListError pins the wrap behaviour for `runTriageRepo`. The
// wrap prefix `list repo issues with labels:` matches the literal in
// triage.go.
func TestTriage_RepoListError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 502 Bad Gateway")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssuesWithLabels (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "triage")
	if err == nil {
		t.Fatalf("expected triage error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list repo issues with labels") {
		t.Errorf("expected wrap prefix `list repo issues with labels`, got %q", err.Error())
	}
}

// TestAdd_RepoGetRepositoryIDError pins the wrap behaviour for the repo-id
// resolution query in `runAddRepo`. The wrap prefix `get repository id:`
// matches the literal in add.go.
func TestAdd_RepoGetRepositoryIDError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 500 Internal Server Error")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetRepositoryID (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "add", "Title")
	if err == nil {
		t.Fatalf("expected get-repo-id error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "get repository id") {
		t.Errorf("expected wrap prefix `get repository id`, got %q", err.Error())
	}
}

// TestDone_RepoGetIssueError pins the wrap behaviour for the issue-lookup
// query in `runDoneRepo`. The wrap prefix `get issue:` matches the literal
// in done.go.
func TestDone_RepoGetIssueError(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("graphql: HTTP 500 Internal Server Error")
	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetIssueByNumber (", Err: transportErr},
	}}
	d := testDeps(g)
	_, _, err := runCmd(t, d, "done", "7")
	if err == nil {
		t.Fatalf("expected get-issue error to surface, got nil")
	}
	if !errors.Is(err, transportErr) {
		t.Errorf("expected errors.Is(err, transportErr) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "get issue") {
		t.Errorf("expected wrap prefix `get issue`, got %q", err.Error())
	}
}

// ===== Context cancellation =================================================

// TestList_RepoContextCanceled pins the cancellation propagation contract:
// when the cobra root is invoked with an already-cancelled context, the
// paginator's first Do observes ctx.Err() (via ctxAwareGraphQL) and returns
// context.Canceled. `runListRepo` then wraps that with `list repo issues:`
// and returns it; errors.Is(err, context.Canceled) must hold so callers
// (notably `main` for any future signal-handling refactor) can distinguish
// "user pressed Ctrl-C" from a generic transport failure.
func TestList_RepoContextCanceled(t *testing.T) {
	t.Parallel()

	// Pre-canned data path so the test is deterministic if the paginator
	// somehow bypasses ctx — the assertion still fails loudly because the
	// returned err would be nil.
	inner := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query ListRepoIssues (", Data: repoIssuesPayload()},
	}}
	wrapped := &ctxAwareGraphQL{inner: inner}
	d := testDeps(inner, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrapped, REST: fakeREST{}}, nil
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so the first paginator Do short-circuits to ctx.Err().

	_, _, err := runCmdWithContext(t, ctx, d, "list")
	if err == nil {
		t.Fatalf("expected context.Canceled to surface, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected errors.Is(err, context.Canceled) to hold, got %v", err)
	}
	if !strings.Contains(err.Error(), "list repo issues") {
		t.Errorf("expected wrap prefix `list repo issues`, got %q", err.Error())
	}
	if errors.Is(err, cmd.ErrSilent) {
		t.Errorf("context.Canceled must NOT classify as ErrSilent, got %v", err)
	}
}
