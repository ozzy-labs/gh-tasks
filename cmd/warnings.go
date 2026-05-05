package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// truncationKind enumerates the result-list categories that can hit the
// per-query page limit. Using a typed enum (rather than free-form strings)
// keeps callers from drifting into kinds that have no matching i18n label
// and lets the lookup table in [truncationKindLabel] stay exhaustive.
type truncationKind string

const (
	kindRepoIssues   truncationKind = "repo_issues"
	kindClosedIssues truncationKind = "closed_issues"
	kindMergedPRs    truncationKind = "merged_prs"
	kindOpenIssues   truncationKind = "open_issues"
	kindMilestones   truncationKind = "milestones"
	kindProjectItems truncationKind = "project_items"
)

// truncationKindLabel resolves a [truncationKind] to its localized label
// using the resolved locale carried on Resolved. The keys are emitted as
// string literals (not built from `kind` at runtime) so the check-i18n
// scanner can see every catalog reference statically.
func truncationKindLabel(r Resolved, kind truncationKind) string {
	switch kind {
	case kindRepoIssues:
		return r.T("warn.results.kind.repo_issues")
	case kindClosedIssues:
		return r.T("warn.results.kind.closed_issues")
	case kindMergedPRs:
		return r.T("warn.results.kind.merged_prs")
	case kindOpenIssues:
		return r.T("warn.results.kind.open_issues")
	case kindMilestones:
		return r.T("warn.results.kind.milestones")
	case kindProjectItems:
		return r.T("warn.results.kind.project_items")
	}
	return string(kind)
}

// warnIfTruncated emits a stderr warning when a list query returned at or
// above the page limit, signalling that older entries may have been
// silently dropped. Pagination is not yet wired through the GraphQL
// operations (operations.graphql lacks pageInfo), so the most we can do is
// surface the truncation to the user.
func warnIfTruncated(c *cobra.Command, r Resolved, kind truncationKind, fetched, limit int) {
	if fetched < limit {
		return
	}
	label := truncationKindLabel(r, kind)
	fmt.Fprintln(c.ErrOrStderr(), r.T("warn.results.truncated", "kind", label, "limit", limit))
}
