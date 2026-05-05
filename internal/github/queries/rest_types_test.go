package queries_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
)

// TestCreateMilestoneResult_Unmarshal pins the JSON tag bindings of the
// hand-coded REST response shape used by `cmd/plan.go`. The wire format is
// fixed by GitHub's REST v3 docs for POST /repos/{owner}/{repo}/milestones,
// so a regression here would only be caused by an accidental edit to
// rest_types.go. Pinning the tags decouples that risk from the cmd-level
// flow tests, where a tag rename would surface as a confusing "milestone
// number 0" or empty node-id in the user-visible URL.
func TestCreateMilestoneResult_Unmarshal(t *testing.T) {
	t.Parallel()

	// Sample shaped after the documented response of
	//   POST /repos/{owner}/{repo}/milestones
	// — extra fields are present in real responses and must be tolerated
	// without erroring (CreateMilestoneResult only decodes the four
	// fields cmd/plan.go actually consumes).
	body := []byte(`{
		"url": "https://api.github.com/repos/o/r/milestones/12",
		"html_url": "https://github.com/o/r/milestone/12",
		"id": 1234567,
		"node_id": "MDk6TWlsZXN0b25lMTIzNDU2Nw==",
		"number": 12,
		"title": "Sprint 12",
		"description": "two-week iteration",
		"open_issues": 3,
		"closed_issues": 1,
		"state": "open"
	}`)

	var got queries.CreateMilestoneResult
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	want := queries.CreateMilestoneResult{
		NodeID: "MDk6TWlsZXN0b25lMTIzNDU2Nw==",
		ID:     1234567,
		Number: 12,
		Title:  "Sprint 12",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("CreateMilestoneResult mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateMilestoneResult_UnmarshalRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	var got queries.CreateMilestoneResult
	if err := json.Unmarshal([]byte("not json"), &got); err == nil {
		t.Fatal("expected error on malformed JSON, got nil")
	}
}

func TestCreateMilestoneResult_UnmarshalRejectsTypeMismatch(t *testing.T) {
	t.Parallel()

	// A common API regression mode: a numeric field arrives as a string.
	// json.Unmarshal must surface this as an error rather than silently
	// keeping the zero value, so plan.go does not bind a freshly-created
	// milestone with number=0.
	body := []byte(`{"node_id":"X","id":1,"number":"twelve","title":"Bad"}`)
	var got queries.CreateMilestoneResult
	if err := json.Unmarshal(body, &got); err == nil {
		t.Errorf("expected type-mismatch error, got nil; result=%+v", got)
	}
}
