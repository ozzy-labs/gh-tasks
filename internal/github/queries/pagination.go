// Package queries paginator helpers.
//
// Each `Paginate<List>` function below wraps the corresponding genqlient
// `List<List>` operation and walks pageInfo cursors until either:
//
//   - the upstream connection reports `hasNextPage: false`,
//   - `len(accumulated)` reaches the caller's `limit`, or
//   - the safety valve `maxPages` is hit.
//
// Semantics shared by every paginator:
//
//   - Page size: per request, `min(limit - len(accumulated), maxPageSize)`.
//   - Partial-failure policy: if any page fails, the accumulated nodes are
//     discarded and the raw error is returned. Callers must not assume a
//     successful return implies "all entries up to limit"; they only get
//     "we hit limit OR we exhausted the upstream connection". The discard
//     keeps the invariant "len(items) < limit ⇒ full fetch completed".
//   - Error wrapping: errors are returned raw. Callers are expected to wrap
//     them at the cmd layer (e.g. `fmt.Errorf("list repo issues: %w", err)`)
//     so the resulting message is consistent with the existing call sites.
//   - Sentinel errors (`ErrRepoNotFound`, `ErrProjectNotFound`) are returned
//     in English, so the cmd layer can detect them with `errors.Is` and
//     surface a localized message via `r.T(...)`.
//   - `EndCursor` is `*string` per the GraphQL schema (PageInfo.endCursor:
//     String). A nil or empty cursor terminates pagination defensively even
//     if `hasNextPage` was true, because GitHub never returns nil cursors
//     when there are more pages — and continuing with nil after such a
//     mismatch would just refetch page 1 forever.
package queries

import (
	"context"
	"errors"

	"github.com/Khan/genqlient/graphql"
)

// Short aliases for genqlient's deeply-nested response types. Each alias
// targets the `nodes []*Issue` (or PR / Milestone) leaf for one ListXxx
// operation; paginator return types use these aliases to keep signatures
// readable. ProjectV2FieldNode / ProjectV2ItemNode are already exposed
// under short names by the existing genqlient (typename: ...) directives
// in operations.graphql, so they are reused as-is below.

// RepoIssue is the per-node type returned by [ListRepoIssues].
type RepoIssue = ListRepoIssuesRepositoryIssuesIssueConnectionNodesIssue

// RepoIssueWithLabel is the per-node type returned by [ListRepoIssuesWithLabels].
type RepoIssueWithLabel = ListRepoIssuesWithLabelsRepositoryIssuesIssueConnectionNodesIssue

// ClosedIssue is the per-node type returned by [ListClosedIssues].
type ClosedIssue = ListClosedIssuesRepositoryIssuesIssueConnectionNodesIssue

// MergedPR is the per-node type returned by [ListMergedPRs].
type MergedPR = ListMergedPRsRepositoryPullRequestsPullRequestConnectionNodesPullRequest

// RepoIssueWithMilestone is the per-node type returned by [ListRepoIssuesWithMilestone].
type RepoIssueWithMilestone = ListRepoIssuesWithMilestoneRepositoryIssuesIssueConnectionNodesIssue

// Milestone is the per-node type returned by [ListMilestones].
type Milestone = ListMilestonesRepositoryMilestonesMilestoneConnectionNodesMilestone

const (
	// maxPageSize matches GitHub's hard upper bound on `first:` for v4
	// connections. Paginator helpers cap their request size at this value
	// even when the caller's limit is higher.
	maxPageSize = 100

	// maxPages is a safety valve in case GitHub keeps returning
	// `hasNextPage: true` indefinitely (e.g. cursor mishandling, schema
	// change). At maxPageSize=100 this caps at 1000 items per call —
	// well above any legitimate v0.1.0 use case.
	maxPages = 10
)

// ErrRepoNotFound is returned when a repository connection paginator was
// asked for a repo that does not exist or is invisible to the active token.
// Callers test with `errors.Is(err, queries.ErrRepoNotFound)`.
var ErrRepoNotFound = errors.New("repository not found")

// ErrProjectNotFound is returned when a Projects v2 connection paginator
// was asked for a project node id that no longer resolves to a ProjectV2.
// Callers test with `errors.Is(err, queries.ErrProjectNotFound)`.
var ErrProjectNotFound = errors.New("project not found")

// pageStep computes the request size for the next page given the running
// accumulated count and the caller's overall limit. Returns 0 when the
// caller has already hit limit and the loop should break.
func pageStep(accumulated, limit int) int {
	remaining := limit - accumulated
	if remaining <= 0 {
		return 0
	}
	if remaining < maxPageSize {
		return remaining
	}
	return maxPageSize
}

// PaginateRepoIssues fetches up to `limit` open Issues for owner/name by
// walking the `repository.issues` connection.
//
// Partial-failure policy: a page-level error discards every node accumulated
// so far. The error is returned raw; the caller is expected to wrap it.
//
// Returns ErrRepoNotFound when the repository node is missing on any page.
func PaginateRepoIssues(ctx context.Context, client graphql.Client, owner, name string, limit int) ([]*RepoIssue, error) {
	var (
		all    []*RepoIssue
		cursor *string
	)
	for page := 0; page < maxPages; page++ {
		size := pageStep(len(all), limit)
		if size <= 0 {
			break
		}
		resp, err := ListRepoIssues(ctx, client, owner, name, size, cursor)
		if err != nil {
			return nil, err
		}
		if resp.Repository == nil {
			return nil, ErrRepoNotFound
		}
		all = append(all, resp.Repository.Issues.Nodes...)
		pi := resp.Repository.Issues.PageInfo
		if pi == nil || !pi.HasNextPage {
			break
		}
		if pi.EndCursor == nil || *pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// PaginateRepoIssuesWithLabels fetches up to `limit` open Issues with their
// label connections.
//
// Partial-failure policy: a page-level error discards every node accumulated
// so far. The error is returned raw; the caller is expected to wrap it.
//
// Returns ErrRepoNotFound when the repository node is missing on any page.
func PaginateRepoIssuesWithLabels(ctx context.Context, client graphql.Client, owner, name string, limit int) ([]*RepoIssueWithLabel, error) {
	var (
		all    []*RepoIssueWithLabel
		cursor *string
	)
	for page := 0; page < maxPages; page++ {
		size := pageStep(len(all), limit)
		if size <= 0 {
			break
		}
		resp, err := ListRepoIssuesWithLabels(ctx, client, owner, name, size, cursor)
		if err != nil {
			return nil, err
		}
		if resp.Repository == nil {
			return nil, ErrRepoNotFound
		}
		all = append(all, resp.Repository.Issues.Nodes...)
		pi := resp.Repository.Issues.PageInfo
		if pi == nil || !pi.HasNextPage {
			break
		}
		if pi.EndCursor == nil || *pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// PaginateClosedIssues fetches up to `limit` recently CLOSED Issues.
//
// Partial-failure policy: a page-level error discards every node accumulated
// so far. The error is returned raw; the caller is expected to wrap it.
//
// Returns ErrRepoNotFound when the repository node is missing on any page.
func PaginateClosedIssues(ctx context.Context, client graphql.Client, owner, name string, limit int) ([]*ClosedIssue, error) {
	var (
		all    []*ClosedIssue
		cursor *string
	)
	for page := 0; page < maxPages; page++ {
		size := pageStep(len(all), limit)
		if size <= 0 {
			break
		}
		resp, err := ListClosedIssues(ctx, client, owner, name, size, cursor)
		if err != nil {
			return nil, err
		}
		if resp.Repository == nil {
			return nil, ErrRepoNotFound
		}
		all = append(all, resp.Repository.Issues.Nodes...)
		pi := resp.Repository.Issues.PageInfo
		if pi == nil || !pi.HasNextPage {
			break
		}
		if pi.EndCursor == nil || *pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// PaginateMergedPRs fetches up to `limit` recently MERGED PRs.
//
// Partial-failure policy: a page-level error discards every node accumulated
// so far. The error is returned raw; the caller is expected to wrap it.
//
// Returns ErrRepoNotFound when the repository node is missing on any page.
func PaginateMergedPRs(ctx context.Context, client graphql.Client, owner, name string, limit int) ([]*MergedPR, error) {
	var (
		all    []*MergedPR
		cursor *string
	)
	for page := 0; page < maxPages; page++ {
		size := pageStep(len(all), limit)
		if size <= 0 {
			break
		}
		resp, err := ListMergedPRs(ctx, client, owner, name, size, cursor)
		if err != nil {
			return nil, err
		}
		if resp.Repository == nil {
			return nil, ErrRepoNotFound
		}
		all = append(all, resp.Repository.PullRequests.Nodes...)
		pi := resp.Repository.PullRequests.PageInfo
		if pi == nil || !pi.HasNextPage {
			break
		}
		if pi.EndCursor == nil || *pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// PaginateRepoIssuesWithMilestone fetches up to `limit` open Issues with
// their milestone bindings.
//
// Partial-failure policy: a page-level error discards every node accumulated
// so far. The error is returned raw; the caller is expected to wrap it.
//
// Returns ErrRepoNotFound when the repository node is missing on any page.
func PaginateRepoIssuesWithMilestone(ctx context.Context, client graphql.Client, owner, name string, limit int) ([]*RepoIssueWithMilestone, error) {
	var (
		all    []*RepoIssueWithMilestone
		cursor *string
	)
	for page := 0; page < maxPages; page++ {
		size := pageStep(len(all), limit)
		if size <= 0 {
			break
		}
		resp, err := ListRepoIssuesWithMilestone(ctx, client, owner, name, size, cursor)
		if err != nil {
			return nil, err
		}
		if resp.Repository == nil {
			return nil, ErrRepoNotFound
		}
		all = append(all, resp.Repository.Issues.Nodes...)
		pi := resp.Repository.Issues.PageInfo
		if pi == nil || !pi.HasNextPage {
			break
		}
		if pi.EndCursor == nil || *pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// PaginateMilestones fetches up to `limit` recently updated open milestones.
//
// Partial-failure policy: a page-level error discards every node accumulated
// so far. The error is returned raw; the caller is expected to wrap it.
//
// Returns ErrRepoNotFound when the repository node is missing on any page.
func PaginateMilestones(ctx context.Context, client graphql.Client, owner, name string, limit int) ([]*Milestone, error) {
	var (
		all    []*Milestone
		cursor *string
	)
	for page := 0; page < maxPages; page++ {
		size := pageStep(len(all), limit)
		if size <= 0 {
			break
		}
		resp, err := ListMilestones(ctx, client, owner, name, size, cursor)
		if err != nil {
			return nil, err
		}
		if resp.Repository == nil {
			return nil, ErrRepoNotFound
		}
		all = append(all, resp.Repository.Milestones.Nodes...)
		pi := resp.Repository.Milestones.PageInfo
		if pi == nil || !pi.HasNextPage {
			break
		}
		if pi.EndCursor == nil || *pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// PaginateProjectV2Fields fetches up to `limit` field descriptors for a
// Projects v2 board, walking the inner `fields` connection.
//
// The `node(id:)` lookup goes through the schema's Node interface, so the
// projection must narrow to ProjectV2 with a type assertion every page; if
// the returned node has any other concrete type (or is nil), the loop
// returns ErrProjectNotFound. The variant union for `nodes` (Common /
// SingleSelect / Iteration) is preserved in the returned slice — each
// element keeps its concrete genqlient type so callers can downcast to
// the variant they need (used by projectitem.FieldsOf to flatten into
// FieldDescriptor).
//
// Partial-failure policy: a page-level error discards every node accumulated
// so far. The error is returned raw; the caller is expected to wrap it.
func PaginateProjectV2Fields(ctx context.Context, client graphql.Client, projectID string, limit int) ([]ProjectV2FieldNode, error) {
	var (
		all    []ProjectV2FieldNode
		cursor *string
	)
	for page := 0; page < maxPages; page++ {
		size := pageStep(len(all), limit)
		if size <= 0 {
			break
		}
		resp, err := ListProjectV2Fields(ctx, client, projectID, size, cursor)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.Node == nil {
			return nil, ErrProjectNotFound
		}
		pv2, ok := (*resp.Node).(*ProjectV2FieldsNodeProjectV2)
		if !ok || pv2 == nil || pv2.Fields == nil {
			return nil, ErrProjectNotFound
		}
		for _, n := range pv2.Fields.Nodes {
			if n == nil || *n == nil {
				continue
			}
			all = append(all, *n)
		}
		pi := pv2.Fields.PageInfo
		if pi == nil || !pi.HasNextPage {
			break
		}
		if pi.EndCursor == nil || *pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// PaginateProjectV2Items fetches up to `limit` items for a Projects v2
// board, walking the inner `items` connection.
//
// Same Node-interface caveat as PaginateProjectV2Fields: when the returned
// node is missing or has the wrong concrete type, ErrProjectNotFound is
// returned.
//
// Partial-failure policy: a page-level error discards every node accumulated
// so far. The error is returned raw; the caller is expected to wrap it.
func PaginateProjectV2Items(ctx context.Context, client graphql.Client, projectID string, limit int) ([]*ProjectV2ItemNode, error) {
	var (
		all    []*ProjectV2ItemNode
		cursor *string
	)
	for page := 0; page < maxPages; page++ {
		size := pageStep(len(all), limit)
		if size <= 0 {
			break
		}
		resp, err := ListProjectV2Items(ctx, client, projectID, size, cursor)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.Node == nil {
			return nil, ErrProjectNotFound
		}
		pv2, ok := (*resp.Node).(*ProjectV2ItemsNodeProjectV2)
		if !ok || pv2 == nil || pv2.Items == nil {
			return nil, ErrProjectNotFound
		}
		all = append(all, pv2.Items.Nodes...)
		pi := pv2.Items.PageInfo
		if pi == nil || !pi.HasNextPage {
			break
		}
		if pi.EndCursor == nil || *pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}
