package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

const (
	doneFieldsLimit = 50
	doneItemsLimit  = 100
)

func newDoneCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "done <id>",
		Short: "Mark a task as done",
		RunE: func(c *cobra.Command, args []string) error {
			return runDone(c.Context(), c, deps, args)
		},
	}
}

func runDone(ctx context.Context, c *cobra.Command, deps Deps, args []string) error {
	r, err := deps.Resolve()
	if err != nil {
		return localizedError(c, r, err)
	}
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.done.idRequired"))
		return ErrSilentArgs
	}
	rawID := args[0]
	sc, err := scope.Detect(scope.DetectOptions{
		Argv:         deps.Argv,
		HasGitRemote: deps.HasGitRemote,
		DefaultScope: r.Config.DefaultScope,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	if sc == scope.Repo {
		return runDoneRepo(ctx, c, deps, r, rawID)
	}
	return runDoneProject(ctx, c, deps, r, sc, rawID)
}

func runDoneRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, rawID string) error {
	num, ok := parseIssueNumber(rawID)
	if !ok {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.done.idRequired"))
		return ErrSilentArgs
	}
	id, err := repo.Resolve(repo.ResolveOptions{Argv: deps.Argv, GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	var resp queries.GetIssueByNumberResponse
	if err := clients.GraphQL.Do(ctx, queries.GetIssueByNumber, map[string]any{
		"owner": id.Owner, "name": id.Name, "number": num,
	}, &resp); err != nil {
		return fmt.Errorf("get issue: %w", err)
	}
	if resp.Repository == nil || resp.Repository.Issue == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.issue.notFound", "owner", id.Owner, "name", id.Name, "number", num))
		return ErrSilentRuntime
	}
	if resp.Repository.Issue.State == "CLOSED" {
		fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("done.alreadyClosed"), resp.Repository.Issue.URL)
		return nil
	}
	var closed queries.CloseIssueResponse
	if err := clients.GraphQL.Do(ctx, queries.CloseIssue, map[string]any{
		"input": map[string]any{"issueId": resp.Repository.Issue.ID},
	}, &closed); err != nil {
		return fmt.Errorf("close issue: %w", err)
	}
	fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("done.closed"), closed.CloseIssue.Issue.URL)
	return nil
}

func runDoneProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, itemID string) error {
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
	var fieldsResp queries.ListProjectV2FieldsResponse
	if err := clients.GraphQL.Do(ctx, queries.ListProjectV2Fields, map[string]any{
		"projectId": pid, "first": doneFieldsLimit,
	}, &fieldsResp); err != nil {
		return fmt.Errorf("list project fields: %w", err)
	}
	if fieldsResp.Node == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	statusField := findStatusField(fieldsResp.Node.Fields.Nodes)
	if statusField == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.done.statusFieldMissing"))
		return ErrSilentRuntime
	}
	doneOption := findOption(statusField.Options, "done")
	if doneOption == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.done.doneOptionMissing"))
		return ErrSilentRuntime
	}

	var itemsResp queries.ListProjectV2ItemsResponse
	if err := clients.GraphQL.Do(ctx, queries.ListProjectV2Items, map[string]any{
		"projectId": pid, "first": doneItemsLimit,
	}, &itemsResp); err != nil {
		return fmt.Errorf("list project items: %w", err)
	}
	var target *queries.ProjectV2ItemNode
	if itemsResp.Node != nil {
		for i := range itemsResp.Node.Items.Nodes {
			if itemsResp.Node.Items.Nodes[i].ID == itemID {
				target = &itemsResp.Node.Items.Nodes[i]
				break
			}
		}
	}
	if target == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.projectItem.notFound", "id", itemID))
		return ErrSilentRuntime
	}
	if isAlreadyDone(*target, statusField.ID, doneOption.ID) {
		fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("done.alreadyDone.project"), itemID)
		return nil
	}
	var update queries.UpdateProjectV2ItemFieldValueResponse
	if err := clients.GraphQL.Do(ctx, queries.UpdateProjectV2ItemFieldValue, map[string]any{
		"input": map[string]any{
			"projectId": pid,
			"itemId":    itemID,
			"fieldId":   statusField.ID,
			"value":     map[string]any{"singleSelectOptionId": doneOption.ID},
		},
	}, &update); err != nil {
		return fmt.Errorf("update item field value: %w", err)
	}
	fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("done.statusUpdated.project"), itemID)
	return nil
}

func findStatusField(fields []queries.ProjectV2FieldNode) *queries.ProjectV2FieldNode {
	for i := range fields {
		f := &fields[i]
		if f.DataType == "SINGLE_SELECT" && strings.EqualFold(f.Name, "status") {
			return f
		}
	}
	return nil
}

func findOption(opts []queries.ProjectV2SelectOption, name string) *queries.ProjectV2SelectOption {
	for i := range opts {
		if strings.EqualFold(opts[i].Name, name) {
			return &opts[i]
		}
	}
	return nil
}

func isAlreadyDone(item queries.ProjectV2ItemNode, fieldID, optID string) bool {
	for _, v := range item.FieldValues.Nodes {
		if v.Typename == "ProjectV2ItemFieldSingleSelectValue" && v.Field.ID == fieldID && v.OptionID == optID {
			return true
		}
	}
	return false
}

func parseIssueNumber(raw string) (int, bool) {
	raw = strings.TrimPrefix(raw, "#")
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
