// Package projectitem provides helpers for resolving and rendering Projects
// v2 items. The format helpers preserve byte-identical output with the prior
// TS implementation so review/standup/list outputs do not drift.
package projectitem

import (
	"context"
	"fmt"
	"strings"

	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

// ProjectItemError carries a localized payload for failures inside this
// package (currently: GraphQL transport errors during project node ID
// resolution). It satisfies [i18n.Localized] via the embedded Payload, so
// the cmd layer's localizedError helper can render it in the active locale.
//
// Use errors.As(err, &target) to test for this type:
//
//	var pe *projectitem.ProjectItemError
//	if errors.As(err, &pe) { ... }
type ProjectItemError struct {
	i18n.Payload
	cause error
}

// Error renders the en-locale message so log/wrap paths still surface a
// human-readable string when bypassing localizedError.
func (e *ProjectItemError) Error() string { return e.Localize(i18n.LocaleEN) }

// Unwrap exposes the underlying cause (typically a github transport
// error) so callers can keep using errors.Is / errors.As for transport
// classification.
func (e *ProjectItemError) Unwrap() error { return e.cause }

// ResolveProjectNodeID resolves a [project.Ref] to its Projects v2 node id by
// issuing the appropriate GraphQL query for the scope. Returns ("", nil) when
// the project cannot be found (wrong owner, wrong number, or insufficient
// scopes on the token).
//
// Calling this with [scope.Repo] is a programmer error (Projects v2 are not
// used in repo scope) and returns a localizable [scope.ScopeError]. Callers
// should resolve the project ahead of this call via [project.Resolve].
//
// GraphQL transport errors are wrapped in a [ProjectItemError] so the cmd
// layer can surface a localized message instead of cobra's default
// "Error: get org project: ..." prefix.
func ResolveProjectNodeID(ctx context.Context, gql github.GraphQLClient, sc scope.Scope, ref project.Ref) (string, error) {
	if sc == scope.Repo {
		return "", &scope.ScopeError{Payload: i18n.NewPayload("error.scope.invalidForProjectResolution")}
	}
	gqlClient := github.AsGenqlientClientFor(gql)

	if sc == scope.Org {
		resp, err := queries.GetOrgProjectV2(ctx, gqlClient, ref.Owner, ref.Number)
		if err != nil {
			return "", &ProjectItemError{
				Payload: i18n.NewPayload(
					"error.projectItem.getOrgProjectFailed",
					"owner", ref.Owner, "number", ref.Number, "reason", err.Error(),
				),
				cause: err,
			}
		}
		if resp.Organization == nil || resp.Organization.ProjectV2 == nil {
			return "", nil
		}
		return resp.Organization.ProjectV2.Id, nil
	}

	resp, err := queries.GetUserProjectV2(ctx, gqlClient, ref.Owner, ref.Number)
	if err != nil {
		return "", &ProjectItemError{
			Payload: i18n.NewPayload(
				"error.projectItem.getUserProjectFailed",
				"owner", ref.Owner, "number", ref.Number, "reason", err.Error(),
			),
			cause: err,
		}
	}
	if resp.User == nil || resp.User.ProjectV2 == nil {
		return "", nil
	}
	return resp.User.ProjectV2.Id, nil
}

// ItemsFromResponse extracts the items slice from a [ListProjectV2Items]
// response, returning nil when the project node is missing or has the
// wrong concrete type (genqlient models `node(id:)` as a Node interface
// because the schema's resolver could in principle return any
// Node-implementing type — `... on ProjectV2` narrows what we read but
// the static return type stays an interface).
func ItemsFromResponse(resp *queries.ListProjectV2ItemsResponse) []*queries.ProjectV2ItemNode {
	if resp == nil || resp.Node == nil {
		return nil
	}
	pv2, ok := (*resp.Node).(*queries.ProjectV2ItemsNodeProjectV2)
	if !ok || pv2 == nil || pv2.Items == nil {
		return nil
	}
	return pv2.Items.Nodes
}

// FieldsFromResponse extracts the fields slice from a [ListProjectV2Fields]
// response. Same caveat as [ItemsFromResponse]: genqlient's response model
// goes through the Node interface, so callers must accept that the typed
// projection may be empty when the server returns a non-ProjectV2 Node.
func FieldsFromResponse(resp *queries.ListProjectV2FieldsResponse) []queries.ProjectV2FieldNode {
	if resp == nil || resp.Node == nil {
		return nil
	}
	pv2, ok := (*resp.Node).(*queries.ProjectV2FieldsNodeProjectV2)
	if !ok || pv2 == nil || pv2.Fields == nil {
		return nil
	}
	out := make([]queries.ProjectV2FieldNode, 0, len(pv2.Fields.Nodes))
	for _, n := range pv2.Fields.Nodes {
		if n == nil || *n == nil {
			continue
		}
		out = append(out, *n)
	}
	return out
}

// HasProjectNode reports whether the [ListProjectV2ItemsResponse]'s Node
// field resolved to a ProjectV2 (i.e. the project id was valid). Used by
// callers to distinguish "project not found" from "project has zero items".
func HasProjectNode(resp *queries.ListProjectV2ItemsResponse) bool {
	if resp == nil || resp.Node == nil {
		return false
	}
	_, ok := (*resp.Node).(*queries.ProjectV2ItemsNodeProjectV2)
	return ok
}

// HasFieldsNode reports whether the [ListProjectV2FieldsResponse]'s Node
// field resolved to a ProjectV2.
func HasFieldsNode(resp *queries.ListProjectV2FieldsResponse) bool {
	if resp == nil || resp.Node == nil {
		return false
	}
	_, ok := (*resp.Node).(*queries.ProjectV2FieldsNodeProjectV2)
	return ok
}

// ContentSummary is a flat view of the union content for a Projects v2 item.
// Issue / PullRequest carry Number, Title, URL, State, UpdatedAt, optional
// ClosedAt / MergedAt / Author / Assignees. DraftIssue carries only Title
// and Body. Typename ("Issue" | "PullRequest" | "DraftIssue" | "") drives
// branching by callers; an empty Typename means the item has no content
// attached (genqlient renders this as Content == nil on the parent item).
type ContentSummary struct {
	Typename  string
	ID        string
	Number    int
	Title     string
	URL       string
	State     string
	UpdatedAt string
	ClosedAt  string
	MergedAt  string
	Body      string
	Author    string
	Assignees []string
}

// ContentOf flattens an item's union Content into a [ContentSummary].
// Returns the zero value when item or item.Content is nil.
func ContentOf(item *queries.ProjectV2ItemNode) ContentSummary {
	if item == nil || item.Content == nil {
		return ContentSummary{}
	}
	switch c := (*item.Content).(type) {
	case *queries.ProjectV2ItemContentIssue:
		return ContentSummary{
			Typename:  "Issue",
			ID:        c.Id,
			Number:    c.Number,
			Title:     c.Title,
			URL:       c.Url,
			State:     string(c.IssueState),
			UpdatedAt: c.UpdatedAt,
			ClosedAt:  derefString(c.ClosedAt),
			Author:    actorLogin(c.Author),
			Assignees: assigneeLogins(c.Assignees),
		}
	case *queries.ProjectV2ItemContentPullRequest:
		return ContentSummary{
			Typename:  "PullRequest",
			ID:        c.Id,
			Number:    c.Number,
			Title:     c.Title,
			URL:       c.Url,
			State:     string(c.PrState),
			UpdatedAt: c.UpdatedAt,
			MergedAt:  derefString(c.MergedAt),
			Author:    actorLogin(c.Author),
			Assignees: assigneeLogins(c.Assignees),
		}
	case *queries.ProjectV2ItemContentDraftIssue:
		return ContentSummary{
			Typename: "DraftIssue",
			ID:       c.Id,
			Title:    c.Title,
			Body:     c.Body,
		}
	}
	return ContentSummary{}
}

func actorLogin(a *queries.ProjectV2ItemContentLogin) string {
	if a == nil || *a == nil {
		return ""
	}
	return (*a).GetLogin()
}

func assigneeLogins(a *queries.ProjectV2ItemContentAssignees) []string {
	if a == nil {
		return nil
	}
	out := make([]string, 0, len(a.Nodes))
	for _, n := range a.Nodes {
		if n == nil {
			continue
		}
		out = append(out, n.Login)
	}
	return out
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// FieldValueRef is a flat view of one field-value entry on a Projects v2
// item. Typename selects which fields are populated; Field carries the id
// and name of the field that owns this value.
type FieldValueRef struct {
	Typename    string
	Name        string
	OptionID    string
	IterationID string
	Title       string
	StartDate   string
	Duration    int
	Text        string
	Date        string
	Field       FieldRef
}

// FieldRef is the {id,name} pair returned for the field that owns a value.
type FieldRef struct {
	ID   string
	Name string
}

// FieldValuesOf flattens an item's fieldValues union list into a slice of
// [FieldValueRef]. Returns nil when item or item.FieldValues is nil.
func FieldValuesOf(item *queries.ProjectV2ItemNode) []FieldValueRef {
	if item == nil || item.FieldValues == nil {
		return nil
	}
	out := make([]FieldValueRef, 0, len(item.FieldValues.Nodes))
	for _, vp := range item.FieldValues.Nodes {
		if vp == nil || *vp == nil {
			continue
		}
		switch v := (*vp).(type) {
		case *queries.ProjectV2ItemFieldValueProjectV2ItemFieldSingleSelectValue:
			out = append(out, FieldValueRef{
				Typename: "ProjectV2ItemFieldSingleSelectValue",
				Name:     derefString(v.Name),
				OptionID: derefString(v.OptionId),
				Field:    fieldRefOf(v.Field),
			})
		case *queries.ProjectV2ItemFieldValueProjectV2ItemFieldIterationValue:
			out = append(out, FieldValueRef{
				Typename:    "ProjectV2ItemFieldIterationValue",
				IterationID: v.IterationId,
				Title:       v.Title,
				StartDate:   v.StartDate,
				Duration:    v.Duration,
				Field:       fieldRefOf(v.Field),
			})
		case *queries.ProjectV2ItemFieldValueProjectV2ItemFieldTextValue:
			out = append(out, FieldValueRef{
				Typename: "ProjectV2ItemFieldTextValue",
				Text:     derefString(v.Text),
				Field:    fieldRefOf(v.Field),
			})
		case *queries.ProjectV2ItemFieldValueProjectV2ItemFieldDateValue:
			out = append(out, FieldValueRef{
				Typename: "ProjectV2ItemFieldDateValue",
				Date:     derefString(v.Date),
				Field:    fieldRefOf(v.Field),
			})
		}
	}
	return out
}

func fieldRefOf(f queries.ProjectV2ItemFieldValueFieldRef) FieldRef {
	if f == nil {
		return FieldRef{}
	}
	switch fc := f.(type) {
	case *queries.ProjectV2ItemFieldValueFieldRefProjectV2Field:
		return FieldRef{ID: fc.Id, Name: fc.Name}
	case *queries.ProjectV2ItemFieldValueFieldRefProjectV2IterationField:
		return FieldRef{ID: fc.Id, Name: fc.Name}
	case *queries.ProjectV2ItemFieldValueFieldRefProjectV2SingleSelectField:
		return FieldRef{ID: fc.Id, Name: fc.Name}
	}
	return FieldRef{}
}

// FieldDescriptor is a flat view of one entry in a project's fields list
// (the response of [ListProjectV2Fields]). DataType is the schema enum
// (e.g. "SINGLE_SELECT", "ITERATION", "TEXT"); Options is populated only
// for SINGLE_SELECT, Configuration only for ITERATION.
type FieldDescriptor struct {
	ID            string
	Name          string
	DataType      string
	Options       []FieldOption
	Configuration *IterationConfiguration
}

// FieldOption is a single-select field's option entry.
type FieldOption struct {
	ID   string
	Name string
}

// IterationConfiguration mirrors the genqlient-generated
// [queries.ProjectV2IterationConfig] but uses non-pointer slices of
// [IterationOption] so callers don't have to nil-guard inner pointers.
type IterationConfiguration struct {
	Iterations          []IterationOption
	CompletedIterations []IterationOption
}

// IterationOption is a single iteration entry in a project's iteration
// field configuration.
type IterationOption struct {
	ID        string
	Title     string
	StartDate string
	Duration  int
}

// FieldsOf flattens the fields slice from a [ListProjectV2Fields] response
// into a list of [FieldDescriptor]. Use together with [FieldsFromResponse]:
//
//	fields := projectitem.FieldsOf(projectitem.FieldsFromResponse(resp))
func FieldsOf(fields []queries.ProjectV2FieldNode) []FieldDescriptor {
	out := make([]FieldDescriptor, 0, len(fields))
	for _, f := range fields {
		if f == nil {
			continue
		}
		switch fc := f.(type) {
		case *queries.ProjectV2FieldNodeProjectV2Field:
			out = append(out, FieldDescriptor{
				ID:       fc.Id,
				Name:     fc.Name,
				DataType: string(fc.DataType),
			})
		case *queries.ProjectV2FieldNodeProjectV2IterationField:
			d := FieldDescriptor{
				ID:       fc.Id,
				Name:     fc.Name,
				DataType: string(fc.DataType),
			}
			if fc.Configuration != nil {
				d.Configuration = iterationConfigOf(fc.Configuration)
			}
			out = append(out, d)
		case *queries.ProjectV2FieldNodeProjectV2SingleSelectField:
			d := FieldDescriptor{
				ID:       fc.Id,
				Name:     fc.Name,
				DataType: string(fc.DataType),
			}
			d.Options = make([]FieldOption, 0, len(fc.Options))
			for _, opt := range fc.Options {
				if opt == nil {
					continue
				}
				d.Options = append(d.Options, FieldOption{ID: opt.Id, Name: opt.Name})
			}
			out = append(out, d)
		}
	}
	return out
}

func iterationConfigOf(c *queries.ProjectV2IterationConfig) *IterationConfiguration {
	out := &IterationConfiguration{
		Iterations:          make([]IterationOption, 0, len(c.Iterations)),
		CompletedIterations: make([]IterationOption, 0, len(c.CompletedIterations)),
	}
	for _, it := range c.Iterations {
		if it == nil {
			continue
		}
		out.Iterations = append(out.Iterations, IterationOption{
			ID: it.Id, Title: it.Title, StartDate: it.StartDate, Duration: it.Duration,
		})
	}
	for _, it := range c.CompletedIterations {
		if it == nil {
			continue
		}
		out.CompletedIterations = append(out.CompletedIterations, IterationOption{
			ID: it.Id, Title: it.Title, StartDate: it.StartDate, Duration: it.Duration,
		})
	}
	return out
}

// FindStatus returns the value of the conventionally-named "Status" single
// select field, or "" when the item has no Status set.
func FindStatus(values []FieldValueRef) string {
	for _, v := range values {
		if v.Typename == "ProjectV2ItemFieldSingleSelectValue" &&
			strings.EqualFold(v.Field.Name, "status") {
			return v.Name
		}
	}
	return ""
}

// FormatItem renders a multi-line "list" view of a Projects v2 item. The
// output matches the prior TS formatItem byte-for-byte:
//
//   - Issue / PullRequest: "<prefix>#<n>  <title>[  [Status]]\n  <url>\n"
//     (prefix is "PR" for PullRequest and empty for Issue)
//   - DraftIssue:          "(draft)  <title>[  [Status]]\n"
//   - missing content:     "(no content)[  [Status]]\n"
//
// Trailing newlines are intentional — callers write the result directly
// without adding their own newline.
func FormatItem(item *queries.ProjectV2ItemNode) string {
	statusSuffix := ""
	if status := FindStatus(FieldValuesOf(item)); status != "" {
		statusSuffix = "  [" + status + "]"
	}
	c := ContentOf(item)
	switch c.Typename {
	case "Issue":
		return fmt.Sprintf("#%d  %s%s\n  %s\n", c.Number, c.Title, statusSuffix, c.URL)
	case "PullRequest":
		return fmt.Sprintf("PR#%d  %s%s\n  %s\n", c.Number, c.Title, statusSuffix, c.URL)
	case "DraftIssue":
		return "(draft)  " + c.Title + statusSuffix + "\n"
	default:
		return "(no content)" + statusSuffix + "\n"
	}
}

// FormatItemLineCompact renders a single-line "compact" view of a Projects
// v2 item, used when the caller embeds the line into a bulleted Markdown
// list. Format matches the prior TS formatItemLine in review.ts / standup.ts:
//
//   - Issue / PullRequest: "<prefix>#<n> <title> (<url>)"
//   - DraftIssue:          "(draft) <title>"
//   - missing content:     "(no content)"
//
// No leading indent, no trailing newline, no Status suffix.
func FormatItemLineCompact(item *queries.ProjectV2ItemNode) string {
	c := ContentOf(item)
	switch c.Typename {
	case "Issue":
		return fmt.Sprintf("#%d %s (%s)", c.Number, c.Title, c.URL)
	case "PullRequest":
		return fmt.Sprintf("PR#%d %s (%s)", c.Number, c.Title, c.URL)
	case "DraftIssue":
		return "(draft) " + c.Title
	default:
		return "(no content)"
	}
}
