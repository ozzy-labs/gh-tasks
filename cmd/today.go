package cmd

import (
	"context"
	"errors"
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
	c := &cobra.Command{
		Use:   "today",
		Short: "Show today's tasks",
		RunE: func(c *cobra.Command, _ []string) error {
			return runToday(c.Context(), c, deps)
		},
	}
	addJSONFlags(c)
	addJSONCompletion(c, itemJSONFields)
	return c
}

func runToday(ctx context.Context, c *cobra.Command, deps Deps) error {
	r, err := deps.Resolve(c)
	if err != nil {
		return localizedError(c, r, err)
	}
	jsonReq, jsonOn, err := resolveJSONRequest(c, r, itemJSONFields)
	if err != nil {
		return err
	}
	sc, err := scope.Detect(scope.DetectOptions{
		Flag:         flagString(c, "scope"),
		HasGitRemote: deps.HasGitRemote,
		DefaultScope: r.Config.DefaultScope,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	startUTC, endUTC := todayRange(deps.Now())
	if sc == scope.Repo {
		return runTodayRepo(ctx, c, deps, r, startUTC, endUTC, jsonOn, jsonReq)
	}
	return runTodayProject(ctx, c, deps, r, sc, startUTC, endUTC, jsonOn, jsonReq)
}

func runTodayRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, startUTC, endUTC time.Time, jsonOn bool, jsonReq jsonRequest) error {
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	issues, err := queries.PaginateRepoIssues(ctx, clients.AsGenqlientClient(), id.Owner, id.Name, todayFetchLimit)
	if errors.Is(err, queries.ErrRepoNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list repo issues", err)
	}
	hits := []*queries.RepoIssue{}
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, issue.UpdatedAt)
		if err != nil {
			continue
		}
		if (t.Equal(startUTC) || t.After(startUTC)) && t.Before(endUTC) {
			hits = append(hits, issue)
		}
	}
	if jsonOn {
		return renderJSONItems(c, r, repoIssueRowsToJSON(hits), jsonReq, itemJSONFields)
	}
	if len(hits) == 0 {
		fmt.Fprintln(c.OutOrStdout(), r.T("today.empty"))
		return nil
	}
	for _, issue := range hits {
		fmt.Fprintf(c.OutOrStdout(), "#%d  %s\n  %s\n", issue.Number, issue.Title, issue.Url)
	}
	return nil
}

func runTodayProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, startUTC, endUTC time.Time, jsonOn bool, jsonReq jsonRequest) error {
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
	items, err := queries.PaginateProjectV2Items(ctx, clients.AsGenqlientClient(), pid, todayFetchLimit)
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
		t, err := time.Parse(time.RFC3339, item.UpdatedAt)
		if err != nil {
			continue
		}
		if (t.Equal(startUTC) || t.After(startUTC)) && t.Before(endUTC) {
			hits = append(hits, item)
		}
	}
	if jsonOn {
		return renderJSONItems(c, r, projectItemRowsToJSON(hits), jsonReq, itemJSONFields)
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
