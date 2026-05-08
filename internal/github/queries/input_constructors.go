package queries

import "encoding/json"

// Hand-written workarounds for genqlient-generated input types that
// serialize as JSON `null` when left at their zero value. GitHub's
// Projects v2 mutations reject explicit `null` for several fields:
//
//  1. Nullable list inputs (`[ID!]` in the schema → `[]string` in Go):
//     no `omitempty` tag means a nil slice marshals as `null`. The
//     `addProjectV2DraftIssue.assigneeIds` field reproducibly fails
//     with a generic "Something went wrong" 500 error in this case
//     (confirmed 2026-05-09).
//
//  2. The `ProjectV2FieldValue` "oneOf" sub-input: all five sub-fields
//     are nullable pointers without `omitempty`, so the four unset
//     fields marshal as explicit `null` and GitHub rejects with
//     "must include exactly one of the following arguments" because
//     it counts present-but-null as "specified".
//
// `New*` constructors below pre-populate affected list fields with an
// empty slice (`[]string{}`) so the JSON payload contains `[]`. The
// `MarshalJSON` override on `*ProjectV2FieldValue` emits only the
// non-nil sub-field. If genqlient gains per-field `omitempty` /
// per-type oneOf handling, this file can be deleted and the call sites
// can return to direct struct literals.

// NewAddProjectV2DraftIssueInput returns an input for the
// `addProjectV2DraftIssue` mutation with `AssigneeIds` initialized to a
// non-nil empty slice.
func NewAddProjectV2DraftIssueInput(projectID, title string) *AddProjectV2DraftIssueInput {
	return &AddProjectV2DraftIssueInput{
		ProjectId:   projectID,
		Title:       title,
		AssigneeIds: []string{},
	}
}

// NewCreateIssueInput returns an input for the `createIssue` mutation
// with all four nullable list fields initialized to non-nil empty slices.
func NewCreateIssueInput(repositoryID, title string) *CreateIssueInput {
	return &CreateIssueInput{
		RepositoryId: repositoryID,
		Title:        title,
		AssigneeIds:  []string{},
		LabelIds:     []string{},
		ProjectIds:   []string{},
		ProjectV2Ids: []string{},
	}
}

// NewUpdatePullRequestInput returns an input for the `updatePullRequest`
// mutation with the three nullable list fields initialized.
func NewUpdatePullRequestInput(pullRequestID string) *UpdatePullRequestInput {
	return &UpdatePullRequestInput{
		PullRequestId: pullRequestID,
		AssigneeIds:   []string{},
		LabelIds:      []string{},
		ProjectIds:    []string{},
	}
}

// NewUpdateIssueInput returns an input for the `updateIssue` mutation
// with the three nullable list fields initialized.
func NewUpdateIssueInput(id string) *UpdateIssueInput {
	return &UpdateIssueInput{
		Id:          id,
		AssigneeIds: []string{},
		LabelIds:    []string{},
		ProjectIds:  []string{},
	}
}

// MarshalJSON emits only the non-nil sub-field of ProjectV2FieldValue.
// GitHub's `updateProjectV2ItemFieldValue` mutation enforces a "oneOf"
// constraint on this input and treats any present-but-null field as
// "specified", so the genqlient-default serialization (5 explicit
// nulls + 1 set value) is rejected. Returning an empty object when no
// field is set lets the GraphQL layer surface the "exactly one"
// validation error instead of double-faulting at JSON encoding.
func (v *ProjectV2FieldValue) MarshalJSON() ([]byte, error) {
	out := map[string]any{}
	if v.Date != nil {
		out["date"] = *v.Date
	}
	if v.IterationId != nil {
		out["iterationId"] = *v.IterationId
	}
	if v.Number != nil {
		out["number"] = *v.Number
	}
	if v.SingleSelectOptionId != nil {
		out["singleSelectOptionId"] = *v.SingleSelectOptionId
	}
	if v.Text != nil {
		out["text"] = *v.Text
	}
	return json.Marshal(out)
}
