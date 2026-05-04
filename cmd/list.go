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
	r, err := deps.Resolve()
	if err != nil {
		return localizedError(c, r, err)
	}
	sc, err := scope.Detect(scope.DetectOptions{
		Argv:         deps.Argv,
		HasGitRemote: deps.HasGitRemote,
		DefaultScope: r.Config.DefaultScope,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	limit, _ := c.Flags().GetInt("limit")
	if limit <= 0 {
		limit = defaultListLimit
	}
	if sc == scope.Repo {
		return runListRepo(ctx, c, deps, r, limit)
	}
	return runListProject(ctx, c, deps, r, sc, limit)
}

func runListRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, limit int) error {
	id, err := repo.Resolve(repo.ResolveOptions{Argv: deps.Argv, GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	var resp queries.ListRepoIssuesResponse
	if err := clients.GraphQL.Do(ctx, queries.ListRepoIssues, map[string]any{
		"owner": id.Owner, "name": id.Name, "first": limit,
	}, &resp); err != nil {
		return err
	}
	if resp.Repository == nil {
		fmt.Fprintf(c.ErrOrStderr(), "repository not found: %s/%s\n", id.Owner, id.Name)
		return ErrSilent
	}
	if len(resp.Repository.Issues.Nodes) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("list.empty"))
		return nil
	}
	for _, issue := range resp.Repository.Issues.Nodes {
		fmt.Fprintf(c.OutOrStdout(), "#%d  %s\n  %s\n", issue.Number, issue.Title, issue.URL)
	}
	return nil
}

func runListProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, limit int) error {
	pref, err := project.Resolve(project.ResolveOptions{
		Scope:       sc,
		Argv:        deps.Argv,
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
		fmt.Fprintf(c.ErrOrStderr(), "project not found: %s/%d (--scope %s)\n", pref.Owner, pref.Number, sc)
		return ErrSilent
	}
	var resp queries.ListProjectV2ItemsResponse
	if err := clients.GraphQL.Do(ctx, queries.ListProjectV2Items, map[string]any{
		"projectId": pid, "first": limit,
	}, &resp); err != nil {
		return err
	}
	if resp.Node == nil {
		fmt.Fprintf(c.ErrOrStderr(), "project not found: %s/%d (--scope %s)\n", pref.Owner, pref.Number, sc)
		return ErrSilent
	}
	if len(resp.Node.Items.Nodes) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("list.empty.project"))
		return nil
	}
	for _, item := range resp.Node.Items.Nodes {
		fmt.Fprint(c.OutOrStdout(), projectitem.FormatItem(item))
	}
	return nil
}

// ErrSilent signals that an error has already been written to stderr and the
// caller should exit non-zero without a duplicate error print.
var ErrSilent = errors.New("silent error")

// localizedError renders any internal error carrying an i18n.Localized payload
// using the resolved locale, prints it to stderr, and returns ErrSilent.
// Errors that don't implement Localized are returned as-is.
func localizedError(c *cobra.Command, r Resolved, err error) error {
	var loc i18n.Localized
	if errors.As(err, &loc) {
		fmt.Fprintln(c.ErrOrStderr(), i18n.T(r.Locale, loc.I18nKey(), i18n.Flat(loc.I18nArgs())...))
		return ErrSilent
	}
	return err
}
