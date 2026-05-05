package github_test

import (
	"context"
	"errors"
	"testing"

	gqlclient "github.com/Khan/genqlient/graphql"
	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

func TestAsGenqlientClient_NilReceiver(t *testing.T) {
	t.Parallel()

	var c *github.Clients
	if got := c.AsGenqlientClient(); got != nil {
		t.Fatalf("nil Clients should yield nil adapter, got %#v", got)
	}
}

func TestGenqlientAdapter_NoVariables(t *testing.T) {
	t.Parallel()

	rec := &testfake.RecordingGraphQL{
		Resp: map[string]any{"viewer": map[string]any{"login": "alice"}},
	}
	clients := &github.Clients{GraphQL: rec}
	adapter := clients.AsGenqlientClient()

	type viewerResp struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
	}
	data := &viewerResp{}
	req := &gqlclient.Request{
		OpName: "GetViewerLogin",
		Query:  "query GetViewerLogin { viewer { login } }",
	}
	resp := &gqlclient.Response{Data: data}

	if err := adapter.MakeRequest(context.Background(), req, resp); err != nil {
		t.Fatalf("MakeRequest: %v", err)
	}
	if len(rec.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(rec.Calls))
	}
	if rec.Calls[0].Vars != nil {
		t.Fatalf("expected nil vars (no variables), got %#v", rec.Calls[0].Vars)
	}
	if data.Viewer.Login != "alice" {
		t.Fatalf("Login = %q, want %q", data.Viewer.Login, "alice")
	}
}

func TestGenqlientAdapter_StructVariablesRoundTrip(t *testing.T) {
	t.Parallel()

	type ListRepoIssuesVars struct {
		Owner string `json:"owner"`
		Name  string `json:"name"`
		First int    `json:"first"`
	}

	rec := &testfake.RecordingGraphQL{}
	clients := &github.Clients{GraphQL: rec}
	adapter := clients.AsGenqlientClient()

	vars := &ListRepoIssuesVars{Owner: "ozzy-labs", Name: "gh-tasks", First: 50}
	req := &gqlclient.Request{
		OpName:    "ListRepoIssues",
		Query:     "query ListRepoIssues($owner: String!, $name: String!, $first: Int!) { repository { id } }",
		Variables: vars,
	}
	resp := &gqlclient.Response{Data: &struct{}{}}

	if err := adapter.MakeRequest(context.Background(), req, resp); err != nil {
		t.Fatalf("MakeRequest: %v", err)
	}

	want := map[string]any{
		"owner": "ozzy-labs",
		"name":  "gh-tasks",
		"first": float64(50), // json.Unmarshal into map[string]any decodes numbers as float64
	}
	if diff := cmp.Diff(want, rec.Calls[0].Vars); diff != "" {
		t.Fatalf("vars mismatch (-want +got):\n%s", diff)
	}
}

func TestGenqlientAdapter_ErrorPropagates(t *testing.T) {
	t.Parallel()

	cause := errors.New("upstream boom")
	rec := &testfake.RecordingGraphQL{Err: cause}
	clients := &github.Clients{GraphQL: rec}
	adapter := clients.AsGenqlientClient()

	req := &gqlclient.Request{Query: "query Q { __typename }"}
	resp := &gqlclient.Response{Data: &struct{}{}}

	err := adapter.MakeRequest(context.Background(), req, resp)
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is should succeed against the wrapped cause; got %v", err)
	}
}

func TestAsGenqlientClientFor_RoundTripsTypedResponse(t *testing.T) {
	t.Parallel()

	// Smoke test against one of the read operations migrated under #230 to
	// confirm the adapter + generated bindings decode a wire-shaped payload
	// into the typed response struct.
	rec := &testfake.RecordingGraphQL{
		Resp: map[string]any{
			"repository": map[string]any{"id": "R_kg2c"},
		},
	}
	adapter := github.AsGenqlientClientFor(rec)

	resp, err := queries.GetRepositoryID(context.Background(), adapter, "ozzy-labs", "gh-tasks")
	if err != nil {
		t.Fatalf("GetRepositoryID: %v", err)
	}
	if resp == nil || resp.Repository == nil {
		t.Fatal("expected non-nil repository in response")
	}
	if got, want := resp.Repository.Id, "R_kg2c"; got != want {
		t.Fatalf("Repository.Id = %q, want %q", got, want)
	}
	if len(rec.Calls) != 1 {
		t.Fatalf("expected 1 underlying call, got %d", len(rec.Calls))
	}
	wantVars := map[string]any{"owner": "ozzy-labs", "name": "gh-tasks"}
	if diff := cmp.Diff(wantVars, rec.Calls[0].Vars); diff != "" {
		t.Fatalf("vars mismatch (-want +got):\n%s", diff)
	}
}

func TestGenqlientAdapter_NilInner(t *testing.T) {
	t.Parallel()

	clients := &github.Clients{GraphQL: nil}
	adapter := clients.AsGenqlientClient()

	req := &gqlclient.Request{Query: "query Q { __typename }"}
	resp := &gqlclient.Response{Data: &struct{}{}}
	if err := adapter.MakeRequest(context.Background(), req, resp); err == nil {
		t.Fatalf("expected error for nil inner client, got nil")
	}
}

// TestGenqlientAdapter_TypedNilPointerVariables exercises the
// `string(buf) == "null"` early return inside marshalVars: when a caller
// passes a typed nil pointer (e.g. `(*FooVars)(nil)`), json.Marshal
// renders it as `"null"` and the adapter must collapse that back to nil
// vars rather than passing an empty `{}` payload to the underlying
// GraphQL client.
func TestGenqlientAdapter_TypedNilPointerVariables(t *testing.T) {
	t.Parallel()

	type FooVars struct {
		Owner string `json:"owner"`
	}
	rec := &testfake.RecordingGraphQL{Resp: map[string]any{}}
	clients := &github.Clients{GraphQL: rec}
	adapter := clients.AsGenqlientClient()

	req := &gqlclient.Request{
		Query:     "query Q { __typename }",
		Variables: (*FooVars)(nil),
	}
	resp := &gqlclient.Response{Data: &struct{}{}}
	if err := adapter.MakeRequest(context.Background(), req, resp); err != nil {
		t.Fatalf("MakeRequest: %v", err)
	}
	if len(rec.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(rec.Calls))
	}
	if rec.Calls[0].Vars != nil {
		t.Errorf("typed nil pointer must collapse to nil vars, got %#v", rec.Calls[0].Vars)
	}
}

// TestGenqlientAdapter_UnmarshalableVariables exercises the
// `json.Marshal` error branch in marshalVars: when Variables contains a
// channel (which encoding/json refuses), the adapter must surface the
// wrapped error instead of silently passing an empty map.
func TestGenqlientAdapter_UnmarshalableVariables(t *testing.T) {
	t.Parallel()

	type ChanVars struct {
		C chan int `json:"c"`
	}
	rec := &testfake.RecordingGraphQL{}
	clients := &github.Clients{GraphQL: rec}
	adapter := clients.AsGenqlientClient()

	req := &gqlclient.Request{
		Query:     "query Q { __typename }",
		Variables: &ChanVars{C: make(chan int)},
	}
	resp := &gqlclient.Response{Data: &struct{}{}}
	err := adapter.MakeRequest(context.Background(), req, resp)
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	if len(rec.Calls) != 0 {
		t.Errorf("inner client must not be called when marshal fails, got %d calls", len(rec.Calls))
	}
}
