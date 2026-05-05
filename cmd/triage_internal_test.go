package cmd

import (
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
)

// makeStatusItem builds a ProjectV2ItemNode whose Status SINGLE_SELECT field
// carries the given option name. Used by isUntriaged / isItemDone tests to
// drive case-fold and value variations through projectitem.FindStatus.
//
// Pass status="" to construct an item whose fieldValues list is non-nil but
// empty (i.e. no Status field set at all), exercising the `status==""`
// branches in the matchers.
func makeStatusItem(status string) *queries.ProjectV2ItemNode {
	if status == "" {
		return &queries.ProjectV2ItemNode{
			Id: "PVTI_no_status",
			FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{
				Nodes: nil,
			},
		}
	}
	field := queries.ProjectV2ItemFieldValueFieldRef(&queries.ProjectV2ItemFieldValueFieldRefProjectV2SingleSelectField{
		Id:   "F_status",
		Name: "Status",
	})
	optID := "OPT_" + status
	val := queries.ProjectV2ItemFieldValue(&queries.ProjectV2ItemFieldValueProjectV2ItemFieldSingleSelectValue{
		Name:     &status,
		OptionId: &optID,
		Field:    field,
	})
	return &queries.ProjectV2ItemNode{
		Id: "PVTI_x",
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{
			Nodes: []*queries.ProjectV2ItemFieldValue{&val},
		},
	}
}

// TestIsUntriaged pins the contract that runTriageProject uses to filter
// ProjectV2 items into the "needs triage" bucket: status=="" OR
// EqualFold(status, "triage"). The case-insensitivity matters because
// project templates may use "Triage" / "triage" / "TRIAGE" — all three
// must classify identically, and a future refactor that swaps EqualFold
// for == would silently regress this.
func TestIsUntriaged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status string
		want   bool
	}{
		{status: "", want: true},
		{status: "triage", want: true},
		{status: "Triage", want: true},
		{status: "TRIAGE", want: true},
		{status: "TrIaGe", want: true},
		{status: "Todo", want: false},
		{status: "In Progress", want: false},
		{status: "Done", want: false},
		// "triaged" must not match "triage" — EqualFold compares full strings.
		{status: "triaged", want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.status, func(t *testing.T) {
			t.Parallel()
			got := isUntriaged(makeStatusItem(tc.status))
			if got != tc.want {
				t.Errorf("isUntriaged(status=%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
