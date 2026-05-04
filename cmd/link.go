package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

func newLinkCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "link <pr> <task>",
		Short: "Link a PR with an Issue / Project item",
		RunE: func(c *cobra.Command, args []string) error {
			return runLink(c.Context(), c, deps, args)
		},
	}
}

func runLink(ctx context.Context, c *cobra.Command, deps Deps, args []string) error {
	r, err := deps.Resolve()
	if err != nil {
		return localizedError(c, r, err)
	}
	pr, task, ok := parseLinkArgs(args)
	if !ok {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.link.argsRequired"))
		return ErrSilent
	}
	sc, err := scope.Detect(scope.DetectOptions{
		Argv:         deps.Argv,
		HasGitRemote: deps.HasGitRemote,
		DefaultScope: r.Config.DefaultScope,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	if sc == scope.Repo {
		return runLinkRepo(ctx, c, deps, r, pr, task)
	}
	return runLinkProject(ctx, c, deps, r, sc, pr, task)
}

func runLinkRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, pr, task int) error {
	id, err := repo.Resolve(repo.ResolveOptions{Argv: deps.Argv, GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	var prResp queries.GetPullRequestByNumberResponse
	if err := clients.GraphQL.Do(ctx, queries.GetPullRequestByNumber, map[string]any{
		"owner": id.Owner, "name": id.Name, "number": pr,
	}, &prResp); err != nil {
		return fmt.Errorf("get pull request: %w", err)
	}
	if prResp.Repository == nil || prResp.Repository.PullRequest == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.pullRequest.notFound", "owner", id.Owner, "name", id.Name, "number", pr))
		return ErrSilent
	}
	prNode := prResp.Repository.PullRequest
	if ContainsCloseLink(prNode.Body, task) {
		fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("link.alreadyLinked"), prNode.URL)
		return nil
	}
	updatedBody := AppendCloseLink(prNode.Body, task)
	var updated queries.UpdatePullRequestResponse
	if err := clients.GraphQL.Do(ctx, queries.UpdatePullRequest, map[string]any{
		"input": map[string]any{"pullRequestId": prNode.ID, "body": updatedBody},
	}, &updated); err != nil {
		return fmt.Errorf("update pull request body: %w", err)
	}
	fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("link.added"), updated.UpdatePullRequest.PullRequest.URL)
	return nil
}

func runLinkProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, pr, task int) error {
	pref, err := project.Resolve(project.ResolveOptions{
		Scope:       sc,
		Argv:        deps.Argv,
		OrgProject:  r.Config.OrgProject,
		UserProject: r.Config.UserProject,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	id, err := repo.Resolve(repo.ResolveOptions{Argv: deps.Argv, GetRemoteURL: deps.GetRemoteURL})
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
	var prResp queries.GetPullRequestByNumberResponse
	if err := clients.GraphQL.Do(ctx, queries.GetPullRequestByNumber, map[string]any{
		"owner": id.Owner, "name": id.Name, "number": pr,
	}, &prResp); err != nil {
		return fmt.Errorf("get pull request: %w", err)
	}
	if prResp.Repository == nil || prResp.Repository.PullRequest == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.pullRequest.notFound", "owner", id.Owner, "name", id.Name, "number", pr))
		return ErrSilent
	}
	var issueResp queries.GetIssueByNumberResponse
	if err := clients.GraphQL.Do(ctx, queries.GetIssueByNumber, map[string]any{
		"owner": id.Owner, "name": id.Name, "number": task,
	}, &issueResp); err != nil {
		return fmt.Errorf("get issue: %w", err)
	}
	if issueResp.Repository == nil || issueResp.Repository.Issue == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.issue.notFound", "owner", id.Owner, "name", id.Name, "number", task))
		return ErrSilent
	}
	var addResp queries.AddProjectV2ItemByIDResponse
	if err := clients.GraphQL.Do(ctx, queries.AddProjectV2ItemByID, map[string]any{
		"input": map[string]any{"projectId": pid, "contentId": prResp.Repository.PullRequest.ID},
	}, &addResp); err != nil {
		return fmt.Errorf("add PR to project: %w", err)
	}
	if err := clients.GraphQL.Do(ctx, queries.AddProjectV2ItemByID, map[string]any{
		"input": map[string]any{"projectId": pid, "contentId": issueResp.Repository.Issue.ID},
	}, &addResp); err != nil {
		return fmt.Errorf("add issue to project: %w", err)
	}
	fmt.Fprintf(c.OutOrStdout(), "%s: %s ↔ %s\n",
		r.T("link.added.project"),
		prResp.Repository.PullRequest.URL,
		issueResp.Repository.Issue.URL)
	return nil
}

var closeLinkRegexpFmt = regexp.MustCompile(`(?i)\b(Closes|Fixes|Resolves)\s+#(\d+)\b`)

// ContainsCloseLink reports whether body already contains a Closes/Fixes/Resolves
// reference to the given task number.
func ContainsCloseLink(body string, taskNumber int) bool {
	for _, m := range closeLinkRegexpFmt.FindAllStringSubmatch(body, -1) {
		if m[2] == strconv.Itoa(taskNumber) {
			return true
		}
	}
	return false
}

// AppendCloseLink appends a `Closes #N` line to body, separated by a blank
// line when body is non-empty.
func AppendCloseLink(body string, taskNumber int) string {
	trimmed := strings.TrimRightFunc(body, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	sep := ""
	if trimmed != "" {
		sep = "\n\n"
	}
	return fmt.Sprintf("%s%sCloses #%d\n", trimmed, sep, taskNumber)
}

func parseLinkArgs(args []string) (int, int, bool) {
	if len(args) < 2 {
		return 0, 0, false
	}
	pr, ok := parseIssueNumber(args[0])
	if !ok {
		return 0, 0, false
	}
	task, ok := parseIssueNumber(args[1])
	if !ok {
		return 0, 0, false
	}
	return pr, task, true
}
