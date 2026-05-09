package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

func newAddCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a task (Issue or Project draft item)",
		RunE: func(c *cobra.Command, args []string) error {
			return runAdd(c.Context(), c, deps, args)
		},
	}
	c.Flags().String("body", "", "Issue / draft item body")
	addJSONFlags(c)
	addJSONCompletion(c, itemJSONFields)
	return c
}

func runAdd(ctx context.Context, c *cobra.Command, deps Deps, args []string) error {
	r, err := deps.Resolve(c)
	if err != nil {
		return localizedError(c, r, err)
	}
	jsonReq, jsonOn, err := resolveJSONRequest(c, r, itemJSONFields)
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.add.titleRequired"))
		return ErrSilentArgs
	}
	title := args[0]
	body, _ := c.Flags().GetString("body")

	sc, err := scope.Detect(scope.DetectOptions{
		Flag:         flagString(c, "scope"),
		HasGitRemote: deps.HasGitRemote,
		DefaultScope: r.Config.DefaultScope,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	if sc == scope.Repo {
		return runAddRepo(ctx, c, deps, r, title, body, jsonOn, jsonReq)
	}
	return runAddProject(ctx, c, deps, r, sc, title, body, jsonOn, jsonReq)
}

func runAddRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, title, body string, jsonOn bool, jsonReq jsonRequest) error {
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	gqlClient := clients.AsGenqlientClient()
	idResp, err := queries.GetRepositoryID(ctx, gqlClient, id.Owner, id.Name)
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "get repository id", err)
	}
	if idResp.Repository == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	input := queries.NewCreateIssueInput(idResp.Repository.Id, title)
	if body != "" {
		input.Body = &body
	}
	resp, err := queries.CreateIssue(ctx, gqlClient, input)
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "create issue", err)
	}
	if jsonOn {
		// updatedAt is not in the CreateIssue mutation response; emit null
		// per the contract (selected fields always appear). Consumers who
		// need the timestamp can re-fetch via gh tasks list --json.
		return renderJSONItems(c, r, []map[string]any{{
			"id": resp.CreateIssue.Issue.Id, "number": resp.CreateIssue.Issue.Number,
			"state": "OPEN", "title": title, "type": "ISSUE",
			"updatedAt": nil, "url": resp.CreateIssue.Issue.Url,
		}}, jsonReq, itemJSONFields)
	}
	fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("add.created.repo"), resp.CreateIssue.Issue.Url)
	return nil
}

func runAddProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, title, body string, jsonOn bool, jsonReq jsonRequest) error {
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
	input := queries.NewAddProjectV2DraftIssueInput(pid, title)
	if body != "" {
		input.Body = &body
	}
	resp, err := queries.AddProjectV2DraftIssue(ctx, clients.AsGenqlientClient(), input)
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "add project draft issue", err)
	}
	if jsonOn {
		// Draft items have no GitHub-side number / url / updatedAt at
		// creation time. Number is 0 (zero-valued int), url is empty,
		// updatedAt is null per the contract.
		return renderJSONItems(c, r, []map[string]any{{
			"id": resp.AddProjectV2DraftIssue.ProjectItem.Id, "number": 0,
			"state": "", "title": title, "type": "DRAFT_ISSUE",
			"updatedAt": nil, "url": "",
		}}, jsonReq, itemJSONFields)
	}
	fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("add.created.project"), resp.AddProjectV2DraftIssue.ProjectItem.Id)
	return nil
}
