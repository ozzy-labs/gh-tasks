package queries_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
)

// Wire format pin tests for genqlient input types that gh-tasks sends to
// GitHub. These pin the JSON shape that goes over the wire, complementing
// flow tests in cmd/ which only check the call graph (testfake.FakeGraphQL
// matches by query name and ignores the request body).
//
// Categories pinned per mutation:
//
//   - Default: zero value beyond required fields. The hand-written
//     constructor must replace nil-prone list fields with `[]string{}`
//     so the JSON output has no `"X": null` for nullable lists (see
//     docs/design/genqlient-quirks.md "Pattern 1").
//   - Optional fields populated: body / assignees / labels — make sure
//     the field shows up correctly when set.
//
// `ProjectV2FieldValue` (oneOf input) gets its own block: the custom
// `MarshalJSON` is the workaround for "Pattern 2" (oneOf nested null
// rejection).

// ---- AddProjectV2DraftIssueInput ----

func TestWire_AddProjectV2DraftIssue_Default(t *testing.T) {
	t.Parallel()
	got := wireOf(t, queries.NewAddProjectV2DraftIssueInput("PVT_x", "title"))
	assertNotNullArray(t, got, "assigneeIds")
	if got["projectId"] != "PVT_x" {
		t.Errorf("projectId=%v want PVT_x", got["projectId"])
	}
	if got["title"] != "title" {
		t.Errorf("title=%v want title", got["title"])
	}
}

func TestWire_AddProjectV2DraftIssue_WithBody(t *testing.T) {
	t.Parallel()
	in := queries.NewAddProjectV2DraftIssueInput("PVT_x", "title")
	body := "Hello"
	in.Body = &body
	got := wireOf(t, in)
	if got["body"] != body {
		t.Errorf("body=%v want %q", got["body"], body)
	}
}

func TestWire_AddProjectV2DraftIssue_WithAssignees(t *testing.T) {
	t.Parallel()
	in := queries.NewAddProjectV2DraftIssueInput("PVT_x", "title")
	in.AssigneeIds = []string{"U_kgDOAAAAAA1", "U_kgDOAAAAAA2"}
	got := wireOf(t, in)
	arr, ok := got["assigneeIds"].([]any)
	if !ok || len(arr) != 2 {
		t.Errorf("assigneeIds=%v want length 2 array", got["assigneeIds"])
	}
}

// ---- CreateIssueInput ----

func TestWire_CreateIssue_Default(t *testing.T) {
	t.Parallel()
	got := wireOf(t, queries.NewCreateIssueInput("R_x", "title"))
	for _, k := range []string{"assigneeIds", "labelIds", "projectIds", "projectV2Ids"} {
		assertNotNullArray(t, got, k)
	}
	if got["repositoryId"] != "R_x" {
		t.Errorf("repositoryId=%v want R_x", got["repositoryId"])
	}
}

func TestWire_CreateIssue_WithBody(t *testing.T) {
	t.Parallel()
	in := queries.NewCreateIssueInput("R_x", "title")
	body := "details"
	in.Body = &body
	got := wireOf(t, in)
	if got["body"] != body {
		t.Errorf("body=%v want %q", got["body"], body)
	}
}

// ---- UpdatePullRequestInput ----

func TestWire_UpdatePullRequest_Default(t *testing.T) {
	t.Parallel()
	got := wireOf(t, queries.NewUpdatePullRequestInput("PR_x"))
	for _, k := range []string{"assigneeIds", "labelIds", "projectIds"} {
		assertNotNullArray(t, got, k)
	}
	if got["pullRequestId"] != "PR_x" {
		t.Errorf("pullRequestId=%v want PR_x", got["pullRequestId"])
	}
}

func TestWire_UpdatePullRequest_WithBody(t *testing.T) {
	t.Parallel()
	in := queries.NewUpdatePullRequestInput("PR_x")
	body := "Closes #42"
	in.Body = &body
	got := wireOf(t, in)
	if got["body"] != body {
		t.Errorf("body=%v want %q", got["body"], body)
	}
}

// ---- UpdateIssueInput ----

func TestWire_UpdateIssue_Default(t *testing.T) {
	t.Parallel()
	got := wireOf(t, queries.NewUpdateIssueInput("I_x"))
	for _, k := range []string{"assigneeIds", "labelIds", "projectIds"} {
		assertNotNullArray(t, got, k)
	}
	if got["id"] != "I_x" {
		t.Errorf("id=%v want I_x", got["id"])
	}
}

func TestWire_UpdateIssue_WithMilestone(t *testing.T) {
	t.Parallel()
	in := queries.NewUpdateIssueInput("I_x")
	ms := "M_42"
	in.MilestoneId = &ms
	got := wireOf(t, in)
	if got["milestoneId"] != ms {
		t.Errorf("milestoneId=%v want %q", got["milestoneId"], ms)
	}
}

// ---- ProjectV2FieldValue (oneOf MarshalJSON) ----

func TestWire_ProjectV2FieldValue_SingleSelectOnly(t *testing.T) {
	t.Parallel()
	id := "98236657"
	v := &queries.ProjectV2FieldValue{SingleSelectOptionId: &id}
	got := wireOf(t, v)
	want := map[string]any{"singleSelectOptionId": id}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("oneOf wire mismatch (-want +got):\n%s", diff)
	}
}

func TestWire_ProjectV2FieldValue_TextOnly(t *testing.T) {
	t.Parallel()
	text := "hello"
	v := &queries.ProjectV2FieldValue{Text: &text}
	got := wireOf(t, v)
	want := map[string]any{"text": text}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("text-only wire mismatch (-want +got):\n%s", diff)
	}
}

func TestWire_ProjectV2FieldValue_NumberOnly(t *testing.T) {
	t.Parallel()
	n := 1.5
	v := &queries.ProjectV2FieldValue{Number: &n}
	got := wireOf(t, v)
	want := map[string]any{"number": n}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("number-only wire mismatch (-want +got):\n%s", diff)
	}
}

func TestWire_ProjectV2FieldValue_DateOnly(t *testing.T) {
	t.Parallel()
	d := "2026-05-09"
	v := &queries.ProjectV2FieldValue{Date: &d}
	got := wireOf(t, v)
	want := map[string]any{"date": d}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("date-only wire mismatch (-want +got):\n%s", diff)
	}
}

func TestWire_ProjectV2FieldValue_IterationOnly(t *testing.T) {
	t.Parallel()
	id := "iter_1"
	v := &queries.ProjectV2FieldValue{IterationId: &id}
	got := wireOf(t, v)
	want := map[string]any{"iterationId": id}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("iteration-only wire mismatch (-want +got):\n%s", diff)
	}
}

// TestWire_ProjectV2FieldValue_AllSet pins that MarshalJSON does NOT
// silently drop fields when more than one is set. The "exactly one"
// validation is GitHub's responsibility; the local code must not hide
// the violation by omitting fields.
func TestWire_ProjectV2FieldValue_AllSet(t *testing.T) {
	t.Parallel()
	text := "x"
	num := 1.0
	v := &queries.ProjectV2FieldValue{Text: &text, Number: &num}
	got := wireOf(t, v)
	if got["text"] != text || got["number"] != num {
		t.Errorf("two-field set wire mismatch: got=%v", got)
	}
	for _, k := range []string{"date", "iterationId", "singleSelectOptionId"} {
		if _, present := got[k]; present {
			t.Errorf("%s should be omitted; got=%v", k, got)
		}
	}
}

// TestWire_ProjectV2FieldValue_Empty pins the empty-object output.
// Returning `{}` lets GitHub surface its "exactly one" error rather
// than the JSON encoder choking on a totally null struct.
func TestWire_ProjectV2FieldValue_Empty(t *testing.T) {
	t.Parallel()
	v := &queries.ProjectV2FieldValue{}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != "{}" {
		t.Errorf("empty oneOf wire = %s; want {}", b)
	}
}

// ---- helpers ----

// wireOf marshals v to JSON and decodes into a map for shape inspection.
// Failing fast at marshal/unmarshal makes shape assertions read clearly.
func wireOf(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v\npayload=%s", err, b)
	}
	return got
}

// assertNotNullArray pins the workaround for genqlient nullable list
// serialization (docs/design/genqlient-quirks.md "Pattern 1"): the
// field must be a JSON array, never null.
func assertNotNullArray(t *testing.T, got map[string]any, key string) {
	t.Helper()
	if got[key] == nil {
		t.Errorf("%s serialized as null; want []. payload=%v", key, got)
		return
	}
	if _, ok := got[key].([]any); !ok {
		t.Errorf("%s = %v (type %T); want array", key, got[key], got[key])
	}
}
