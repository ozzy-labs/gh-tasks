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
	r, err := deps.Resolve(c)
	if err != nil {
		return localizedError(c, r, err)
	}
	pflag, _ := c.Flags().GetString("period")
	dryRun, _ := c.Flags().GetBool("dry-run")
	p, err := period.Parse(pflag)
	if err != nil {
		return localizedError(c, r, err)
	}
	now := deps.Now()
	rng := period.Of(p, period.Options{Getenv: deps.Env, Now: now})
	sc, err := scope.Detect(scope.DetectOptions{
		Flag:         flagString(c, "scope"),
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
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	gqlClient := clients.AsGenqlientClient()
	issuesResp, err := queries.ListRepoIssuesWithMilestone(ctx, gqlClient, id.Owner, id.Name, planFetchLimit)
	if err != nil {
		return fmt.Errorf("list repo issues with milestone: %w", err)
	}
	if issuesResp.Repository == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	warnIfTruncated(c, r, kindRepoIssues, len(issuesResp.Repository.Issues.Nodes), planFetchLimit)
	type issueRow = queries.ListRepoIssuesWithMilestoneRepositoryIssuesIssueConnectionNodesIssue
	inRange := []*issueRow{}
	for _, n := range issuesResp.Repository.Issues.Nodes {
		if n == nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, n.UpdatedAt); err == nil &&
			(t.Equal(rng.Start) || t.After(rng.Start)) && t.Before(rng.End) {
			inRange = append(inRange, n)
		}
	}
	title := period.SuggestMilestoneTitle(p, period.Options{Getenv: deps.Env, Now: now})
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

	milestonesResp, err := queries.ListMilestones(ctx, gqlClient, id.Owner, id.Name, planFetchLimit)
	if err != nil {
		return fmt.Errorf("list milestones: %w", err)
	}
	if milestonesResp.Repository != nil {
		warnIfTruncated(c, r, kindMilestones, len(milestonesResp.Repository.Milestones.Nodes), planFetchLimit)
	}
	var milestoneID string
	var milestoneNumber int
	if milestonesResp.Repository != nil {
		for _, m := range milestonesResp.Repository.Milestones.Nodes {
			if m.Title == title {
				milestoneID = m.Id
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
		if n.Milestone != nil && n.Milestone.Id != milestoneID {
			fmt.Fprintf(out, "  %s: #%d → %s\n", r.T("plan.skippedExisting"), n.Number, n.Milestone.Title)
			continue
		}
		if n.Milestone != nil && n.Milestone.Id == milestoneID {
			continue
		}
		if _, err := queries.UpdateIssueMilestone(ctx, gqlClient, &queries.UpdateIssueInput{
			Id:          n.Id,
			MilestoneId: &milestoneID,
		}); err != nil {
			return fmt.Errorf("update issue milestone (issue #%d): %w", n.Number, err)
		}
		fmt.Fprintf(out, "  %s: #%d\n", r.T("plan.linked"), n.Number)
	}
	fmt.Fprintf(out, "\nhttps://github.com/%s/%s/milestone/%d\n", id.Owner, id.Name, milestoneNumber)
	return nil
}

func runPlanProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, p period.Period, rng period.Range, dryRun bool, now time.Time) error {
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
	fieldsResp, err := queries.ListProjectV2Fields(ctx, gqlClient, pid, planFieldsFetchLimit)
	if err != nil {
		return fmt.Errorf("list project fields: %w", err)
	}
	if !projectitem.HasFieldsNode(fieldsResp) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	fields := projectitem.FieldsOf(projectitem.FieldsFromResponse(fieldsResp))
	itField := findIterationField(fields)
	if itField == nil || itField.Configuration == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.plan.iterationFieldMissing"))
		return ErrSilentRuntime
	}
	target := period.SuggestMilestoneTitle(p, period.Options{Getenv: deps.Env, Now: now})
	resolved := resolveTargetIteration(itField.Configuration.Iterations, target, now)
	if resolved == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.plan.noIterationsAvailable"))
		return ErrSilentRuntime
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

	itemsResp, err := queries.ListProjectV2Items(ctx, gqlClient, pid, planFetchLimit)
	if err != nil {
		return fmt.Errorf("list project items: %w", err)
	}
	allItems := projectitem.ItemsFromResponse(itemsResp)
	warnIfTruncated(c, r, kindProjectItems, len(allItems), planFetchLimit)
	inRange := []*queries.ProjectV2ItemNode{}
	for _, item := range allItems {
		if item == nil {
			continue
		}
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
		if _, err := queries.UpdateProjectV2ItemFieldValue(ctx, gqlClient, &queries.UpdateProjectV2ItemFieldValueInput{
			ProjectId: pid,
			ItemId:    item.Id,
			FieldId:   itField.ID,
			Value:     &queries.ProjectV2FieldValue{IterationId: &resolved.iteration.ID},
		}); err != nil {
			return fmt.Errorf("update item field value (%s): %w", describeItem(item), err)
		}
		fmt.Fprintf(out, "  %s: %s\n", r.T("plan.iterationUpdated.project"), describeItem(item))
	}
	return nil
}

func findIterationField(fields []projectitem.FieldDescriptor) *projectitem.FieldDescriptor {
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
	iteration projectitem.IterationOption
	matched   bool
}

func resolveTargetIteration(iterations []projectitem.IterationOption, target string, now time.Time) *resolvedIteration {
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
	type parsedIteration struct {
		iteration projectitem.IterationOption
		start     time.Time
	}
	upcoming := []parsedIteration{}
	for _, it := range iterations {
		start, err := parseIterationStart(it.StartDate)
		if err != nil {
			continue
		}
		if start.Equal(now) || start.After(now) {
			upcoming = append(upcoming, parsedIteration{iteration: it, start: start})
		}
	}
	// Use SliceStable to preserve catalog order for iterations sharing a start
	// date; the parsed time is captured at filter time, so the comparator never
	// re-parses (and therefore never silently treats a parse error as epoch).
	sort.SliceStable(upcoming, func(i, j int) bool {
		return upcoming[i].start.Before(upcoming[j].start)
	})
	if len(upcoming) > 0 {
		return &resolvedIteration{iteration: upcoming[0].iteration, matched: false}
	}
	return &resolvedIteration{iteration: iterations[len(iterations)-1], matched: false}
}

func iterationContains(it projectitem.IterationOption, now time.Time) bool {
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

func isAlreadyOnIteration(item *queries.ProjectV2ItemNode, fieldID, iterationID string) bool {
	for _, v := range projectitem.FieldValuesOf(item) {
		if v.Typename == "ProjectV2ItemFieldIterationValue" && v.Field.ID == fieldID && v.IterationID == iterationID {
			return true
		}
	}
	return false
}

func formatItemLineForPlan(item *queries.ProjectV2ItemNode) string {
	c := projectitem.ContentOf(item)
	switch c.Typename {
	case "Issue":
		return fmt.Sprintf("  #%d  %s\n", c.Number, c.Title)
	case "PullRequest":
		return fmt.Sprintf("  PR#%d  %s\n", c.Number, c.Title)
	case "DraftIssue":
		return fmt.Sprintf("  (draft)  %s\n", c.Title)
	default:
		return "  (no content)\n"
	}
}

func describeItem(item *queries.ProjectV2ItemNode) string {
	c := projectitem.ContentOf(item)
	switch c.Typename {
	case "Issue":
		return fmt.Sprintf("#%d", c.Number)
	case "PullRequest":
		return fmt.Sprintf("PR#%d", c.Number)
	default:
		if item == nil {
			return ""
		}
		return item.Id
	}
}
