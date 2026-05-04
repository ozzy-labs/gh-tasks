package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/period"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/repo"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

const (
	planFetchLimit       = 100
	planFieldsFetchLimit = 50
)

func newPlanCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "plan",
		Short: "Plan tasks for a period (daily / weekly / sprint)",
		RunE: func(c *cobra.Command, _ []string) error {
			return runPlan(c.Context(), c, deps)
		},
	}
	c.Flags().String("period", "weekly", "aggregation window: daily | weekly | sprint")
	c.Flags().Bool("dry-run", false, "preview without writing milestone / iteration changes")
	return c
}

func runPlan(ctx context.Context, c *cobra.Command, deps Deps) error {
	r, err := deps.Resolve()
	if err != nil {
		return localizedError(c, r, err)
	}
	pflag, _ := c.Flags().GetString("period")
	dryRun, _ := c.Flags().GetBool("dry-run")
	p, _, err := period.ParseFlag([]string{"--period=" + pflag})
	if err != nil {
		return localizedError(c, r, err)
	}
	if p == "" {
		p = period.Weekly
	}
	now := time.Now()
	if deps.Now != nil {
		now = deps.Now()
	}
	rng := period.Of(p, now, "", deps.Env)
	sc, err := scope.Detect(scope.DetectOptions{
		Argv:         deps.Argv,
		HasGitRemote: deps.HasGitRemote,
		DefaultScope: r.Config.DefaultScope,
	})
	if err != nil {
		return localizedError(c, r, err)
	}
	if sc == scope.Repo {
		return runPlanRepo(ctx, c, deps, r, p, rng, dryRun, now)
	}
	return runPlanProject(ctx, c, deps, r, sc, p, rng, dryRun, now)
}

func runPlanRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, p period.Period, rng period.Range, dryRun bool, now time.Time) error {
	id, err := repo.Resolve(repo.ResolveOptions{Argv: deps.Argv, GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	var issuesResp queries.ListRepoIssuesWithMilestoneResponse
	if err := clients.GraphQL.Do(ctx, queries.ListRepoIssuesWithMilestone, map[string]any{
		"owner": id.Owner, "name": id.Name, "first": planFetchLimit,
	}, &issuesResp); err != nil {
		return err
	}
	if issuesResp.Repository == nil {
		fmt.Fprintf(c.ErrOrStderr(), "repository not found: %s/%s\n", id.Owner, id.Name)
		return ErrSilent
	}
	inRange := []queries.RepoIssueWithMilestoneNode{}
	for _, n := range issuesResp.Repository.Issues.Nodes {
		if t, err := time.Parse(time.RFC3339, n.UpdatedAt); err == nil &&
			(t.Equal(rng.Start) || t.After(rng.Start)) && t.Before(rng.End) {
			inRange = append(inRange, n)
		}
	}
	title := period.SuggestMilestoneTitle(p, now, "", deps.Env)
	out := c.OutOrStdout()
	fmt.Fprintf(out, "%s: %s\n", r.T("plan.proposed"), title)
	fmt.Fprintf(out, "  %s → %s\n\n", rng.Start.Format("2006-01-02"), rng.End.Format("2006-01-02"))
	if len(inRange) == 0 {
		fmt.Fprintln(out, r.T("plan.empty"))
		if dryRun {
			fmt.Fprintf(out, "\n%s\n", r.T("plan.dryRunNote"))
		}
		return nil
	}
	fmt.Fprintf(out, "%s (%d)\n", r.T("plan.candidates"), len(inRange))
	for _, n := range inRange {
		fmt.Fprintf(out, "  #%d  %s\n", n.Number, n.Title)
	}
	fmt.Fprintln(out)
	if dryRun {
		fmt.Fprintln(out, r.T("plan.dryRunNote"))
		return nil
	}

	var milestonesResp queries.ListMilestonesResponse
	if err := clients.GraphQL.Do(ctx, queries.ListMilestones, map[string]any{
		"owner": id.Owner, "name": id.Name, "first": planFetchLimit,
	}, &milestonesResp); err != nil {
		return err
	}
	var milestoneID string
	var milestoneNumber int
	if milestonesResp.Repository != nil {
		for _, m := range milestonesResp.Repository.Milestones.Nodes {
			if m.Title == title {
				milestoneID = m.ID
				milestoneNumber = m.Number
				fmt.Fprintf(out, "%s: %s (#%d)\n", r.T("plan.reused"), title, m.Number)
				break
			}
		}
	}
	if milestoneID == "" {
		var created queries.CreateMilestoneResult
		path := fmt.Sprintf("/repos/%s/%s/milestones", id.Owner, id.Name)
		body := map[string]any{"title": title}
		if err := clients.REST.Do(ctx, "POST", path, body, &created); err != nil {
			return fmt.Errorf("create milestone: %w", err)
		}
		milestoneID = created.NodeID
		milestoneNumber = created.Number
		fmt.Fprintf(out, "%s: %s (#%d)\n", r.T("plan.created"), title, created.Number)
	}

	for _, n := range inRange {
		if n.Milestone != nil && n.Milestone.ID != milestoneID {
			fmt.Fprintf(out, "  %s: #%d → %s\n", r.T("plan.skippedExisting"), n.Number, n.Milestone.Title)
			continue
		}
		if n.Milestone != nil && n.Milestone.ID == milestoneID {
			continue
		}
		var update queries.UpdateIssueMilestoneResponse
		if err := clients.GraphQL.Do(ctx, queries.UpdateIssueMilestone, map[string]any{
			"input": map[string]any{"id": n.ID, "milestoneId": milestoneID},
		}, &update); err != nil {
			return err
		}
		fmt.Fprintf(out, "  %s: #%d\n", r.T("plan.linked"), n.Number)
	}
	fmt.Fprintf(out, "\nhttps://github.com/%s/%s/milestone/%d\n", id.Owner, id.Name, milestoneNumber)
	return nil
}

func runPlanProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, p period.Period, rng period.Range, dryRun bool, now time.Time) error {
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
		fmt.Fprintf(c.ErrOrStderr(), "project not found: %s/%d (--scope %s)\n", pref.Owner, pref.Number, sc)
		return ErrSilent
	}
	var fieldsResp queries.ListProjectV2FieldsResponse
	if err := clients.GraphQL.Do(ctx, queries.ListProjectV2Fields, map[string]any{
		"projectId": pid, "first": planFieldsFetchLimit,
	}, &fieldsResp); err != nil {
		return err
	}
	if fieldsResp.Node == nil {
		fmt.Fprintf(c.ErrOrStderr(), "project not found: %s/%d (--scope %s)\n", pref.Owner, pref.Number, sc)
		return ErrSilent
	}
	itField := findIterationField(fieldsResp.Node.Fields.Nodes)
	if itField == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.plan.iterationFieldMissing"))
		return ErrSilent
	}
	target := period.SuggestMilestoneTitle(p, now, "", deps.Env)
	resolved := resolveTargetIteration(itField.Configuration.Iterations, target, now)
	if resolved == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.plan.noIterationsAvailable"))
		return ErrSilent
	}
	out := c.OutOrStdout()
	fmt.Fprintf(out, "%s: %s\n", r.T("plan.proposed.project"), resolved.iteration.Title)
	fmt.Fprintf(out, "  %s → %s\n", rng.Start.Format("2006-01-02"), rng.End.Format("2006-01-02"))
	if resolved.matched {
		fmt.Fprintf(out, "  %s: %s\n\n", r.T("plan.iterationMatched.project"), target)
	} else {
		fmt.Fprintf(c.ErrOrStderr(), "%s: %s → %s\n", r.T("plan.iterationFallback.project"), target, resolved.iteration.Title)
		fmt.Fprintf(out, "  %s\n\n", r.T("plan.iterationFallback.project"))
	}

	var itemsResp queries.ListProjectV2ItemsResponse
	if err := clients.GraphQL.Do(ctx, queries.ListProjectV2Items, map[string]any{
		"projectId": pid, "first": planFetchLimit,
	}, &itemsResp); err != nil {
		return err
	}
	allItems := []queries.ProjectV2ItemNode{}
	if itemsResp.Node != nil {
		allItems = itemsResp.Node.Items.Nodes
	}
	inRange := []queries.ProjectV2ItemNode{}
	for _, item := range allItems {
		if t, err := time.Parse(time.RFC3339, item.UpdatedAt); err == nil &&
			(t.Equal(rng.Start) || t.After(rng.Start)) && t.Before(rng.End) {
			inRange = append(inRange, item)
		}
	}
	if len(inRange) == 0 {
		fmt.Fprintln(out, r.T("plan.empty.project"))
		if dryRun {
			fmt.Fprintf(out, "\n%s\n", r.T("plan.dryRunNote.project"))
		}
		return nil
	}
	fmt.Fprintf(out, "%s (%d)\n", r.T("plan.candidates.project"), len(inRange))
	for _, item := range inRange {
		fmt.Fprint(out, formatItemLineForPlan(item))
	}
	fmt.Fprintln(out)
	if dryRun {
		fmt.Fprintln(out, r.T("plan.dryRunNote.project"))
		return nil
	}
	for _, item := range inRange {
		if isAlreadyOnIteration(item, itField.ID, resolved.iteration.ID) {
			fmt.Fprintf(out, "  %s: %s\n", r.T("plan.iterationAlreadySet.project"), describeItem(item))
			continue
		}
		var update queries.UpdateProjectV2ItemFieldValueResponse
		if err := clients.GraphQL.Do(ctx, queries.UpdateProjectV2ItemFieldValue, map[string]any{
			"input": map[string]any{
				"projectId": pid,
				"itemId":    item.ID,
				"fieldId":   itField.ID,
				"value":     map[string]any{"iterationId": resolved.iteration.ID},
			},
		}, &update); err != nil {
			return err
		}
		fmt.Fprintf(out, "  %s: %s\n", r.T("plan.iterationUpdated.project"), describeItem(item))
	}
	return nil
}

func findIterationField(fields []queries.ProjectV2FieldNode) *queries.ProjectV2FieldNode {
	for i := range fields {
		f := &fields[i]
		if f.DataType == "ITERATION" && strings.EqualFold(f.Name, "iteration") {
			return f
		}
	}
	for i := range fields {
		f := &fields[i]
		if f.DataType == "ITERATION" {
			return f
		}
	}
	return nil
}

type resolvedIteration struct {
	iteration queries.ProjectV2IterationOption
	matched   bool
}

func resolveTargetIteration(iterations []queries.ProjectV2IterationOption, target string, now time.Time) *resolvedIteration {
	if len(iterations) == 0 {
		return nil
	}
	for _, it := range iterations {
		if it.Title == target {
			return &resolvedIteration{iteration: it, matched: true}
		}
	}
	for _, it := range iterations {
		if iterationContains(it, now) {
			return &resolvedIteration{iteration: it, matched: false}
		}
	}
	upcoming := []queries.ProjectV2IterationOption{}
	for _, it := range iterations {
		if start, err := parseIterationStart(it.StartDate); err == nil && (start.Equal(now) || start.After(now)) {
			upcoming = append(upcoming, it)
		}
	}
	sort.Slice(upcoming, func(i, j int) bool {
		ai, _ := parseIterationStart(upcoming[i].StartDate)
		aj, _ := parseIterationStart(upcoming[j].StartDate)
		return ai.Before(aj)
	})
	if len(upcoming) > 0 {
		return &resolvedIteration{iteration: upcoming[0], matched: false}
	}
	return &resolvedIteration{iteration: iterations[len(iterations)-1], matched: false}
}

func iterationContains(it queries.ProjectV2IterationOption, now time.Time) bool {
	start, err := parseIterationStart(it.StartDate)
	if err != nil {
		return false
	}
	end := start.AddDate(0, 0, it.Duration)
	return (now.Equal(start) || now.After(start)) && now.Before(end)
}

func parseIterationStart(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func isAlreadyOnIteration(item queries.ProjectV2ItemNode, fieldID, iterationID string) bool {
	for _, v := range item.FieldValues.Nodes {
		if v.Typename == "ProjectV2ItemFieldIterationValue" && v.Field.ID == fieldID && v.IterationID == iterationID {
			return true
		}
	}
	return false
}

func formatItemLineForPlan(item queries.ProjectV2ItemNode) string {
	c := item.Content
	if c == nil {
		return "  (no content)\n"
	}
	switch c.Typename {
	case "Issue":
		return fmt.Sprintf("  #%d  %s\n", c.Number, c.Title)
	case "PullRequest":
		return fmt.Sprintf("  PR#%d  %s\n", c.Number, c.Title)
	default:
		return fmt.Sprintf("  (draft)  %s\n", c.Title)
	}
}

func describeItem(item queries.ProjectV2ItemNode) string {
	c := item.Content
	if c == nil {
		return item.ID
	}
	switch c.Typename {
	case "Issue":
		return fmt.Sprintf("#%d", c.Number)
	case "PullRequest":
		return fmt.Sprintf("PR#%d", c.Number)
	default:
		return item.ID
	}
}
