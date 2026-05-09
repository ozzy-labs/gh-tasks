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
	c := &cobra.Command{
		Use:   "link <pr> <task>",
		Short: "Link a PR with an Issue / Project item",
		RunE: func(c *cobra.Command, args []string) error {
			return runLink(c.Context(), c, deps, args)
		},
	}
	addJSONFlags(c)
	addJSONCompletion(c, linkJSONFields)
	return c
}

func runLink(ctx context.Context, c *cobra.Command, deps Deps, args []string) error {
	r, err := deps.Resolve(c)
	if err != nil {
		return localizedError(c, r, err)
	}
	jsonReq, jsonOn, err := resolveJSONRequest(c, r, linkJSONFields)
	if err != nil {
		return err
	}
	pr, task, ok := parseLinkArgs(args)
	if !ok {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.link.argsRequired"))
		return ErrSilentArgs
	}
	sc, err := scope.Detect(scope.DetectOptions{
		Flag:         flagString(c, "scope"),
		HasGitRemote: deps.HasGitRemote,
		DefaultScope: r.Config.DefaultScope,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	if sc == scope.Repo {
		return runLinkRepo(ctx, c, deps, r, pr, task, jsonOn, jsonReq)
	}
	return runLinkProject(ctx, c, deps, r, sc, pr, task, jsonOn, jsonReq)
}

func runLinkRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, pr, task int, jsonOn bool, jsonReq jsonRequest) error {
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	gqlClient := clients.AsGenqlientClient()
	prResp, err := queries.GetPullRequestByNumber(ctx, gqlClient, id.Owner, id.Name, pr)
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "get pull request", err)
	}
	if prResp.Repository == nil || prResp.Repository.PullRequest == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.pullRequest.notFound", "owner", id.Owner, "name", id.Name, "number", pr))
		return ErrSilentRuntime
	}
	prNode := prResp.Repository.PullRequest
	if ContainsCloseLink(prNode.Body, task) {
		if jsonOn {
			return renderJSONItems(c, r, []map[string]any{{
				"id": prNode.Id, "number": prNode.Number, "state": "OPEN",
				"title": prNode.Title, "type": "PULL_REQUEST", "updatedAt": prNode.UpdatedAt, "url": prNode.Url,
				"linkType": "closesAdded",
				"linkedTo": nil,
			}}, jsonReq, linkJSONFields)
		}
		fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("link.alreadyLinked"), prNode.Url)
		return nil
	}
	updatedBody := AppendCloseLink(prNode.Body, task)
	updateInput := queries.NewUpdatePullRequestInput(prNode.Id)
	updateInput.Body = &updatedBody
	updated, err := queries.UpdatePullRequest(ctx, gqlClient, updateInput)
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "update pull request body", err)
	}
	if jsonOn {
		return renderJSONItems(c, r, []map[string]any{{
			"id": updated.UpdatePullRequest.PullRequest.Id, "number": prNode.Number,
			"state": "OPEN", "title": updated.UpdatePullRequest.PullRequest.Title, "type": "PULL_REQUEST",
			"updatedAt": updated.UpdatePullRequest.PullRequest.UpdatedAt, "url": updated.UpdatePullRequest.PullRequest.Url,
			"linkType": "closesAdded",
			"linkedTo": map[string]any{"number": task, "type": "ISSUE"},
		}}, jsonReq, linkJSONFields)
	}
	fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("link.added"), updated.UpdatePullRequest.PullRequest.Url)
	return nil
}

func runLinkProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, pr, task int, jsonOn bool, jsonReq jsonRequest) error {
	pref, err := project.Resolve(project.ResolveOptions{
		Scope:       sc,
		Flag:        flagString(c, "project"),
		OrgProject:  r.Config.OrgProject,
		UserProject: r.Config.UserProject,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
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
	gqlClient := clients.AsGenqlientClient()
	prResp, err := queries.GetPullRequestByNumber(ctx, gqlClient, id.Owner, id.Name, pr)
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "get pull request", err)
	}
	if prResp.Repository == nil || prResp.Repository.PullRequest == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.pullRequest.notFound", "owner", id.Owner, "name", id.Name, "number", pr))
		return ErrSilentRuntime
	}
	issueResp, err := queries.GetIssueByNumber(ctx, gqlClient, id.Owner, id.Name, task)
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "get issue", err)
	}
	if issueResp.Repository == nil || issueResp.Repository.Issue == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.issue.notFound", "owner", id.Owner, "name", id.Name, "number", task))
		return ErrSilentRuntime
	}
	if _, err := queries.AddProjectV2ItemById(ctx, gqlClient, &queries.AddProjectV2ItemByIdInput{
		ProjectId: pid,
		ContentId: prResp.Repository.PullRequest.Id,
	}); err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "add PR to project", err)
	}
	if _, err := queries.AddProjectV2ItemById(ctx, gqlClient, &queries.AddProjectV2ItemByIdInput{
		ProjectId: pid,
		ContentId: issueResp.Repository.Issue.Id,
	}); err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "add issue to project", err)
	}
	if jsonOn {
		return renderJSONItems(c, r, []map[string]any{{
			"id": prResp.Repository.PullRequest.Id, "number": prResp.Repository.PullRequest.Number,
			"state": "OPEN", "title": prResp.Repository.PullRequest.Title, "type": "PULL_REQUEST",
			"updatedAt": prResp.Repository.PullRequest.UpdatedAt, "url": prResp.Repository.PullRequest.Url,
			"linkType": "projectBind",
			"linkedTo": map[string]any{
				"id":     issueResp.Repository.Issue.Id,
				"number": issueResp.Repository.Issue.Number,
				"title":  issueResp.Repository.Issue.Title,
				"type":   "ISSUE",
				"url":    issueResp.Repository.Issue.Url,
			},
		}}, jsonReq, linkJSONFields)
	}
	fmt.Fprintf(c.OutOrStdout(), "%s: %s ↔ %s\n",
		r.T("link.added.project"),
		prResp.Repository.PullRequest.Url,
		issueResp.Repository.Issue.Url)
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
