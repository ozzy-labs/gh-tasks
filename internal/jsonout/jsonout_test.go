package jsonout_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/jsonout"
)

var testCatalog = jsonout.FieldList{
	{Name: "id", Description: "the id"},
	{Name: "title", Description: "the title"},
	{Name: "labels", Description: "the labels"},
}

// TestRender_AllFieldsWhenEmpty pins the contract that an empty fields
// slice emits every catalog key — and only catalog keys — for each row.
// Non-catalog keys present on the input map (e.g. helper-internal hints)
// are filtered out so the JSON output never leaks unstable fields.
func TestRender_AllFieldsWhenEmpty(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"id": "I_a", "title": "A", "labels": []string{"bug"}, "internal": "leak"},
	}
	var buf bytes.Buffer
	if err := jsonout.Render(&buf, items, nil, "", testCatalog); err != nil {
		t.Fatalf("Render: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, buf.String())
	}
	if got[0]["id"] != "I_a" || got[0]["title"] != "A" {
		t.Errorf("missing core fields: %v", got[0])
	}
	if _, leaked := got[0]["internal"]; leaked {
		t.Errorf("non-catalog field leaked: %v", got[0])
	}
}

// TestRender_FilterToRequestedFields pins that only requested fields
// appear, regardless of the order they were given (json marshal sorts
// keys alphabetically — that's the gh-aligned contract).
func TestRender_FilterToRequestedFields(t *testing.T) {
	t.Parallel()
	items := []map[string]any{{"id": "I_a", "title": "A", "labels": []string{"x"}}}
	var buf bytes.Buffer
	if err := jsonout.Render(&buf, items, []string{"title", "id"}, "", testCatalog); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.String()
	for _, want := range []string{`"id":`, `"title":`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing requested field %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"labels":`) {
		t.Errorf("unrequested field 'labels' must not appear:\n%s", got)
	}
}

// TestRender_MissingKeyBecomesNull pins the null-vs-omitempty contract.
// When a requested field has no value on the input row, the output must
// still carry the key with a JSON null value.
func TestRender_MissingKeyBecomesNull(t *testing.T) {
	t.Parallel()
	items := []map[string]any{{"id": "I_a"}}
	var buf bytes.Buffer
	if err := jsonout.Render(&buf, items, []string{"id", "title"}, "", testCatalog); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), `"title": null`) {
		t.Errorf("missing field must serialize as null, got:\n%s", buf.String())
	}
}

// TestRender_UnknownField surfaces UnknownFieldError with sorted field
// names so callers get a stable error message regardless of input order.
func TestRender_UnknownField(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := jsonout.Render(&buf, nil, []string{"zzz", "aaa"}, "", testCatalog)
	var ufe *jsonout.UnknownFieldError
	if !errors.As(err, &ufe) {
		t.Fatalf("expected UnknownFieldError, got %v", err)
	}
	if ufe.Fields[0] != "aaa" || ufe.Fields[1] != "zzz" {
		t.Errorf("expected sorted fields, got %v", ufe.Fields)
	}
	if buf.Len() != 0 {
		t.Errorf("stdout buffer must be empty on validation failure, got: %s", buf.String())
	}
}

// TestRender_JQ exercises the gojq integration end-to-end including the
// json round-trip that converts []map[string]any into the generic shapes
// gojq expects (see jsonout.go comment on the marshal/unmarshal cycle).
func TestRender_JQ(t *testing.T) {
	t.Parallel()
	items := []map[string]any{
		{"id": "I_a", "title": "A"},
		{"id": "I_b", "title": "B"},
	}
	var buf bytes.Buffer
	if err := jsonout.Render(&buf, items, []string{"id"}, ".[].id", testCatalog); err != nil {
		t.Fatalf("Render: %v", err)
	}
	got := buf.String()
	for _, want := range []string{`"I_a"`, `"I_b"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in jq output:\n%s", want, got)
		}
	}
}

// TestRender_JQParseError pins that gojq parse failures bubble up as a
// JQError so cmd/* can show a clean diagnostic instead of the raw cryptic
// gojq output.
func TestRender_JQParseError(t *testing.T) {
	t.Parallel()
	items := []map[string]any{{"id": "I_a"}}
	var buf bytes.Buffer
	err := jsonout.Render(&buf, items, []string{"id"}, "nonsense @ token", testCatalog)
	var jqErr *jsonout.JQError
	if !errors.As(err, &jqErr) {
		t.Fatalf("expected JQError, got %v", err)
	}
	if jqErr.Phase != "parse" {
		t.Errorf("expected parse phase, got %q", jqErr.Phase)
	}
}

// TestListFields_PrintsAllNames pins the discoverability contract: every
// catalog name appears in the listing output, with descriptions.
func TestListFields_PrintsAllNames(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	jsonout.ListFields(&buf, testCatalog)
	got := buf.String()
	for _, want := range []string{"id", "title", "labels", "the id", "the title", "the labels"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// TestRender_EmptyCatalog rejects an empty catalog: cmd/* must always
// declare its public field surface.
func TestRender_EmptyCatalog(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := jsonout.Render(&buf, nil, nil, "", jsonout.FieldList{})
	if err == nil || !strings.Contains(err.Error(), "empty catalog") {
		t.Errorf("expected empty-catalog error, got %v", err)
	}
}
