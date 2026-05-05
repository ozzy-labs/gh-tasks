package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
)

// issueItemForPlan builds a minimal ProjectV2ItemNode whose union content is
// an Issue. Field values are wired as raw genqlient nodes so each test can
// stage exactly the iteration / field-id combination it cares about.
func issueItemForPlan(id string, num int, title string, fieldValues ...queries.ProjectV2ItemFieldValue) *queries.ProjectV2ItemNode {
	var content queries.ProjectV2ItemContent = &queries.ProjectV2ItemContentIssue{
		Id:     "I_" + id,
		Number: num,
		Title:  title,
		Url:    "https://example.test/i/" + id,
	}
	item := &queries.ProjectV2ItemNode{
		Id:      "ITEM_" + id,
		Content: &content,
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{
			Nodes: make([]*queries.ProjectV2ItemFieldValue, 0, len(fieldValues)),
		},
	}
	for i := range fieldValues {
		v := fieldValues[i]
		item.FieldValues.Nodes = append(item.FieldValues.Nodes, &v)
	}
	return item
}

// prItemForPlan builds a minimal ProjectV2ItemNode whose union content is a
// PullRequest.
func prItemForPlan(id string, num int, title string) *queries.ProjectV2ItemNode {
	var content queries.ProjectV2ItemContent = &queries.ProjectV2ItemContentPullRequest{
		Id:     "P_" + id,
		Number: num,
		Title:  title,
		Url:    "https://example.test/p/" + id,
	}
	return &queries.ProjectV2ItemNode{
		Id:          "ITEM_" + id,
		Content:     &content,
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{},
	}
}

// draftItemForPlan builds a minimal ProjectV2ItemNode whose union content is
// a DraftIssue.
func draftItemForPlan(id, title string) *queries.ProjectV2ItemNode {
	var content queries.ProjectV2ItemContent = &queries.ProjectV2ItemContentDraftIssue{
		Id:    "DI_" + id,
		Title: title,
	}
	return &queries.ProjectV2ItemNode{
		Id:          "ITEM_" + id,
		Content:     &content,
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{},
	}
}

// emptyItemForPlan builds a ProjectV2ItemNode with no Content union resolved.
func emptyItemForPlan(id string) *queries.ProjectV2ItemNode {
	return &queries.ProjectV2ItemNode{
		Id:          "ITEM_" + id,
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{},
	}
}

// iterationFieldValueForPlan builds a ProjectV2ItemFieldIterationValue with
// the supplied field id and iteration id. isAlreadyOnIteration matches on
// both, so we surface them explicitly rather than relying on a single
// "title"-only fixture.
func iterationFieldValueForPlan(fieldID, iterationID string) queries.ProjectV2ItemFieldValue {
	field := queries.ProjectV2ItemFieldValueFieldRef(&queries.ProjectV2ItemFieldValueFieldRefProjectV2IterationField{
		Id:   fieldID,
		Name: "Iteration",
	})
	return queries.ProjectV2ItemFieldValue(&queries.ProjectV2ItemFieldValueProjectV2ItemFieldIterationValue{
		IterationId: iterationID,
		Title:       "Sprint",
		StartDate:   "2026-05-04",
		Duration:    7,
		Field:       field,
	})
}

func TestIsAlreadyOnIteration(t *testing.T) {
	t.Parallel()

	const fieldID = "F_iter"
	const iterID = "ITER_current"

	cases := []struct {
		name    string
		item    *queries.ProjectV2ItemNode
		fieldID string
		iterID  string
		want    bool
	}{
		{
			name:    "matches-field-and-iteration",
			item:    issueItemForPlan("a", 1, "x", iterationFieldValueForPlan(fieldID, iterID)),
			fieldID: fieldID,
			iterID:  iterID,
			want:    true,
		},
		{
			name:    "different-iteration-id",
			item:    issueItemForPlan("b", 2, "y", iterationFieldValueForPlan(fieldID, "ITER_other")),
			fieldID: fieldID,
			iterID:  iterID,
			want:    false,
		},
		{
			name:    "different-field-id",
			item:    issueItemForPlan("c", 3, "z", iterationFieldValueForPlan("F_other", iterID)),
			fieldID: fieldID,
			iterID:  iterID,
			want:    false,
		},
		{
			name:    "no-iteration-value-set",
			item:    issueItemForPlan("d", 4, "w"),
			fieldID: fieldID,
			iterID:  iterID,
			want:    false,
		},
		{
			name:    "nil-item",
			item:    nil,
			fieldID: fieldID,
			iterID:  iterID,
			want:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isAlreadyOnIteration(tc.item, tc.fieldID, tc.iterID)
			if got != tc.want {
				t.Errorf("isAlreadyOnIteration() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFormatItemLineForPlan(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		item *queries.ProjectV2ItemNode
		want string
	}{
		{
			name: "issue",
			item: issueItemForPlan("a", 42, "Fix login"),
			want: "  #42  Fix login\n",
		},
		{
			name: "pull-request",
			item: prItemForPlan("b", 7, "Add cache"),
			want: "  PR#7  Add cache\n",
		},
		{
			name: "draft",
			item: draftItemForPlan("c", "Draft idea"),
			want: "  (draft)  Draft idea\n",
		},
		{
			name: "no-content",
			item: emptyItemForPlan("d"),
			want: "  (no content)\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatItemLineForPlan(tc.item)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("formatItemLineForPlan() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDescribeItem(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		item *queries.ProjectV2ItemNode
		want string
	}{
		{
			name: "issue",
			item: issueItemForPlan("a", 42, "Fix login"),
			want: "#42",
		},
		{
			name: "pull-request",
			item: prItemForPlan("b", 7, "Add cache"),
			want: "PR#7",
		},
		{
			name: "draft-falls-back-to-item-id",
			item: draftItemForPlan("c", "Draft idea"),
			want: "ITEM_c",
		},
		{
			name: "no-content-falls-back-to-item-id",
			item: emptyItemForPlan("d"),
			want: "ITEM_d",
		},
		{
			name: "nil-item",
			item: nil,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := describeItem(tc.item)
			if got != tc.want {
				t.Errorf("describeItem() = %q, want %q", got, tc.want)
			}
		})
	}
}
