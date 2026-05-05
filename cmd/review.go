package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/period"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

const reviewFetchLimit = 100

func newReviewCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "review",
		Short: "Generate retrospective summary",
		RunE: func(c *cobra.Command, _ []string) error {
			return runReview(c.Context(), c, deps)
		},
	}
	c.Flags().String("period", "weekly", "aggregation window: daily | weekly | sprint")
	return c
}

func runReview(ctx context.Context, c *cobra.Command, deps Deps) error {
	r, err := deps.Resolve()
	if err != nil {
		return localizedError(c, r, err)
	}
	pflag, _ := c.Flags().GetString("period")
	p, err := period.Parse(pflag)
	if err != nil {
		return localizedError(c, r, err)
	}
	rng := period.Of(p, period.Options{Getenv: deps.Env, Now: deps.Now()})
	sc, err := scope.Detect(scope.DetectOptions{
		Argv:         deps.Argv,
		HasGitRemote: deps.HasGitRemote,
		DefaultScope: r.Config.DefaultScope,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	if sc == scope.Repo {
		return runReviewRepo(ctx, c, deps, r, p, rng)
	}
	return runReviewProject(ctx, c, deps, r, sc, p, rng)
}

func runReviewRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, p period.Period, rng period.Range) error {
	id, err := repo.Resolve(repo.ResolveOptions{Argv: deps.Argv, GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	gqlClient := clients.AsGenqlientClient()
	closedResp, err := queries.ListClosedIssues(ctx, gqlClient, id.Owner, id.Name, reviewFetchLimit)
	if err != nil {
		return fmt.Errorf("list closed issues: %w", err)
	}
	prsResp, err := queries.ListMergedPRs(ctx, gqlClient, id.Owner, id.Name, reviewFetchLimit)
	if err != nil {
		return fmt.Errorf("list merged PRs: %w", err)
	}
	type closedItem struct {
		Number int
		Title  string
		URL    string
	}
	closed := []closedItem{}
	if closedResp.Repository != nil {
		for _, n := range closedResp.Repository.Issues.Nodes {
			closedAt := ""
			if n.ClosedAt != nil {
				closedAt = *n.ClosedAt
			}
			if withinPeriodRange(closedAt, rng) {
				closed = append(closed, closedItem{Number: n.Number, Title: n.Title, URL: n.Url})
			}
		}
	}
	type mergedItem struct {
		Number int
		Title  string
		URL    string
	}
	merged := []mergedItem{}
	if prsResp.Repository != nil {
		for _, n := range prsResp.Repository.PullRequests.Nodes {
			mergedAt := ""
			if n.MergedAt != nil {
				mergedAt = *n.MergedAt
			}
			if withinPeriodRange(mergedAt, rng) {
				merged = append(merged, mergedItem{Number: n.Number, Title: n.Title, URL: n.Url})
			}
		}
	}
	out := c.OutOrStdout()
	fmt.Fprintf(out, "# %s (%s)\n", r.T("review.heading"), r.T("review.period."+string(p)))
	fmt.Fprintf(out, "%s → %s\n\n", rng.Start.Format("2006-01-02"), rng.End.Format("2006-01-02"))
	fmt.Fprintf(out, "## %s (%d)\n", r.T("review.closedIssues"), len(closed))
	if len(closed) == 0 {
		fmt.Fprintf(out, "- %s\n", r.T("review.none"))
	} else {
		for _, i := range closed {
			fmt.Fprintf(out, "- #%d %s (%s)\n", i.Number, i.Title, i.URL)
		}
	}
	fmt.Fprintf(out, "\n## %s (%d)\n", r.T("review.mergedPRs"), len(merged))
	if len(merged) == 0 {
		fmt.Fprintf(out, "- %s\n", r.T("review.none"))
	} else {
		for _, p := range merged {
			fmt.Fprintf(out, "- #%d %s (%s)\n", p.Number, p.Title, p.URL)
		}
	}
	fmt.Fprintln(out)
	return nil
}

func runReviewProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, p period.Period, rng period.Range) error {
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
		return ErrSilentRuntime
	}
	var resp queries.ListProjectV2ItemsResponse
	if err := clients.GraphQL.Do(ctx, queries.ListProjectV2Items, map[string]any{
		"projectId": pid, "first": reviewFetchLimit,
	}, &resp); err != nil {
		return fmt.Errorf("list project items: %w", err)
	}
	if resp.Node == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	completed := []queries.ProjectV2ItemNode{}
	for _, item := range resp.Node.Items.Nodes {
		if withinPeriodRange(item.UpdatedAt, rng) && isItemDone(item) {
			completed = append(completed, item)
		}
	}
	out := c.OutOrStdout()
	fmt.Fprintf(out, "# %s (%s)\n", r.T("review.heading"), r.T("review.period."+string(p)))
	fmt.Fprintf(out, "%s → %s\n\n", rng.Start.Format("2006-01-02"), rng.End.Format("2006-01-02"))
	fmt.Fprintf(out, "## %s (%d)\n", r.T("review.completedProjectItems"), len(completed))
	if len(completed) == 0 {
		fmt.Fprintf(out, "- %s\n", r.T("review.empty.project"))
	} else {
		for _, item := range completed {
			fmt.Fprintf(out, "- %s\n", projectitem.FormatItemLineCompact(item))
		}
	}
	fmt.Fprintln(out)
	return nil
}

func withinPeriodRange(iso string, rng period.Range) bool {
	if iso == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return false
	}
	return (t.Equal(rng.Start) || t.After(rng.Start)) && t.Before(rng.End)
}
