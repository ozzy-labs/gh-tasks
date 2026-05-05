package cmd

import (
	"context"
	"errors"
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
	if limit <= 0 {
		limit = triageDefaultLimit
	}
	if sc == scope.Repo {
		return runTriageRepo(ctx, c, deps, r, limit)
	}
	return runTriageProject(ctx, c, deps, r, sc, limit)
}

func runTriageRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, limit int) error {
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	issues, err := queries.PaginateRepoIssuesWithLabels(ctx, clients.AsGenqlientClient(), id.Owner, id.Name, triageFetchLimit)
	if errors.Is(err, queries.ErrRepoNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list repo issues with labels", err)
	}
	type triageHit struct {
		Number int
		Title  string
		URL    string
	}
	hits := []triageHit{}
	for _, n := range issues {
		if n == nil {
			continue
		}
		if len(n.Labels.Nodes) == 0 {
			hits = append(hits, triageHit{Number: n.Number, Title: n.Title, URL: n.Url})
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
	items, err := queries.PaginateProjectV2Items(ctx, clients.AsGenqlientClient(), pid, triageFetchLimit)
	if errors.Is(err, queries.ErrProjectNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list project items", err)
	}
	hits := []*queries.ProjectV2ItemNode{}
	for _, item := range items {
		if item == nil {
			continue
		}
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

func isUntriaged(item *queries.ProjectV2ItemNode) bool {
	status := projectitem.FindStatus(projectitem.FieldValuesOf(item))
	if status == "" {
		return true
	}
	return strings.EqualFold(status, "triage")
}
