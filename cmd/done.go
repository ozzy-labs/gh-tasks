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
	r, err := deps.Resolve(c)
	if err != nil {
		return localizedError(c, r, err)
	}
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.done.idRequired"))
		return ErrSilentArgs
	}
	rawID := args[0]
	sc, err := scope.Detect(scope.DetectOptions{
		Flag:         flagString(c, "scope"),
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
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	gqlClient := clients.AsGenqlientClient()
	resp, err := queries.GetIssueByNumber(ctx, gqlClient, id.Owner, id.Name, num)
	if err != nil {
		return fmt.Errorf("get issue: %w", err)
	}
	if resp.Repository == nil || resp.Repository.Issue == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.issue.notFound", "owner", id.Owner, "name", id.Name, "number", num))
		return ErrSilentRuntime
	}
	if resp.Repository.Issue.State == queries.IssueStateClosed {
		fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("done.alreadyClosed"), resp.Repository.Issue.Url)
		return nil
	}
	closed, err := queries.CloseIssue(ctx, gqlClient, &queries.CloseIssueInput{IssueId: resp.Repository.Issue.Id})
	if err != nil {
		return fmt.Errorf("close issue: %w", err)
	}
	fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("done.closed"), closed.CloseIssue.Issue.Url)
	return nil
}

func runDoneProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, itemID string) error {
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
	gqlClient := clients.AsGenqlientClient()
	fieldsResp, err := queries.ListProjectV2Fields(ctx, gqlClient, pid, doneFieldsLimit)
	if err != nil {
		return fmt.Errorf("list project fields: %w", err)
	}
	if !projectitem.HasFieldsNode(fieldsResp) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	fields := projectitem.FieldsOf(projectitem.FieldsFromResponse(fieldsResp))
	statusField := findStatusField(fields)
	if statusField == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.done.statusFieldMissing"))
		return ErrSilentRuntime
	}
	doneOption := findOption(statusField.Options, "done")
	if doneOption == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.done.doneOptionMissing"))
		return ErrSilentRuntime
	}

	itemsResp, err := queries.ListProjectV2Items(ctx, gqlClient, pid, doneItemsLimit)
	if err != nil {
		return fmt.Errorf("list project items: %w", err)
	}
	itemList := projectitem.ItemsFromResponse(itemsResp)
	var target *queries.ProjectV2ItemNode
	for _, n := range itemList {
		if n != nil && n.Id == itemID {
			target = n
			break
		}
	}
	if target == nil {
		// Distinguish "not found in the page" from "not found in the
		// project at all": when the response was at the page limit the
		// caller should know pagination might have hidden the item, since
		// operations.graphql currently has no pageInfo wiring.
		if len(itemList) >= doneItemsLimit {
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.done.searchLimit", "id", itemID, "limit", doneItemsLimit))
		} else {
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.projectItem.notFound", "id", itemID))
		}
		return ErrSilentRuntime
	}
	if isAlreadyDone(target, statusField.ID, doneOption.ID) {
		fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("done.alreadyDone.project"), itemID)
		return nil
	}
	if _, err := queries.UpdateProjectV2ItemFieldValue(ctx, gqlClient, &queries.UpdateProjectV2ItemFieldValueInput{
		ProjectId: pid,
		ItemId:    itemID,
		FieldId:   statusField.ID,
		Value:     &queries.ProjectV2FieldValue{SingleSelectOptionId: &doneOption.ID},
	}); err != nil {
		return fmt.Errorf("update item field value: %w", err)
	}
	fmt.Fprintf(c.OutOrStdout(), "%s: %s\n", r.T("done.statusUpdated.project"), itemID)
	return nil
}

func findStatusField(fields []projectitem.FieldDescriptor) *projectitem.FieldDescriptor {
	for i := range fields {
		f := &fields[i]
		if f.DataType == "SINGLE_SELECT" && strings.EqualFold(f.Name, "status") {
			return f
		}
	}
	return nil
}

func findOption(opts []projectitem.FieldOption, name string) *projectitem.FieldOption {
	for i := range opts {
		if strings.EqualFold(opts[i].Name, name) {
			return &opts[i]
		}
	}
	return nil
}

func isAlreadyDone(item *queries.ProjectV2ItemNode, fieldID, optID string) bool {
	for _, v := range projectitem.FieldValuesOf(item) {
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
