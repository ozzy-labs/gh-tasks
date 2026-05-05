package cmd

import (
	"context"
	"errors"
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
	since, err := standupSince(c, deps.Now())
	if err != nil {
		return localizedError(c, r, err)
	}
	mine, _ := c.Flags().GetBool("mine")
	if sc == scope.Repo {
		return runStandupRepo(ctx, c, deps, r, since, mine)
	}
	return runStandupProject(ctx, c, deps, r, sc, since, mine)
}

func runStandupRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, since time.Time, mine bool) error {
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	viewerLogin := ""
	if mine {
		v, err := queries.GetViewerLogin(ctx, clients.AsGenqlientClient())
		if err != nil {
			return wrapTransport(c.ErrOrStderr(), r.Locale, "get viewer login", err)
		}
		if v == nil || v.Viewer == nil || v.Viewer.Login == "" {
			return localizedError(c, r, newRuntimeError("error.standup.viewerLoginUnresolved"))
		}
		viewerLogin = v.Viewer.Login
	}
	gqlClient := clients.AsGenqlientClient()
	closedIssues, err := queries.PaginateClosedIssues(ctx, gqlClient, id.Owner, id.Name, standupFetchLimit)
	if errors.Is(err, queries.ErrRepoNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list closed issues", err)
	}
	mergedPRs, err := queries.PaginateMergedPRs(ctx, gqlClient, id.Owner, id.Name, standupFetchLimit)
	if errors.Is(err, queries.ErrRepoNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list merged PRs", err)
	}
	openIssues, err := queries.PaginateRepoIssues(ctx, gqlClient, id.Owner, id.Name, standupFetchLimit)
	if errors.Is(err, queries.ErrRepoNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list repo issues", err)
	}

	type closedItem struct {
		Number int
		Title  string
		URL    string
	}
	closed := []closedItem{}
	for _, n := range closedIssues {
		if n == nil {
			continue
		}
		closedAt := ""
		if n.ClosedAt != nil {
			closedAt = *n.ClosedAt
		}
		if !timeAtOrAfter(closedAt, since) {
			continue
		}
		authorLogin := ""
		if n.Author != nil && *n.Author != nil {
			authorLogin = (*n.Author).GetLogin()
		}
		var assignees []string
		if n.Assignees != nil {
			assignees = make([]string, 0, len(n.Assignees.Nodes))
			for _, a := range n.Assignees.Nodes {
				if a != nil {
					assignees = append(assignees, a.Login)
				}
			}
		}
		if !matchesViewer(authorLogin, assignees, viewerLogin) {
			continue
		}
		closed = append(closed, closedItem{Number: n.Number, Title: n.Title, URL: n.Url})
	}
	type mergedItem struct {
		Number int
		Title  string
		URL    string
	}
	merged := []mergedItem{}
	for _, n := range mergedPRs {
		if n == nil {
			continue
		}
		mergedAt := ""
		if n.MergedAt != nil {
			mergedAt = *n.MergedAt
		}
		if !timeAtOrAfter(mergedAt, since) {
			continue
		}
		authorLogin := ""
		if n.Author != nil && *n.Author != nil {
			authorLogin = (*n.Author).GetLogin()
		}
		var assignees []string
		if n.Assignees != nil {
			assignees = make([]string, 0, len(n.Assignees.Nodes))
			for _, a := range n.Assignees.Nodes {
				if a != nil {
					assignees = append(assignees, a.Login)
				}
			}
		}
		if !matchesViewer(authorLogin, assignees, viewerLogin) {
			continue
		}
		merged = append(merged, mergedItem{Number: n.Number, Title: n.Title, URL: n.Url})
	}
	type openItem struct {
		Number int
		Title  string
		URL    string
	}
	open := []openItem{}
	for _, n := range openIssues {
		if n == nil {
			continue
		}
		if !timeAtOrAfter(n.UpdatedAt, since) {
			continue
		}
		authorLogin := ""
		if n.Author != nil && *n.Author != nil {
			authorLogin = (*n.Author).GetLogin()
		}
		var assignees []string
		if n.Assignees != nil {
			assignees = make([]string, 0, len(n.Assignees.Nodes))
			for _, a := range n.Assignees.Nodes {
				if a != nil {
					assignees = append(assignees, a.Login)
				}
			}
		}
		if !matchesViewer(authorLogin, assignees, viewerLogin) {
			continue
		}
		open = append(open, openItem{Number: n.Number, Title: n.Title, URL: n.Url})
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
	fmt.Fprintf(out, "\n## %s\n- %s\n", r.T("standup.blockers"), r.T("standup.blockersHint"))
	return nil
}

func runStandupProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, since time.Time, mine bool) error {
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
	viewerLogin := ""
	if mine {
		v, err := queries.GetViewerLogin(ctx, clients.AsGenqlientClient())
		if err != nil {
			return wrapTransport(c.ErrOrStderr(), r.Locale, "get viewer login", err)
		}
		if v == nil || v.Viewer == nil || v.Viewer.Login == "" {
			return localizedError(c, r, newRuntimeError("error.standup.viewerLoginUnresolved"))
		}
		viewerLogin = v.Viewer.Login
	}
	pid, err := projectitem.ResolveProjectNodeID(ctx, clients.GraphQL, sc, pref)
	if err != nil {
		return localizedError(c, r, err)
	}
	if pid == "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	items, err := queries.PaginateProjectV2Items(ctx, clients.AsGenqlientClient(), pid, standupFetchLimit)
	if errors.Is(err, queries.ErrProjectNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list project items", err)
	}
	yesterday := []*queries.ProjectV2ItemNode{}
	today := []*queries.ProjectV2ItemNode{}
	for _, item := range items {
		if item == nil {
			continue
		}
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
	fmt.Fprintf(out, "\n## %s\n- %s\n", r.T("standup.blockers"), r.T("standup.blockersHint"))
	return nil
}

func standupSince(c *cobra.Command, now time.Time) (time.Time, error) {
	since, _ := c.Flags().GetString("since")
	if since == "" {
		return now.Add(-24 * time.Hour), nil
	}
	t, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return time.Time{}, newArgError("error.standup.invalidSince", "value", since)
	}
	return t, nil
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

func matchesViewer(authorLogin string, assigneeLogins []string, viewer string) bool {
	if viewer == "" {
		return true
	}
	if authorLogin == viewer {
		return true
	}
	for _, login := range assigneeLogins {
		if login == viewer {
			return true
		}
	}
	return false
}

// matchesViewerOnItem reports whether the project item's content carries
// the given viewer as author or assignee. Draft items have no author /
// assignees so they're always excluded under `--mine`.
func matchesViewerOnItem(item *queries.ProjectV2ItemNode, viewer string) bool {
	c := projectitem.ContentOf(item)
	if c.Typename == "" || c.Typename == "DraftIssue" {
		return false
	}
	if c.Author != "" && c.Author == viewer {
		return true
	}
	for _, login := range c.Assignees {
		if login == viewer {
			return true
		}
	}
	return false
}

func isItemDone(item *queries.ProjectV2ItemNode) bool {
	status := projectitem.FindStatus(projectitem.FieldValuesOf(item))
	if status == "" {
		return false
	}
	return strings.EqualFold(status, "done")
}
