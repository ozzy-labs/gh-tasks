package cmd

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
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

// TestResolveTargetIteration covers the three-tier match precedence inside
// [resolveTargetIteration]: exact title match, current-window
// (`iterationContains`) match, and the cold-fallback ladder for cases where
// neither the title nor the clock lands inside any configured iteration.
//
// The cold-fallback ladder is the path that audit A flagged (lines 308-331):
// among iterations whose start date is on or after `now`, pick the soonest
// start (SliceStable preserves catalog order on ties). When even that set is
// empty (only-past iterations), the last entry of the original list is
// returned as a last-resort anchor so callers always have *some* iteration
// to plan against.
func TestResolveTargetIteration(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	current := projectitem.IterationOption{ID: "IT_curr", Title: "Sprint A", StartDate: "2026-05-01", Duration: 14}
	exactTitle := projectitem.IterationOption{ID: "IT_target", Title: "Daily 2026-05-04", StartDate: "2026-05-04", Duration: 1}
	pastA := projectitem.IterationOption{ID: "IT_pastA", Title: "Past 1", StartDate: "2026-04-01", Duration: 7}
	pastB := projectitem.IterationOption{ID: "IT_pastB", Title: "Past 2", StartDate: "2026-04-15", Duration: 7}
	futureNear := projectitem.IterationOption{ID: "IT_near", Title: "Future Near", StartDate: "2026-05-10", Duration: 7}
	futureFar := projectitem.IterationOption{ID: "IT_far", Title: "Future Far", StartDate: "2026-06-01", Duration: 7}
	// Same start as futureNear; SliceStable keeps catalog order (futureNear
	// appears earlier in the input slice below).
	futureTie := projectitem.IterationOption{ID: "IT_tie", Title: "Future Tie", StartDate: "2026-05-10", Duration: 7}
	bogusStart := projectitem.IterationOption{ID: "IT_bogus", Title: "Bogus", StartDate: "not-a-date", Duration: 7}

	cases := []struct {
		name        string
		iterations  []projectitem.IterationOption
		target      string
		wantNil     bool
		wantID      string
		wantMatched bool
	}{
		{
			name:       "empty-iterations-returns-nil",
			iterations: nil,
			target:     "Daily 2026-05-04",
			wantNil:    true,
		},
		{
			name:        "title-match-is-preferred",
			iterations:  []projectitem.IterationOption{current, exactTitle, futureNear},
			target:      "Daily 2026-05-04",
			wantID:      "IT_target",
			wantMatched: true,
		},
		{
			name:        "no-title-match-falls-back-to-current-window",
			iterations:  []projectitem.IterationOption{pastA, current, futureNear},
			target:      "Daily 2026-05-04",
			wantID:      "IT_curr",
			wantMatched: false,
		},
		{
			name:        "no-current-window-picks-soonest-future",
			iterations:  []projectitem.IterationOption{pastA, futureFar, futureNear},
			target:      "Daily 2026-05-04",
			wantID:      "IT_near",
			wantMatched: false,
		},
		{
			name:        "future-tie-preserves-catalog-order",
			iterations:  []projectitem.IterationOption{pastA, futureNear, futureTie},
			target:      "Daily 2026-05-04",
			wantID:      "IT_near",
			wantMatched: false,
		},
		{
			name:        "only-past-iterations-falls-back-to-last-entry",
			iterations:  []projectitem.IterationOption{pastA, pastB},
			target:      "Daily 2026-05-04",
			wantID:      "IT_pastB",
			wantMatched: false,
		},
		{
			name:        "unparseable-start-dates-are-ignored-then-last-entry",
			iterations:  []projectitem.IterationOption{pastA, bogusStart},
			target:      "Daily 2026-05-04",
			wantID:      "IT_bogus",
			wantMatched: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := resolveTargetIteration(tc.iterations, tc.target, now)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("resolveTargetIteration() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("resolveTargetIteration() = nil, want non-nil id=%s", tc.wantID)
			}
			if got.iteration.ID != tc.wantID {
				t.Errorf("iteration.ID = %q, want %q", got.iteration.ID, tc.wantID)
			}
			if got.matched != tc.wantMatched {
				t.Errorf("matched = %v, want %v", got.matched, tc.wantMatched)
			}
		})
	}
}

// TestFindIterationField pins the two-pass selection inside
// [findIterationField]: pass 1 finds the conventionally-named "Iteration"
// field, pass 2 falls back to the first ITERATION-typed field regardless of
// name. The fallback path matters for projects whose iteration field has
// been renamed in the UI (e.g. "Sprint", "Cadence").
func TestFindIterationField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		fields  []projectitem.FieldDescriptor
		wantID  string
		wantNil bool
	}{
		{
			name: "first-pass-name-and-datatype-match",
			fields: []projectitem.FieldDescriptor{
				{ID: "F_status", Name: "Status", DataType: "SINGLE_SELECT"},
				{ID: "F_iter", Name: "Iteration", DataType: "ITERATION"},
			},
			wantID: "F_iter",
		},
		{
			name: "first-pass-is-case-insensitive",
			fields: []projectitem.FieldDescriptor{
				{ID: "F_iter", Name: "iTeRaTiOn", DataType: "ITERATION"},
			},
			wantID: "F_iter",
		},
		{
			name: "datatype-fallback-when-name-differs",
			fields: []projectitem.FieldDescriptor{
				{ID: "F_status", Name: "Status", DataType: "SINGLE_SELECT"},
				{ID: "F_sprint", Name: "Sprint", DataType: "ITERATION"},
			},
			wantID: "F_sprint",
		},
		{
			name: "literal-iteration-name-with-wrong-datatype-is-skipped",
			fields: []projectitem.FieldDescriptor{
				// Name says "Iteration" but datatype is TEXT — pass 1 fails,
				// pass 2 then picks the genuine ITERATION field.
				{ID: "F_text", Name: "Iteration", DataType: "TEXT"},
				{ID: "F_real", Name: "Cadence", DataType: "ITERATION"},
			},
			wantID: "F_real",
		},
		{
			name: "no-iteration-field-returns-nil",
			fields: []projectitem.FieldDescriptor{
				{ID: "F_status", Name: "Status", DataType: "SINGLE_SELECT"},
				{ID: "F_text", Name: "Notes", DataType: "TEXT"},
			},
			wantNil: true,
		},
		{
			name:    "empty-fields-returns-nil",
			fields:  []projectitem.FieldDescriptor{},
			wantNil: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := findIterationField(tc.fields)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("findIterationField() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("findIterationField() = nil, want id=%s", tc.wantID)
			}
			if got.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tc.wantID)
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
