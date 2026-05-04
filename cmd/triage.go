package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

const (
	triageDefaultLimit = 20
	triageFetchLimit   = 100
)

func newTriageCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "triage",
		Short: "Triage untriaged Issues / Project draft items",
		RunE: func(c *cobra.Command, _ []string) error {
			return runTriage(c.Context(), c, deps)
		},
	}
	c.Flags().Int("limit", triageDefaultLimit, "max number of untriaged items to show")
	return c
}

func runTriage(ctx context.Context, c *cobra.Command, deps Deps) error {
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
		limit = triageDefaultLimit
	}
	if sc == scope.Repo {
		return runTriageRepo(ctx, c, deps, r, limit)
	}
	return runTriageProject(ctx, c, deps, r, sc, limit)
}

func runTriageRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, limit int) error {
	id, err := repo.Resolve(repo.ResolveOptions{Argv: deps.Argv, GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	var resp queries.ListRepoIssuesWithLabelsResponse
	if err := clients.GraphQL.Do(ctx, queries.ListRepoIssuesWithLabels, map[string]any{
		"owner": id.Owner, "name": id.Name, "first": triageFetchLimit,
	}, &resp); err != nil {
		return fmt.Errorf("list repo issues with labels: %w", err)
	}
	if resp.Repository == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilent
	}
	hits := []queries.RepoIssueWithLabelsNode{}
	for _, n := range resp.Repository.Issues.Nodes {
		if len(n.Labels.Nodes) == 0 {
			hits = append(hits, n)
			if len(hits) >= limit {
				break
			}
		}
	}
	if len(hits) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("triage.empty"))
		return nil
	}
	fmt.Fprintf(c.OutOrStdout(), "%s (%d)\n", r.T("triage.found"), len(hits))
	for _, n := range hits {
		fmt.Fprintf(c.OutOrStdout(), "#%d  %s\n  %s\n", n.Number, n.Title, n.URL)
	}
	return nil
}

func runTriageProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, limit int) error {
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
		"projectId": pid, "first": triageFetchLimit,
	}, &resp); err != nil {
		return fmt.Errorf("list project items: %w", err)
	}
	if resp.Node == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilent
	}
	hits := []queries.ProjectV2ItemNode{}
	for _, item := range resp.Node.Items.Nodes {
		if isUntriaged(item) {
			hits = append(hits, item)
			if len(hits) >= limit {
				break
			}
		}
	}
	if len(hits) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("triage.empty.project"))
		return nil
	}
	fmt.Fprintf(c.OutOrStdout(), "%s (%d)\n", r.T("triage.found.project"), len(hits))
	for _, item := range hits {
		fmt.Fprint(c.OutOrStdout(), projectitem.FormatItem(item))
	}
	return nil
}

func isUntriaged(item queries.ProjectV2ItemNode) bool {
	status := projectitem.FindStatus(item.FieldValues.Nodes)
	if status == "" {
		return true
	}
	return strings.EqualFold(status, "triage")
}
