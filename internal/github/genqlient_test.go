package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	gqlclient "github.com/Khan/genqlient/graphql"
	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/github"
)

// recordingGraphQL captures every Do call so adapter tests can assert on the
// exact (query, variables, out-type) tuple delivered to the underlying
// transport. Implements [github.GraphQLClient].
type recordingGraphQL struct {
	calls []recordedCall
	resp  any
	err   error
}

type recordedCall struct {
	query string
	vars  map[string]any
}

func (r *recordingGraphQL) Do(_ context.Context, query string, vars map[string]any, out any) error {
	r.calls = append(r.calls, recordedCall{query: query, vars: vars})
	if r.err != nil {
		return r.err
	}
	if r.resp == nil || out == nil {
		return nil
	}
	buf, err := json.Marshal(r.resp)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, out)
}

func TestAsGenqlientClient_NilReceiver(t *testing.T) {
	t.Parallel()

	var c *github.Clients
	if got := c.AsGenqlientClient(); got != nil {
		t.Fatalf("nil Clients should yield nil adapter, got %#v", got)
	}
}

func TestGenqlientAdapter_NoVariables(t *testing.T) {
	t.Parallel()

	rec := &recordingGraphQL{
		resp: map[string]any{"viewer": map[string]any{"login": "alice"}},
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
	if len(rec.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(rec.calls))
	}
	if rec.calls[0].vars != nil {
		t.Fatalf("expected nil vars (no variables), got %#v", rec.calls[0].vars)
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

	rec := &recordingGraphQL{}
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
	if diff := cmp.Diff(want, rec.calls[0].vars); diff != "" {
		t.Fatalf("vars mismatch (-want +got):\n%s", diff)
	}
}

func TestGenqlientAdapter_ErrorPropagates(t *testing.T) {
	t.Parallel()

	cause := errors.New("upstream boom")
	rec := &recordingGraphQL{err: cause}
	clients := &github.Clients{GraphQL: rec}
	adapter := clients.AsGenqlientClient()

	req := &gqlclient.Request{Query: "query Q { __typename }"}
	resp := &gqlclient.Response{Data: &struct{}{}}

	err := adapter.MakeRequest(context.Background(), req, resp)
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is should succeed against the wrapped cause; got %v", err)
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
