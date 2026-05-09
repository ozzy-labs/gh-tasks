package cmd

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/jsonout"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
)

// itemJSONFields is the shared catalog for commands whose JSON output is a
// flat list of GitHub items (Issue / PR / Project item / draft). list and
// today both publish exactly this set; standup and review extend it via
// activityJSONFields below.
var itemJSONFields = jsonout.FieldList{
	{Name: "id", Description: "GraphQL global ID of the Issue or Project item"},
	{Name: "number", Description: "Issue / Project item number (0 for draft items)"},
	{Name: "state", Description: "Issue / PR state (`OPEN` / `CLOSED` / `MERGED`); empty string for draft items where it does not apply"},
	{Name: "title", Description: "Title of the Issue or Project item"},
	{Name: "type", Description: "ISSUE | PULL_REQUEST | DRAFT_ISSUE"},
	{Name: "updatedAt", Description: "Last-update timestamp (RFC 3339)"},
	{Name: "url", Description: "Absolute URL on github.com (empty for draft items)"},
}

// activityJSONFields is itemJSONFields plus a `category` discriminator used
// by standup / review to flatten their multi-section output (closed,
// merged, in-progress, etc.) into a single jq-friendly array.
var activityJSONFields = append(append(jsonout.FieldList{}, itemJSONFields...),
	jsonout.Field{Name: "category", Description: "Activity bucket the row belongs to (e.g. closed / merged / in-progress / done / completed)"})

// linkJSONFields is itemJSONFields plus link-specific metadata. Used by
// `link` to describe the PR row plus the task it was bound to.
var linkJSONFields = append(append(jsonout.FieldList{}, itemJSONFields...),
	jsonout.Field{Name: "linkType", Description: "How the link was established: `closesAdded` (PR body got `Closes #N`) or `projectBind` (PR + task bound to the same Project v2)"},
	jsonout.Field{Name: "linkedTo", Description: "Target task that the PR was linked to. Object `{id, number, type, url}` or null when the link was already in place"},
)

// projectInitJSONFields is the catalog for `projects init` and
// `projects init-templates`. Each row describes a Project v2 (created
// or template) plus its field definitions.
var projectInitJSONFields = jsonout.FieldList{
	{Name: "id", Description: "GraphQL global ID of the created Project v2 (empty for --dry-run / init-templates)"},
	{Name: "number", Description: "Project v2 number (0 for --dry-run / init-templates)"},
	{Name: "title", Description: "Project title"},
	{Name: "url", Description: "Project URL on github.com (empty for --dry-run / init-templates)"},
	{Name: "owner", Description: "Owner login (`@me` resolved to the actual viewer login at runtime; empty for init-templates)"},
	{Name: "template", Description: "Template name (`user` / `org`) or empty when a custom yaml path was used"},
	{Name: "fields", Description: "Array of `{name, dataType, options?}` for the configured field set"},
}

// jsonRequest carries the resolved --json / --jq flag values from the
// shared resolver to per-scope handlers.
type jsonRequest struct {
	fields []string
	jq     string
}

// addJSONFlags wires --json / --jq onto a cobra command. See list.go for
// the rationale on `String` vs `StringSlice`.
func addJSONFlags(c *cobra.Command) {
	c.Flags().String("json", "", "output as JSON. Empty value (`--json=`) lists available fields.")
	c.Flags().String("jq", "", "filter JSON output via jq expression")
}

// addPaginateFlag wires --paginate onto a read-only command. When set,
// the per-command fetch limit is overridden with math.MaxInt32 so the
// underlying paginator walks the whole result set. Note that Projects v2
// GraphQL is cost-heavy; --paginate may consume noticeable rate-limit
// budget on large boards.
func addPaginateFlag(c *cobra.Command) {
	c.Flags().Bool("paginate", false, "fetch all pages (overrides --limit). May consume noticeable rate-limit budget on large Projects v2.")
}

// effectivePaginateLimit returns math.MaxInt32 when --paginate is set,
// otherwise returns the supplied default. Use it in read-only commands
// after computing the local default so --paginate cleanly overrides.
func effectivePaginateLimit(c *cobra.Command, defaultLimit int) int {
	paginate, _ := c.Flags().GetBool("paginate")
	if paginate {
		return math.MaxInt32
	}
	return defaultLimit
}

// addJSONCompletion registers a flag-completion function for `--json`
// that suggests catalog field names. Comma-separated entries are
// supported: existing fields are filtered out so the shell only offers
// names that have not been picked yet. cobra's `ShellCompDirectiveNoSpace`
// keeps the cursor adjacent to the comma so the user can chain field
// names without re-tabbing.
func addJSONCompletion(c *cobra.Command, catalog jsonout.FieldList) {
	_ = c.RegisterFlagCompletionFunc("json", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return jsonout.CompleteFields(catalog, toComplete), cobra.ShellCompDirectiveNoSpace
	})
}

// resolveJSONRequest reads --json / --jq from the command, validates them,
// and returns (request, jsonOn, error). When --json is given as empty
// (`--json=` or `--json ""`), the catalog is printed to stderr and
// ErrSilentArgs is returned. --jq without --json is also rejected.
func resolveJSONRequest(c *cobra.Command, r Resolved, catalog jsonout.FieldList) (jsonRequest, bool, error) {
	jq, _ := c.Flags().GetString("jq")
	jsonChanged := c.Flags().Changed("json")
	if !jsonChanged {
		if jq != "" {
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.json.jqWithoutJson"))
			return jsonRequest{}, false, ErrSilentArgs
		}
		return jsonRequest{}, false, nil
	}
	raw, _ := c.Flags().GetString("json")
	fields := splitJSONFields(raw)
	if len(fields) == 0 {
		jsonout.ListFields(c.ErrOrStderr(), catalog)
		return jsonRequest{}, false, ErrSilentArgs
	}
	return jsonRequest{fields: fields, jq: jq}, true, nil
}

// splitJSONFields parses the comma-separated `--json` value into a clean
// list of field names, dropping empty entries (e.g. trailing commas) and
// trimming whitespace.
func splitJSONFields(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// renderJSONItems writes items as JSON to stdout, with consistent error
// handling: UnknownFieldError re-prints the catalog so users can self-
// correct, JQError prints the gojq diagnostic, and any other failure flows
// through wrapTransport. Use this from cmd/* whenever a command has built
// its DTO rows and needs to honour the --json / --jq contract.
func renderJSONItems(c *cobra.Command, r Resolved, items []map[string]any, req jsonRequest, catalog jsonout.FieldList) error {
	if err := jsonout.Render(c.OutOrStdout(), items, req.fields, req.jq, catalog); err != nil {
		var unknown *jsonout.UnknownFieldError
		if errors.As(err, &unknown) {
			fmt.Fprintln(c.ErrOrStderr(), unknown.Error())
			jsonout.ListFields(c.ErrOrStderr(), catalog)
			return ErrSilentArgs
		}
		var jqErr *jsonout.JQError
		if errors.As(err, &jqErr) {
			fmt.Fprintln(c.ErrOrStderr(), jqErr.Error())
			return ErrSilentArgs
		}
		return wrapTransport(c.ErrOrStderr(), r.Locale, "render JSON", err)
	}
	return nil
}

// repoIssueRowsToJSON maps repo-scope issues to the camelCase rows expected
// by jsonout.Render for itemJSONFields. Nil entries from the paginator are
// skipped. Caller may add extra keys (e.g. `category`) to each row before
// passing to renderJSONItems. State is "OPEN" because the source paginator
// (PaginateRepoIssues) only returns open issues.
func repoIssueRowsToJSON(issues []*queries.RepoIssue) []map[string]any {
	out := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		out = append(out, map[string]any{
			"id":        issue.Id,
			"number":    issue.Number,
			"state":     "OPEN",
			"title":     issue.Title,
			"type":      "ISSUE",
			"updatedAt": issue.UpdatedAt,
			"url":       issue.Url,
		})
	}
	return out
}

// projectItemRowsToJSON flattens project-scope items via projectitem.ContentOf
// so org/user-scope output matches the repo shape on the shared keys.
// `state` reflects the GitHub-side state of the underlying content (Issue /
// PR); draft items have no native state so `""` is emitted.
func projectItemRowsToJSON(items []*queries.ProjectV2ItemNode) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		c := projectitem.ContentOf(item)
		out = append(out, map[string]any{
			"id":        item.Id,
			"number":    c.Number,
			"state":     c.State,
			"title":     c.Title,
			"type":      contentTypeName(c.Typename),
			"updatedAt": item.UpdatedAt,
			"url":       c.URL,
		})
	}
	return out
}

// contentTypeName converts the GraphQL __typename of a project item content
// (`Issue` / `PullRequest` / `DraftIssue`) into the upper-snake-case wire
// format used in JSON output. Matches the GraphQL enum convention.
func contentTypeName(typename string) string {
	switch typename {
	case "Issue":
		return "ISSUE"
	case "PullRequest":
		return "PULL_REQUEST"
	case "DraftIssue":
		return "DRAFT_ISSUE"
	default:
		return "UNKNOWN"
	}
}

// withCategory returns a shallow copy of row with `category` set. Used by
// standup / review to mark each row with its activity bucket.
func withCategory(row map[string]any, category string) map[string]any {
	out := make(map[string]any, len(row)+1)
	for k, v := range row {
		out[k] = v
	}
	out["category"] = category
	return out
}
