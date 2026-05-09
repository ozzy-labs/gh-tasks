package cmd

import (
	"context"
	"errors"
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
	c.Flags().Bool("write", false, "apply milestone / iteration changes (otherwise preview only)")
	addJSONFlags(c)
	addJSONCompletion(c, itemJSONFields)
	return c
}

func runPlan(ctx context.Context, c *cobra.Command, deps Deps) error {
	r, err := deps.Resolve(c)
	if err != nil {
		return localizedError(c, r, err)
	}
	jsonReq, jsonOn, err := resolveJSONRequest(c, r, itemJSONFields)
	if err != nil {
		return err
	}
	pflag, _ := c.Flags().GetString("period")
	write, _ := c.Flags().GetBool("write")
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
		return runPlanRepo(ctx, c, deps, r, p, rng, write, now, jsonOn, jsonReq)
	}
	return runPlanProject(ctx, c, deps, r, sc, p, rng, write, now, jsonOn, jsonReq)
}

func runPlanRepo(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, p period.Period, rng period.Range, write bool, now time.Time, jsonOn bool, jsonReq jsonRequest) error {
	id, err := repo.Resolve(repo.ResolveOptions{Flag: flagString(c, "repo"), GetRemoteURL: deps.GetRemoteURL})
	if err != nil {
		return localizedError(c, r, err)
	}
	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	gqlClient := clients.AsGenqlientClient()
	issues, err := queries.PaginateRepoIssuesWithMilestone(ctx, gqlClient, id.Owner, id.Name, planFetchLimit)
	if errors.Is(err, queries.ErrRepoNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list repo issues with milestone", err)
	}
	inRange := []*queries.RepoIssueWithMilestone{}
	for _, n := range issues {
		if n == nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, n.UpdatedAt); err == nil &&
			(t.Equal(rng.Start) || t.After(rng.Start)) && t.Before(rng.End) {
			inRange = append(inRange, n)
		}
	}
	if jsonOn && !write {
		// preview-only path: emit the candidate list as a flat JSON
		// array. write-mode JSON falls through to the bind loop below
		// and emits the bound items at the end.
		rows := make([]map[string]any, 0, len(inRange))
		for _, n := range inRange {
			rows = append(rows, map[string]any{
				"id": n.Id, "number": n.Number, "state": "OPEN",
				"title": n.Title, "type": "ISSUE", "updatedAt": n.UpdatedAt, "url": n.Url,
			})
		}
		return renderJSONItems(c, r, rows, jsonReq, itemJSONFields)
	}
	title := period.SuggestMilestoneTitle(p, period.Options{Getenv: deps.Env, Now: now})
	out := c.OutOrStdout()
	fmt.Fprintf(out, "%s: %s\n", r.T("plan.proposed"), title)
	fmt.Fprintf(out, "  %s → %s\n\n", rng.Start.Format("2006-01-02"), rng.End.Format("2006-01-02"))
	if len(inRange) == 0 {
		fmt.Fprintln(out, r.T("plan.empty"))
		if !write {
			fmt.Fprintf(out, "\n%s\n", r.T("plan.previewNote"))
		}
		return nil
	}
	fmt.Fprintf(out, "%s (%d)\n", r.T("plan.candidates"), len(inRange))
	for _, n := range inRange {
		fmt.Fprintf(out, "  #%d  %s\n", n.Number, n.Title)
	}
	fmt.Fprintln(out)
	if !write {
		fmt.Fprintln(out, r.T("plan.previewNote"))
		return nil
	}

	milestones, err := queries.PaginateMilestones(ctx, gqlClient, id.Owner, id.Name, planFetchLimit)
	if errors.Is(err, queries.ErrRepoNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.repo.notFound", "owner", id.Owner, "name", id.Name))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list milestones", err)
	}
	var milestoneID string
	var milestoneNumber int
	for _, m := range milestones {
		if m == nil {
			continue
		}
		if m.Title == title {
			milestoneID = m.Id
			milestoneNumber = m.Number
			fmt.Fprintf(out, "%s: %s (#%d)\n", r.T("plan.reused"), title, m.Number)
			break
		}
	}
	if milestoneID == "" {
		var created queries.CreateMilestoneResult
		path := fmt.Sprintf("repos/%s/%s/milestones", id.Owner, id.Name)
		body := map[string]any{"title": title}
		if err := clients.REST.Do(ctx, "POST", path, body, &created); err != nil {
			return wrapTransport(c.ErrOrStderr(), r.Locale, "create milestone", err)
		}
		milestoneID = created.NodeID
		milestoneNumber = created.Number
		fmt.Fprintf(out, "%s: %s (#%d)\n", r.T("plan.created"), title, created.Number)
	}

	bound := []*queries.RepoIssueWithMilestone{}
	for _, n := range inRange {
		if n.Milestone != nil && n.Milestone.Id != milestoneID {
			fmt.Fprintf(out, "  %s: #%d → %s\n", r.T("plan.skippedExisting"), n.Number, n.Milestone.Title)
			continue
		}
		if n.Milestone != nil && n.Milestone.Id == milestoneID {
			bound = append(bound, n)
			continue
		}
		updateInput := queries.NewUpdateIssueInput(n.Id)
		updateInput.MilestoneId = &milestoneID
		if _, err := queries.UpdateIssueMilestone(ctx, gqlClient, updateInput); err != nil {
			return wrapTransport(c.ErrOrStderr(), r.Locale, fmt.Sprintf("update issue milestone (issue #%d)", n.Number), err)
		}
		bound = append(bound, n)
		fmt.Fprintf(out, "  %s: #%d\n", r.T("plan.linked"), n.Number)
	}
	fmt.Fprintf(out, "\nhttps://github.com/%s/%s/milestone/%d\n", id.Owner, id.Name, milestoneNumber)
	if jsonOn {
		rows := make([]map[string]any, 0, len(bound))
		for _, n := range bound {
			rows = append(rows, map[string]any{
				"id": n.Id, "number": n.Number, "state": "OPEN",
				"title": n.Title, "type": "ISSUE", "updatedAt": n.UpdatedAt, "url": n.Url,
			})
		}
		return renderJSONItems(c, r, rows, jsonReq, itemJSONFields)
	}
	return nil
}

func runPlanProject(ctx context.Context, c *cobra.Command, deps Deps, r Resolved, sc scope.Scope, p period.Period, rng period.Range, write bool, now time.Time, jsonOn bool, jsonReq jsonRequest) error {
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
	fieldNodes, err := queries.PaginateProjectV2Fields(ctx, gqlClient, pid, planFieldsFetchLimit)
	if errors.Is(err, queries.ErrProjectNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list project fields", err)
	}
	fields := projectitem.FieldsOf(fieldNodes)
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

	allItems, err := queries.PaginateProjectV2Items(ctx, gqlClient, pid, planFetchLimit)
	if errors.Is(err, queries.ErrProjectNotFound) {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.project.notFound", "owner", pref.Owner, "number", pref.Number, "scope", sc))
		return ErrSilentRuntime
	}
	if err != nil {
		return wrapTransport(c.ErrOrStderr(), r.Locale, "list project items", err)
	}
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
	if jsonOn && !write {
		// preview-only path (see runPlanRepo for rationale).
		return renderJSONItems(c, r, projectItemRowsToJSON(inRange), jsonReq, itemJSONFields)
	}
	if len(inRange) == 0 {
		fmt.Fprintln(out, r.T("plan.empty.project"))
		if !write {
			fmt.Fprintf(out, "\n%s\n", r.T("plan.previewNote.project"))
		}
		return nil
	}
	fmt.Fprintf(out, "%s (%d)\n", r.T("plan.candidates.project"), len(inRange))
	for _, item := range inRange {
		fmt.Fprint(out, formatItemLineForPlan(item))
	}
	fmt.Fprintln(out)
	if !write {
		fmt.Fprintln(out, r.T("plan.previewNote.project"))
		return nil
	}
	bound := []*queries.ProjectV2ItemNode{}
	for _, item := range inRange {
		if isAlreadyOnIteration(item, itField.ID, resolved.iteration.ID) {
			fmt.Fprintf(out, "  %s: %s\n", r.T("plan.iterationAlreadySet.project"), describeItem(item))
			bound = append(bound, item)
			continue
		}
		if _, err := queries.UpdateProjectV2ItemFieldValue(ctx, gqlClient, &queries.UpdateProjectV2ItemFieldValueInput{
			ProjectId: pid,
			ItemId:    item.Id,
			FieldId:   itField.ID,
			Value:     &queries.ProjectV2FieldValue{IterationId: &resolved.iteration.ID},
		}); err != nil {
			return wrapTransport(c.ErrOrStderr(), r.Locale, fmt.Sprintf("update item field value (%s)", describeItem(item)), err)
		}
		fmt.Fprintf(out, "  %s: %s\n", r.T("plan.iterationUpdated.project"), describeItem(item))
		bound = append(bound, item)
	}
	if jsonOn {
		return renderJSONItems(c, r, projectItemRowsToJSON(bound), jsonReq, itemJSONFields)
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
