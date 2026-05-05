package testfake_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

// Compile-time pin that both fakes satisfy the production GraphQLClient
// surface. If this stops compiling, every consumer of testfake (cmd /
// internal/projectitem / internal/github tests) is at risk.
var (
	_ github.GraphQLClient = (*testfake.FakeGraphQL)(nil)
	_ github.GraphQLClient = (*testfake.RecordingGraphQL)(nil)
)

type sampleResp struct {
	Repository struct {
		Name string `json:"name"`
	} `json:"repository"`
}

func TestFakeGraphQL_Do_DataPath(t *testing.T) {
	t.Parallel()

	t.Run("matched-substring-decodes-data-into-out", func(t *testing.T) {
		t.Parallel()
		f := &testfake.FakeGraphQL{
			Responses: []testfake.FakeResponse{{
				MatchSubstring: "query GetRepo(",
				Data: map[string]any{
					"repository": map[string]any{"name": "gh-tasks"},
				},
			}},
		}
		var out sampleResp
		if err := f.Do(context.Background(), "query GetRepo($id:ID!){...}", nil, &out); err != nil {
			t.Fatalf("Do: %v", err)
		}
		if out.Repository.Name != "gh-tasks" {
			t.Errorf("out.Repository.Name = %q, want gh-tasks", out.Repository.Name)
		}
	})

	t.Run("err-returned-verbatim-without-decoding-data", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("transport boom")
		f := &testfake.FakeGraphQL{
			Responses: []testfake.FakeResponse{{
				MatchSubstring: "query X(",
				Data:           map[string]any{"should": "be ignored"},
				Err:            sentinel,
			}},
		}
		var out sampleResp
		err := f.Do(context.Background(), "query X(){...}", nil, &out)
		if !errors.Is(err, sentinel) {
			t.Fatalf("Do err = %v, want sentinel", err)
		}
		if out.Repository.Name != "" {
			t.Errorf("out should be untouched on Err path, got %+v", out)
		}
	})

	t.Run("unmatched-query-returns-error", func(t *testing.T) {
		t.Parallel()
		f := &testfake.FakeGraphQL{
			Responses: []testfake.FakeResponse{{
				MatchSubstring: "query A(",
				Data:           map[string]any{},
			}},
		}
		var out sampleResp
		err := f.Do(context.Background(), "query B(){...}", nil, &out)
		if err == nil {
			t.Fatalf("expected error for unmatched query")
		}
		if !strings.Contains(err.Error(), "no fake response matched") {
			t.Errorf("err = %q, want unmatched-query message", err.Error())
		}
	})
}

func TestFakeGraphQL_Do_CursorAdvances(t *testing.T) {
	t.Parallel()

	// Two entries sharing the same MatchSubstring: the cursor must advance
	// after the first match so the second call replays the second entry.
	f := &testfake.FakeGraphQL{
		Responses: []testfake.FakeResponse{
			{MatchSubstring: "query Page(", Data: map[string]any{
				"repository": map[string]any{"name": "page-1"},
			}},
			{MatchSubstring: "query Page(", Data: map[string]any{
				"repository": map[string]any{"name": "page-2"},
			}},
		},
	}

	var out sampleResp
	if err := f.Do(context.Background(), "query Page(){...}", nil, &out); err != nil {
		t.Fatalf("first Do: %v", err)
	}
	if out.Repository.Name != "page-1" {
		t.Errorf("first call: name = %q, want page-1", out.Repository.Name)
	}

	out = sampleResp{}
	if err := f.Do(context.Background(), "query Page(){...}", nil, &out); err != nil {
		t.Fatalf("second Do: %v", err)
	}
	if out.Repository.Name != "page-2" {
		t.Errorf("second call: name = %q, want page-2", out.Repository.Name)
	}

	// Third call must fail — cursor exhausted, no entry left to match.
	if err := f.Do(context.Background(), "query Page(){...}", nil, &out); err == nil {
		t.Errorf("third Do: expected error after cursor exhaustion")
	}
}

func TestFakeGraphQL_Do_OrderEnforced(t *testing.T) {
	t.Parallel()

	// Even though the second entry would also match, the first unconsumed
	// entry must be picked. After consuming entry #0, a query that only
	// matches entry #0 must fail because the cursor is already past it.
	f := &testfake.FakeGraphQL{
		Responses: []testfake.FakeResponse{
			{MatchSubstring: "query First(", Data: map[string]any{}},
			{MatchSubstring: "query Second(", Data: map[string]any{}},
		},
	}

	var out sampleResp
	if err := f.Do(context.Background(), "query First(){...}", nil, &out); err != nil {
		t.Fatalf("first Do: %v", err)
	}
	// Replaying "query First(" should now miss — cursor is at index 1, where
	// only "query Second(" can match.
	err := f.Do(context.Background(), "query First(){...}", nil, &out)
	if err == nil {
		t.Errorf("expected error replaying already-consumed substring")
	}
}

func TestFakeGraphQL_Do_MarshalRoundTrip(t *testing.T) {
	t.Parallel()

	// Pin the json.Marshal -> json.Unmarshal pipeline: a typed Data struct
	// must reach a typed out struct intact even though the intermediate is
	// generic JSON.
	type input struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}
	type output struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}
	want := input{Foo: "hello", Bar: 42}

	f := &testfake.FakeGraphQL{
		Responses: []testfake.FakeResponse{{
			MatchSubstring: "query Q(",
			Data:           want,
		}},
	}
	var got output
	if err := f.Do(context.Background(), "query Q(){...}", nil, &got); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if diff := cmp.Diff(input(want), input(got)); diff != "" {
		t.Errorf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestRecordingGraphQL_Do_RecordsCalls(t *testing.T) {
	t.Parallel()

	r := &testfake.RecordingGraphQL{}

	vars1 := map[string]any{"a": 1}
	vars2 := map[string]any{"b": "two"}
	if err := r.Do(context.Background(), "query A", vars1, nil); err != nil {
		t.Fatalf("first Do: %v", err)
	}
	if err := r.Do(context.Background(), "query B", vars2, nil); err != nil {
		t.Fatalf("second Do: %v", err)
	}

	want := []testfake.RecordedCall{
		{Query: "query A", Vars: vars1},
		{Query: "query B", Vars: vars2},
	}
	if diff := cmp.Diff(want, r.Calls); diff != "" {
		t.Errorf("Calls mismatch (-want +got):\n%s", diff)
	}
}

func TestRecordingGraphQL_Do_ResponsePaths(t *testing.T) {
	t.Parallel()

	t.Run("resp-decoded-into-out", func(t *testing.T) {
		t.Parallel()
		r := &testfake.RecordingGraphQL{
			Resp: map[string]any{
				"repository": map[string]any{"name": "captured"},
			},
		}
		var out sampleResp
		if err := r.Do(context.Background(), "q", nil, &out); err != nil {
			t.Fatalf("Do: %v", err)
		}
		if out.Repository.Name != "captured" {
			t.Errorf("out = %+v", out)
		}
	})

	t.Run("err-returned-verbatim", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("recording boom")
		r := &testfake.RecordingGraphQL{Err: sentinel}
		var out sampleResp
		err := r.Do(context.Background(), "q", nil, &out)
		if !errors.Is(err, sentinel) {
			t.Errorf("Do err = %v, want sentinel", err)
		}
	})

	t.Run("nil-resp-and-nil-out-is-noop", func(t *testing.T) {
		t.Parallel()
		// The contract: a nil Resp + nil out must be a valid empty response.
		// No marshal/unmarshal happens, so no error is produced.
		r := &testfake.RecordingGraphQL{}
		if err := r.Do(context.Background(), "q", nil, nil); err != nil {
			t.Errorf("Do: %v", err)
		}
		if len(r.Calls) != 1 {
			t.Errorf("Calls len = %d, want 1", len(r.Calls))
		}
	})

	t.Run("nil-out-skips-decoding-even-with-resp", func(t *testing.T) {
		t.Parallel()
		// out == nil short-circuits the decode regardless of Resp, so a
		// caller that doesn't care about the response body can just pass nil.
		r := &testfake.RecordingGraphQL{Resp: map[string]any{"x": 1}}
		if err := r.Do(context.Background(), "q", nil, nil); err != nil {
			t.Errorf("Do: %v", err)
		}
	})
}
