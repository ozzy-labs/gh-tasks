package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

const todayFetchLimit = 100

func newTodayCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "today",
		Short: "Show today's tasks",
		RunE: func(c *cobra.Command, _ []string) error {
			return runToday(c.Context(), c, deps)
		},
	}
}

func runToday(ctx context.Context, c *cobra.Command, deps Deps) error {
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
	startUTC, endUTC := todayRange(deps.Now())
	if sc == scope.Repo {
		return runTodayRepo(ctx, c, deps, r, startUTC, endUTC)
	}
	return runTodayProject(ctx, c, deps, r, sc, startUTC, endUTC)
}

func runTodayRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, startUTC, endUTC time.Time) error {
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
		"owner": id.Owner, "name": id.Name, "first": todayFetchLimit,
	}, &resp); err != nil {
		return fmt.Errorf("list repo issues: %w", err)
	}
	if resp.Repository == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilent
	}
	hits := []queries.RepoIssueNode{}
	for _, issue := range resp.Repository.Issues.Nodes {
		t, err := time.Parse(time.RFC3339, issue.UpdatedAt)
		if err != nil {
			continue
		}
		if (t.Equal(startUTC) || t.After(startUTC)) && t.Before(endUTC) {
			hits = append(hits, issue)
		}
	}
	if len(hits) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("today.empty"))
		return nil
	}
	for _, issue := range hits {
		fmt.Fprintf(c.OutOrStdout(), "#%d  %s\n  %s\n", issue.Number, issue.Title, issue.URL)
	}
	return nil
}

func runTodayProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, startUTC, endUTC time.Time) error {
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
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilent
	}
	var resp queries.ListProjectV2ItemsResponse
	if err := clients.GraphQL.Do(ctx, queries.ListProjectV2Items, map[string]any{
		"projectId": pid, "first": todayFetchLimit,
	}, &resp); err != nil {
		return fmt.Errorf("list project items: %w", err)
	}
	if resp.Node == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilent
	}
	hits := []queries.ProjectV2ItemNode{}
	for _, item := range resp.Node.Items.Nodes {
		t, err := time.Parse(time.RFC3339, item.UpdatedAt)
		if err != nil {
			continue
		}
		if (t.Equal(startUTC) || t.After(startUTC)) && t.Before(endUTC) {
			hits = append(hits, item)
		}
	}
	if len(hits) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("today.empty.project"))
		return nil
	}
	for _, item := range hits {
		fmt.Fprint(c.OutOrStdout(), projectitem.FormatItem(item))
	}
	return nil
}

// todayRange returns [start, end) anchored at UTC midnight matching the TS
// implementation: identical on a JST dev box and a UTC CI runner.
func todayRange(now time.Time) (time.Time, time.Time) {
	utc := now.UTC()
	start := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return start, start.Add(24 * time.Hour)
}
