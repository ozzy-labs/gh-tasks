package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

const standupFetchLimit = 100

func newStandupCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "standup",
		Short: "Generate standup summary of recent activity",
		RunE: func(c *cobra.Command, _ []string) error {
			return runStandup(c.Context(), c, deps)
		},
	}
	c.Flags().String("since", "", "ISO-8601 timestamp to anchor the window (default: 24h ago)")
	c.Flags().Bool("mine", false, "filter to items where the viewer is author or assignee")
	return c
}

func runStandup(ctx context.Context, c *cobra.Command, deps Deps) error {
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
	now := time.Now()
	if deps.Now != nil {
		now = deps.Now()
	}
	since := standupSince(c, now)
	mine, _ := c.Flags().GetBool("mine")
	if sc == scope.Repo {
		return runStandupRepo(ctx, c, deps, r, since, mine)
	}
	return runStandupProject(ctx, c, deps, r, sc, since, mine)
}

func runStandupRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, since time.Time, mine bool) error {
	id, err := repo.Resolve(repo.ResolveOptions{Argv: deps.Argv, GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	viewerLogin := ""
	if mine {
		var v queries.GetViewerLoginResponse
		if err := clients.GraphQL.Do(ctx, queries.GetViewerLogin, nil, &v); err != nil {
			return err
		}
		viewerLogin = v.Viewer.Login
	}
	var closedResp queries.ListClosedIssuesResponse
	var prsResp queries.ListMergedPRsResponse
	var openResp queries.ListRepoIssuesResponse
	q := map[string]any{"owner": id.Owner, "name": id.Name, "first": standupFetchLimit}
	if err := clients.GraphQL.Do(ctx, queries.ListClosedIssues, q, &closedResp); err != nil {
		return err
	}
	if err := clients.GraphQL.Do(ctx, queries.ListMergedPRs, q, &prsResp); err != nil {
		return err
	}
	if err := clients.GraphQL.Do(ctx, queries.ListRepoIssues, q, &openResp); err != nil {
		return err
	}

	closed := []queries.ClosedIssueNode{}
	if closedResp.Repository != nil {
		for _, n := range closedResp.Repository.Issues.Nodes {
			if !timeAtOrAfter(n.ClosedAt, since) {
				continue
			}
			if !matchesViewerClosed(n, viewerLogin) {
				continue
			}
			closed = append(closed, n)
		}
	}
	merged := []queries.MergedPRNode{}
	if prsResp.Repository != nil {
		for _, n := range prsResp.Repository.PullRequests.Nodes {
			if !timeAtOrAfter(n.MergedAt, since) {
				continue
			}
			if !matchesViewerMerged(n, viewerLogin) {
				continue
			}
			merged = append(merged, n)
		}
	}
	open := []queries.RepoIssueNode{}
	if openResp.Repository != nil {
		for _, n := range openResp.Repository.Issues.Nodes {
			if !timeAtOrAfter(n.UpdatedAt, since) {
				continue
			}
			if !matchesViewerOpen(n, viewerLogin) {
				continue
			}
			open = append(open, n)
		}
	}

	out := c.OutOrStdout()
	header := r.T("standup.heading")
	if mine && viewerLogin != "" {
		header = fmt.Sprintf("%s (@%s)", header, viewerLogin)
	}
	fmt.Fprintf(out, "# %s\n", header)
	fmt.Fprintf(out, "since %s\n\n", since.UTC().Format(time.RFC3339))
	fmt.Fprintf(out, "## %s\n", r.T("standup.yesterday"))
	if len(closed) == 0 && len(merged) == 0 {
		fmt.Fprintf(out, "- %s\n", r.T("standup.none"))
	} else {
		for _, i := range closed {
			fmt.Fprintf(out, "- closed: #%d %s (%s)\n", i.Number, i.Title, i.URL)
		}
		for _, p := range merged {
			fmt.Fprintf(out, "- merged: #%d %s (%s)\n", p.Number, p.Title, p.URL)
		}
	}
	fmt.Fprintf(out, "\n## %s\n", r.T("standup.today"))
	if len(open) == 0 {
		fmt.Fprintf(out, "- %s\n", r.T("standup.none"))
	} else {
		for _, i := range open {
			fmt.Fprintf(out, "- in-progress: #%d %s (%s)\n", i.Number, i.Title, i.URL)
		}
	}
	fmt.Fprintf(out, "\n## %s\n- %s\n\n", r.T("standup.blockers"), r.T("standup.blockersHint"))
	return nil
}

func runStandupProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, since time.Time, mine bool) error {
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
	viewerLogin := ""
	if mine {
		var v queries.GetViewerLoginResponse
		if err := clients.GraphQL.Do(ctx, queries.GetViewerLogin, nil, &v); err != nil {
			return err
		}
		viewerLogin = v.Viewer.Login
	}
	pid, err := projectitem.ResolveProjectNodeID(ctx, clients.GraphQL, sc, pref)
	if err != nil {
		return err
	}
	if pid == "" {
		fmt.Fprintf(c.ErrOrStderr(), "project not found: %s/%d (--scope %s)\n", pref.Owner, pref.Number, sc)
		return errSilent
	}
	var resp queries.ListProjectV2ItemsResponse
	if err := clients.GraphQL.Do(ctx, queries.ListProjectV2Items, map[string]any{
		"projectId": pid, "first": standupFetchLimit,
	}, &resp); err != nil {
		return err
	}
	if resp.Node == nil {
		fmt.Fprintf(c.ErrOrStderr(), "project not found: %s/%d (--scope %s)\n", pref.Owner, pref.Number, sc)
		return errSilent
	}
	yesterday := []queries.ProjectV2ItemNode{}
	today := []queries.ProjectV2ItemNode{}
	for _, item := range resp.Node.Items.Nodes {
		if !timeAtOrAfter(item.UpdatedAt, since) {
			continue
		}
		if viewerLogin != "" && !matchesViewerOnItem(item, viewerLogin) {
			continue
		}
		if isItemDone(item) {
			yesterday = append(yesterday, item)
		} else {
			today = append(today, item)
		}
	}
	out := c.OutOrStdout()
	header := r.T("standup.heading")
	if mine && viewerLogin != "" {
		header = fmt.Sprintf("%s (@%s)", header, viewerLogin)
	}
	fmt.Fprintf(out, "# %s\n", header)
	fmt.Fprintf(out, "since %s\n\n", since.UTC().Format(time.RFC3339))
	fmt.Fprintf(out, "## %s\n", r.T("standup.yesterday"))
	if len(yesterday) == 0 {
		fmt.Fprintf(out, "- %s\n", r.T("standup.empty.project"))
	} else {
		for _, item := range yesterday {
			fmt.Fprintf(out, "- done: %s\n", projectitem.FormatItemLineCompact(item))
		}
	}
	fmt.Fprintf(out, "\n## %s\n", r.T("standup.today"))
	if len(today) == 0 {
		fmt.Fprintf(out, "- %s\n", r.T("standup.empty.project"))
	} else {
		for _, item := range today {
			fmt.Fprintf(out, "- in-progress: %s\n", projectitem.FormatItemLineCompact(item))
		}
	}
	fmt.Fprintf(out, "\n## %s\n- %s\n\n", r.T("standup.blockers"), r.T("standup.blockersHint"))
	return nil
}

func standupSince(c *cobra.Command, now time.Time) time.Time {
	since, _ := c.Flags().GetString("since")
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			return t
		}
	}
	return now.Add(-24 * time.Hour)
}

func timeAtOrAfter(iso string, threshold time.Time) bool {
	if iso == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return false
	}
	return t.Equal(threshold) || t.After(threshold)
}

func matchesViewerClosed(n queries.ClosedIssueNode, viewer string) bool {
	if viewer == "" {
		return true
	}
	if n.Author != nil && n.Author.Login == viewer {
		return true
	}
	for _, a := range n.Assignees.Nodes {
		if a.Login == viewer {
			return true
		}
	}
	return false
}

func matchesViewerMerged(n queries.MergedPRNode, viewer string) bool {
	if viewer == "" {
		return true
	}
	if n.Author != nil && n.Author.Login == viewer {
		return true
	}
	for _, a := range n.Assignees.Nodes {
		if a.Login == viewer {
			return true
		}
	}
	return false
}

func matchesViewerOpen(n queries.RepoIssueNode, viewer string) bool {
	if viewer == "" {
		return true
	}
	if n.Author != nil && n.Author.Login == viewer {
		return true
	}
	for _, a := range n.Assignees.Nodes {
		if a.Login == viewer {
			return true
		}
	}
	return false
}

func matchesViewerOnItem(item queries.ProjectV2ItemNode, viewer string) bool {
	c := item.Content
	if c == nil || c.Typename == "DraftIssue" {
		return false
	}
	if c.Author != nil && c.Author.Login == viewer {
		return true
	}
	if c.Assignees != nil {
		for _, a := range c.Assignees.Nodes {
			if a.Login == viewer {
				return true
			}
		}
	}
	return false
}

func isItemDone(item queries.ProjectV2ItemNode) bool {
	status := projectitem.FindStatus(item.FieldValues.Nodes)
	if status == "" {
		return false
	}
	return strings.EqualFold(status, "done")
}
