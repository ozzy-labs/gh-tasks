// Package projectitem provides helpers for resolving and rendering Projects
// v2 items. The format helpers preserve byte-identical output with the prior
// TS implementation so review/standup/list outputs do not drift.
package projectitem

import (
	"context"
	"fmt"
	"strings"

	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

// ResolveProjectNodeID resolves a [project.Ref] to its Projects v2 node id by
// issuing the appropriate GraphQL query for the scope. Returns ("", nil) when
// the project cannot be found (wrong owner, wrong number, or insufficient
// scopes on the token).
//
// Calling this with [scope.Repo] is a programmer error (Projects v2 are not
// used in repo scope) and returns a localizable [scope.ScopeError]. Callers
// should resolve the project ahead of this call via [project.Resolve].
func ResolveProjectNodeID(ctx context.Context, gql github.GraphQLClient, sc scope.Scope, ref project.Ref) (string, error) {
	if sc == scope.Repo {
		return "", &scope.ScopeError{Payload: i18n.NewPayload("error.scope.invalidForProjectResolution")}
	}
	gqlClient := github.AsGenqlientClientFor(gql)

	if sc == scope.Org {
		resp, err := queries.GetOrgProjectV2(ctx, gqlClient, ref.Owner, ref.Number)
		if err != nil {
			return "", fmt.Errorf("get org project: %w", err)
		}
		if resp.Organization == nil || resp.Organization.ProjectV2 == nil {
			return "", nil
		}
		return resp.Organization.ProjectV2.Id, nil
	}

	resp, err := queries.GetUserProjectV2(ctx, gqlClient, ref.Owner, ref.Number)
	if err != nil {
		return "", fmt.Errorf("get user project: %w", err)
	}
	if resp.User == nil || resp.User.ProjectV2 == nil {
		return "", nil
	}
	return resp.User.ProjectV2.Id, nil
}

// FindStatus returns the value of the conventionally-named "Status" single
// select field, or "" when the item has no Status set.
func FindStatus(values []queries.ProjectV2ItemFieldValue) string {
	for _, v := range values {
		if v.Typename == "ProjectV2ItemFieldSingleSelectValue" &&
			strings.EqualFold(v.Field.Name, "status") {
			return v.Name
		}
	}
	return ""
}

// FormatItem renders a multi-line "list" view of a Projects v2 item. The
// output matches the prior TS formatItem byte-for-byte:
//
//   - Issue / PullRequest: "<prefix>#<n>  <title>[  [Status]]\n  <url>\n"
//     (prefix is "PR" for PullRequest and empty for Issue)
//   - DraftIssue:          "(draft)  <title>[  [Status]]\n"
//   - missing content:     "(no content)[  [Status]]\n"
//
// Trailing newlines are intentional — callers write the result directly
// without adding their own newline.
func FormatItem(item queries.ProjectV2ItemNode) string {
	statusSuffix := ""
	if status := FindStatus(item.FieldValues.Nodes); status != "" {
		statusSuffix = "  [" + status + "]"
	}
	c := item.Content
	if c == nil {
		return "(no content)" + statusSuffix + "\n"
	}
	switch c.Typename {
	case "Issue":
		return fmt.Sprintf("#%d  %s%s\n  %s\n", c.Number, c.Title, statusSuffix, c.URL)
	case "PullRequest":
		return fmt.Sprintf("PR#%d  %s%s\n  %s\n", c.Number, c.Title, statusSuffix, c.URL)
	default:
		return "(draft)  " + c.Title + statusSuffix + "\n"
	}
}

// FormatItemLineCompact renders a single-line "compact" view of a Projects
// v2 item, used when the caller embeds the line into a bulleted Markdown
// list. Format matches the prior TS formatItemLine in review.ts / standup.ts:
//
//   - Issue / PullRequest: "<prefix>#<n> <title> (<url>)"
//   - DraftIssue:          "(draft) <title>"
//   - missing content:     "(no content)"
//
// No leading indent, no trailing newline, no Status suffix.
func FormatItemLineCompact(item queries.ProjectV2ItemNode) string {
	c := item.Content
	if c == nil {
		return "(no content)"
	}
	switch c.Typename {
	case "Issue":
		return fmt.Sprintf("#%d %s (%s)", c.Number, c.Title, c.URL)
	case "PullRequest":
		return fmt.Sprintf("PR#%d %s (%s)", c.Number, c.Title, c.URL)
	default:
		return "(draft) " + c.Title
	}
}
