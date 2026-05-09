package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/jsonout"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

const defaultListLimit = 30

// listJSONFields is the catalog of `--json` fields exposed by `gh tasks list`.
// Phase 1 keeps the catalog deliberately small (#367 PR 1); future PRs add
// state / status / labels / assignees / milestone / iteration without
// breaking existing callers because field addition is non-breaking under
// the stability policy in `docs/design/json-output.md`.
var listJSONFields = jsonout.FieldList{
	{Name: "id", Description: "GraphQL global ID of the Issue or Project item"},
	{Name: "number", Description: "Issue / Project item number (0 for draft items)"},
	{Name: "title", Description: "Title of the Issue or Project item"},
	{Name: "type", Description: "ISSUE | PULL_REQUEST | DRAFT_ISSUE"},
	{Name: "updatedAt", Description: "Last-update timestamp (RFC 3339)"},
	{Name: "url", Description: "Absolute URL on github.com (empty for draft items)"},
}

func newListCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(c *cobra.Command, args []string) error {
			return runList(c.Context(), c, deps)
		},
	}
	c.Flags().Int("limit", defaultListLimit, "max number of items to list")
	// `--json` is a comma-separated list of field names. To list available
	// fields, pass an empty value: `--json=` or `--json ""`. We chose
	// String over StringSlice because StringSlice's NoOptDefVal interacts
	// badly with value-bearing invocations (NoOptDefVal hijacks the slot
	// even when the user passed a real value). Bare `--json` (no value
	// token) is rejected by cobra with the standard "flag needs an
	// argument" error, which is acceptable: the help text directs users
	// to the empty-value form.
	c.Flags().String("json", "", "output as JSON. Empty value (`--json=`) lists available fields.")
	c.Flags().String("jq", "", "filter JSON output via jq expression")
	return c
}

func runList(ctx context.Context, c *cobra.Command, deps Deps) error {
	r, err := deps.Resolve(c)
	if err != nil {
		return localizedError(c, r, err)
	}
	jsonReq, ok, err := resolveJSONRequest(c, r, listJSONFields)
	if err != nil {
		return err
	}
	jsonOn := ok
	sc, err := scope.Detect(scope.DetectOptions{
		Flag:         flagString(c, "scope"),
		HasGitRemote: deps.HasGitRemote,
		DefaultScope: r.Config.DefaultScope,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	limit, _ := c.Flags().GetInt("limit")
	// Defensive default-back: cobra's IntFlag default is defaultListLimit, but
	// explicit `--limit=0` or negative values are not valid and were never
	// honoured by the legacy TS implementation either, so fall back to the
	// default. Kept as documentation rather than a pflag.Var validator to
	// preserve the TS toLimit() parity.
	if limit <= 0 {
		limit = defaultListLimit
	}
	if sc == scope.Repo {
		return runListRepo(ctx, c, deps, r, limit, jsonOn, jsonReq)
	}
	return runListProject(ctx, c, deps, r, sc, limit, jsonOn, jsonReq)
}

func runListRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, limit int, jsonOn bool, jsonReq jsonRequest) error {
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	issues, err := queries.PaginateRepoIssues(ctx, clients.AsGenqlientClient(), id.Owner, id.Name, limit)
	if errors.Is(err, queries.ErrRepoNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list repo issues", err)
	}
	if jsonOn {
		return renderListJSON(c, r, repoIssuesToJSON(issues), jsonReq)
	}
	if len(issues) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("list.empty"))
		return nil
	}
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		fmt.Fprintf(c.OutOrStdout(), "#%d  %s\n  %s\n", issue.Number, issue.Title, issue.Url)
	}
	return nil
}

func runListProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, limit int, jsonOn bool, jsonReq jsonRequest) error {
	pref, err := project.Resolve(project.ResolveOptions{
		Scope:       sc,
		Flag:        flagString(c, "project"),
		OrgProject:  r.Config.OrgProject,
		UserProject: r.Config.UserProject,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	pid, err := projectitem.ResolveProjectNodeID(ctx, clients.GraphQL, sc, pref)
	if err != nil {
		return localizedError(c, r, err)
	}
	if pid == "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	items, err := queries.PaginateProjectV2Items(ctx, clients.AsGenqlientClient(), pid, limit)
	if errors.Is(err, queries.ErrProjectNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list project items", err)
	}
	if jsonOn {
		return renderListJSON(c, r, projectItemsToJSON(items), jsonReq)
	}
	if len(items) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("list.empty.project"))
		return nil
	}
	for _, item := range items {
		fmt.Fprint(c.OutOrStdout(), projectitem.FormatItem(item))
	}
	return nil
}

// repoIssuesToJSON maps repo-scope issues to the camelCase rows expected
// by jsonout.Render. Nil entries from the paginator are skipped.
func repoIssuesToJSON(issues []*queries.RepoIssue) []map[string]any {
	out := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		out = append(out, map[string]any{
			"id":        issue.Id,
			"number":    issue.Number,
			"title":     issue.Title,
			"type":      "ISSUE",
			"updatedAt": issue.UpdatedAt,
			"url":       issue.Url,
		})
	}
	return out
}

// projectItemsToJSON flattens project-scope items via projectitem.ContentOf
// so org/user-scope output matches the repo shape on the shared keys.
// Draft items have no GitHub URL or number, so `url` is "" and `number` is
// 0 — both kept as JSON values rather than null because the underlying type
// has a zero value (consumers can filter via `.url != ""` if needed).
func projectItemsToJSON(items []*queries.ProjectV2ItemNode) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		c := projectitem.ContentOf(item)
		out = append(out, map[string]any{
			"id":        item.Id,
			"number":    c.Number,
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

// jsonRequest carries the resolved --json / --jq flag values between the
// runList dispatcher and the per-scope handlers.
type jsonRequest struct {
	fields []string
	jq     string
}

// resolveJSONRequest reads --json / --jq from the command, validates them,
// and returns (request, jsonOn, error). When --json is given without a
// value, the catalog is printed to stderr and ErrSilentArgs is returned.
// --jq without --json is also rejected (text mode and JSON filtering are
// not mixed; see json-output.md "CLI surface").
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
// trimming whitespace. Bare `--json` arrives as "" thanks to NoOptDefVal
// and naturally produces an empty slice — handled by the caller as
// "list available fields".
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

// renderListJSON writes items as JSON to stdout, falling back to the
// catalog listing on UnknownFieldError so the user can self-correct
// without re-reading docs. Other errors map to ErrSilentRuntime via the
// transport wrapper since they signal an internal rendering bug.
func renderListJSON(c *cobra.Command, r Resolved, items []map[string]any, req jsonRequest) error {
	if err := jsonout.Render(c.OutOrStdout(), items, req.fields, req.jq, listJSONFields); err != nil {
		var unknown *jsonout.UnknownFieldError
		if errors.As(err, &unknown) {
			fmt.Fprintln(c.ErrOrStderr(), unknown.Error())
			jsonout.ListFields(c.ErrOrStderr(), listJSONFields)
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

// localizedError renders any internal error carrying an i18n.Localized payload
// using the resolved locale, prints it to stderr, and returns either
// [ErrSilentArgs] (when err is a known arg-validation domain error type) or
// [ErrSilentRuntime] (otherwise). Errors that don't implement Localized are
// returned as-is so cobra surfaces them normally.
func localizedError(c *cobra.Command, r Resolved, err error) error {
	var loc i18n.Localized
	if errors.As(err, &loc) {
		fmt.Fprintln(c.ErrOrStderr(), loc.Localize(r.Locale))
		if classifyArgError(err) {
			return ErrSilentArgs
		}
		return ErrSilentRuntime
	}
	return err
}
