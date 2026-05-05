package queries_test

// Defensive nil-inner-connection coverage for the 5 paginators that #286
// fixed alongside PaginateRepoIssues. PaginateRepoIssues already had
// TestPaginateRepoIssues_NilInnerConnection in pagination_test.go; this
// file pins the same contract for the remaining 5, so a regression that
// reintroduces a nil-deref in any one of them surfaces independently.
//
// The contract under test for every paginator: when GitHub returns a
// repository envelope with the inner connection set to nil
// (`repository: { issues: null }` etc.), the paginator must treat it as
// "no more pages", return an empty slice with nil error, and not panic
// on the inner Nodes / PageInfo dereferences that follow.

import (
	"context"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
)

func TestPaginateRepoIssuesWithLabels_NilInnerConnection(t *testing.T) {
	t.Parallel()
	steps := []scriptStep{
		{
			op: "ListRepoIssuesWithLabels", wantSize: 30,
			respond: func(out any) {
				r := out.(*queries.ListRepoIssuesWithLabelsResponse)
				r.Repository = &queries.ListRepoIssuesWithLabelsRepository{Issues: nil}
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateRepoIssuesWithLabels(context.Background(), client, "o", "n", 30)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 items, got %d", len(got))
	}
}

func TestPaginateClosedIssues_NilInnerConnection(t *testing.T) {
	t.Parallel()
	steps := []scriptStep{
		{
			op: "ListClosedIssues", wantSize: 30,
			respond: func(out any) {
				r := out.(*queries.ListClosedIssuesResponse)
				r.Repository = &queries.ListClosedIssuesRepository{Issues: nil}
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateClosedIssues(context.Background(), client, "o", "n", 30)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 items, got %d", len(got))
	}
}

func TestPaginateMergedPRs_NilInnerConnection(t *testing.T) {
	t.Parallel()
	// PullRequests connection on the Repository envelope (not Issues): this
	// paginator's nil-guard targets a different field name than the issue
	// paginators, so it has its own regression risk.
	steps := []scriptStep{
		{
			op: "ListMergedPRs", wantSize: 30,
			respond: func(out any) {
				r := out.(*queries.ListMergedPRsResponse)
				r.Repository = &queries.ListMergedPRsRepository{PullRequests: nil}
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateMergedPRs(context.Background(), client, "o", "n", 30)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 items, got %d", len(got))
	}
}

func TestPaginateRepoIssuesWithMilestone_NilInnerConnection(t *testing.T) {
	t.Parallel()
	steps := []scriptStep{
		{
			op: "ListRepoIssuesWithMilestone", wantSize: 30,
			respond: func(out any) {
				r := out.(*queries.ListRepoIssuesWithMilestoneResponse)
				r.Repository = &queries.ListRepoIssuesWithMilestoneRepository{Issues: nil}
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateRepoIssuesWithMilestone(context.Background(), client, "o", "n", 30)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 items, got %d", len(got))
	}
}

func TestPaginateMilestones_NilInnerConnection(t *testing.T) {
	t.Parallel()
	// Milestones connection: third distinct inner field name in the
	// repo-scoped paginator family (Issues / PullRequests / Milestones).
	steps := []scriptStep{
		{
			op: "ListMilestones", wantSize: 30,
			respond: func(out any) {
				r := out.(*queries.ListMilestonesResponse)
				r.Repository = &queries.ListMilestonesRepository{Milestones: nil}
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateMilestones(context.Background(), client, "o", "n", 30)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 items, got %d", len(got))
	}
}
