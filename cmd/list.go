package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

const defaultListLimit = 30

func newListCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(c *cobra.Command, args []string) error {
			return runList(c.Context(), c, deps)
		},
	}
	c.Flags().Int("limit", defaultListLimit, "max number of items to list")
	return c
}

func runList(ctx context.Context, c *cobra.Command, deps Deps) error {
	r, err := deps.Resolve(c)
	if err != nil {
		return localizedError(c, r, err)
	}
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
		return runListRepo(ctx, c, deps, r, limit)
	}
	return runListProject(ctx, c, deps, r, sc, limit)
}

func runListRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, limit int) error {
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	resp, err := queries.ListRepoIssues(ctx, clients.AsGenqlientClient(), id.Owner, id.Name, limit)
	if err != nil {
		return fmt.Errorf("list repo issues: %w", err)
	}
	if resp.Repository == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	if len(resp.Repository.Issues.Nodes) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("list.empty"))
		return nil
	}
	for _, issue := range resp.Repository.Issues.Nodes {
		fmt.Fprintf(c.OutOrStdout(), "#%d  %s\n  %s\n", issue.Number, issue.Title, issue.Url)
	}
	return nil
}

func runListProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, limit int) error {
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
		return err
	}
	if pid == "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	resp, err := queries.ListProjectV2Items(ctx, clients.AsGenqlientClient(), pid, limit)
	if err != nil {
		return fmt.Errorf("list project items: %w", err)
	}
	if !projectitem.HasProjectNode(resp) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	items := projectitem.ItemsFromResponse(resp)
	if len(items) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("list.empty.project"))
		return nil
	}
	for _, item := range items {
		fmt.Fprint(c.OutOrStdout(), projectitem.FormatItem(item))
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
