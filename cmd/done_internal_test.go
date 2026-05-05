package cmd

import (
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
)

// TestFindStatusField pins the contract that runDoneProject uses to pick
// the Status SINGLE_SELECT field from a project's flattened FieldDescriptor
// list. The match is `EqualFold(Name, "status") AND DataType=="SINGLE_SELECT"`
// — both predicates matter, so we exercise each axis independently.
func TestFindStatusField(t *testing.T) {
	t.Parallel()

	t.Run("matches-single-select-status-exact-name", func(t *testing.T) {
		t.Parallel()
		fields := []projectitem.FieldDescriptor{
			{ID: "F1", Name: "Title", DataType: "TEXT"},
			{ID: "F2", Name: "Status", DataType: "SINGLE_SELECT"},
		}
		got := findStatusField(fields)
		if got == nil || got.ID != "F2" {
			t.Errorf("got %+v, want F2", got)
		}
	})

	t.Run("name-match-is-case-insensitive", func(t *testing.T) {
		t.Parallel()
		// EqualFold semantics: "STATUS" / "status" / "Status" must all match.
		fields := []projectitem.FieldDescriptor{
			{ID: "F1", Name: "STATUS", DataType: "SINGLE_SELECT"},
		}
		if got := findStatusField(fields); got == nil || got.ID != "F1" {
			t.Errorf("uppercase: got %+v", got)
		}
		fields[0].Name = "status"
		if got := findStatusField(fields); got == nil || got.ID != "F1" {
			t.Errorf("lowercase: got %+v", got)
		}
	})

	t.Run("ignores-non-single-select-status-named-field", func(t *testing.T) {
		t.Parallel()
		// A field literally named "Status" but typed as ITERATION (or TEXT)
		// must not be picked up — the data-type guard prevents
		// `UpdateProjectV2ItemFieldValue` from sending a SingleSelect option
		// id to a field that doesn't accept one.
		fields := []projectitem.FieldDescriptor{
			{ID: "F1", Name: "Status", DataType: "ITERATION"},
			{ID: "F2", Name: "Status", DataType: "TEXT"},
		}
		if got := findStatusField(fields); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("ignores-single-select-with-different-name", func(t *testing.T) {
		t.Parallel()
		fields := []projectitem.FieldDescriptor{
			{ID: "F1", Name: "Priority", DataType: "SINGLE_SELECT"},
		}
		if got := findStatusField(fields); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("returns-nil-on-empty-or-nil-input", func(t *testing.T) {
		t.Parallel()
		if got := findStatusField(nil); got != nil {
			t.Errorf("nil input: expected nil, got %+v", got)
		}
		if got := findStatusField([]projectitem.FieldDescriptor{}); got != nil {
			t.Errorf("empty input: expected nil, got %+v", got)
		}
	})

	t.Run("returns-first-match-on-duplicates", func(t *testing.T) {
		t.Parallel()
		// Defensive: the project schema disallows duplicate field names, but
		// the helper still returns the first hit deterministically rather
		// than panicking. Pin the order so a future change to "return last"
		// or "return error" is caught.
		fields := []projectitem.FieldDescriptor{
			{ID: "F_first", Name: "Status", DataType: "SINGLE_SELECT"},
			{ID: "F_second", Name: "status", DataType: "SINGLE_SELECT"},
		}
		got := findStatusField(fields)
		if got == nil || got.ID != "F_first" {
			t.Errorf("got %+v, want F_first", got)
		}
	})
}

func TestFindOption(t *testing.T) {
	t.Parallel()

	opts := []projectitem.FieldOption{
		{ID: "O_todo", Name: "Todo"},
		{ID: "O_doing", Name: "In Progress"},
		{ID: "O_done", Name: "Done"},
	}

	t.Run("exact-name-match", func(t *testing.T) {
		t.Parallel()
		got := findOption(opts, "Done")
		if got == nil || got.ID != "O_done" {
			t.Errorf("got %+v, want O_done", got)
		}
	})

	t.Run("case-insensitive-match", func(t *testing.T) {
		t.Parallel()
		// runDoneProject calls findOption with the literal "done" — the
		// option's display name might be "Done" / "DONE" / "done" depending
		// on the user's project template, so EqualFold matters.
		if got := findOption(opts, "done"); got == nil || got.ID != "O_done" {
			t.Errorf("lowercase: got %+v", got)
		}
		if got := findOption(opts, "DONE"); got == nil || got.ID != "O_done" {
			t.Errorf("uppercase: got %+v", got)
		}
	})

	t.Run("no-match-returns-nil", func(t *testing.T) {
		t.Parallel()
		if got := findOption(opts, "Archived"); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("multi-word-name-match", func(t *testing.T) {
		t.Parallel()
		// "In Progress" carries a space — pin that the helper uses literal
		// equality (mod fold), not whitespace normalisation.
		if got := findOption(opts, "In Progress"); got == nil || got.ID != "O_doing" {
			t.Errorf("got %+v", got)
		}
		if got := findOption(opts, "InProgress"); got != nil {
			t.Errorf("space-stripped variant must NOT match, got %+v", got)
		}
	})

	t.Run("empty-or-nil-input", func(t *testing.T) {
		t.Parallel()
		if got := findOption(nil, "Done"); got != nil {
			t.Errorf("nil input: got %+v", got)
		}
		if got := findOption([]projectitem.FieldOption{}, "Done"); got != nil {
			t.Errorf("empty input: got %+v", got)
		}
	})
}

// makeSingleSelectItem builds a synthetic ProjectV2ItemNode whose only
// field-value is a SINGLE_SELECT pointing at (fieldID, optionID). Used by
// TestIsAlreadyDone to drive isAlreadyDone through projectitem.FieldValuesOf.
func makeSingleSelectItem(fieldID, optionID string) *queries.ProjectV2ItemNode {
	statusFieldName := "Status"
	optName := "Done"
	field := queries.ProjectV2ItemFieldValueFieldRef(&queries.ProjectV2ItemFieldValueFieldRefProjectV2SingleSelectField{
		Id:   fieldID,
		Name: statusFieldName,
	})
	val := queries.ProjectV2ItemFieldValue(&queries.ProjectV2ItemFieldValueProjectV2ItemFieldSingleSelectValue{
		Name:     &optName,
		OptionId: &optionID,
		Field:    field,
	})
	return &queries.ProjectV2ItemNode{
		Id: "PVTI_x",
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{
			Nodes: []*queries.ProjectV2ItemFieldValue{&val},
		},
	}
}

// makeTextValueItem builds a ProjectV2ItemNode whose status-shaped slot is
// occupied by a TEXT value (not SingleSelect). Used to pin that
// isAlreadyDone discriminates on Typename, not just (fieldID, optionID).
func makeTextValueItem(fieldID, text string) *queries.ProjectV2ItemNode {
	field := queries.ProjectV2ItemFieldValueFieldRef(&queries.ProjectV2ItemFieldValueFieldRefProjectV2Field{
		Id:   fieldID,
		Name: "Status",
	})
	val := queries.ProjectV2ItemFieldValue(&queries.ProjectV2ItemFieldValueProjectV2ItemFieldTextValue{
		Text:  &text,
		Field: field,
	})
	return &queries.ProjectV2ItemNode{
		Id: "PVTI_x",
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{
			Nodes: []*queries.ProjectV2ItemFieldValue{&val},
		},
	}
}

func TestIsAlreadyDone(t *testing.T) {
	t.Parallel()

	t.Run("matching-field-and-option-returns-true", func(t *testing.T) {
		t.Parallel()
		item := makeSingleSelectItem("F_status", "O_done")
		if !isAlreadyDone(item, "F_status", "O_done") {
			t.Error("expected true on exact (field, option) match")
		}
	})

	t.Run("different-option-id-returns-false", func(t *testing.T) {
		t.Parallel()
		// Same status field, but item is parked on Todo — must not be
		// reported as already-done.
		item := makeSingleSelectItem("F_status", "O_todo")
		if isAlreadyDone(item, "F_status", "O_done") {
			t.Error("expected false when option ids differ")
		}
	})

	t.Run("different-field-id-returns-false", func(t *testing.T) {
		t.Parallel()
		// Match on option-id alone is not enough — the value must belong to
		// the resolved Status field. Defensive: prevents false positives
		// if a different SingleSelect field coincidentally uses the same
		// option-id as Status.Done.
		item := makeSingleSelectItem("F_other", "O_done")
		if isAlreadyDone(item, "F_status", "O_done") {
			t.Error("expected false when field ids differ")
		}
	})

	t.Run("non-single-select-typename-returns-false", func(t *testing.T) {
		t.Parallel()
		// A TEXT value attached to the Status field must never be classified
		// as already-done, even if its underlying ids match — Typename is
		// the discriminator.
		item := makeTextValueItem("F_status", "Done")
		if isAlreadyDone(item, "F_status", "O_done") {
			t.Error("expected false for non-SingleSelect typename")
		}
	})

	t.Run("nil-item-returns-false", func(t *testing.T) {
		t.Parallel()
		// projectitem.FieldValuesOf is nil-safe; the helper inherits that.
		if isAlreadyDone(nil, "F_status", "O_done") {
			t.Error("expected false for nil item")
		}
	})

	t.Run("empty-field-values-returns-false", func(t *testing.T) {
		t.Parallel()
		item := &queries.ProjectV2ItemNode{
			Id: "PVTI_empty",
			FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{
				Nodes: nil,
			},
		}
		if isAlreadyDone(item, "F_status", "O_done") {
			t.Error("expected false for empty field-values")
		}
	})
}
