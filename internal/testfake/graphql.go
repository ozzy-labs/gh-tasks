// Package testfake provides shared test doubles used across the gh-tasks
// test suite. It exists because Go's test-helper convention (package-private
// _test.go files) cannot be reused from a sibling package's tests, and three
// of our packages (cmd, internal/projectitem, internal/github) needed the
// same shape of GraphQL fake (#284 Phase 1).
//
// The package is import-time inert and the production binary never references
// it — only `_test.go` files in other packages pull it in, so Go's per-package
// build does not link it into the release artefact.
//
// Two transports are exposed:
//
//   - [FakeGraphQL]: ordered, substring-keyed canned responses. Mirrors the
//     behaviour of the legacy `fakeGraphQL` in `cmd/testhelpers_test.go`. Each
//     entry's `MatchSubstring` is matched against the outbound query, and the
//     first matching unconsumed entry is replayed (advancing an internal
//     cursor so tests can pin the expected call order).
//
//   - [RecordingGraphQL]: every call is captured into a slice for later
//     assertion. Suitable for tests that need to verify the exact (query,
//     variables) pair delivered by the genqlient adapter.
//
// Both implement [github.com/ozzy-labs/gh-tasks/internal/github.GraphQLClient]
// (signature: `Do(ctx, query, vars, out) error`).
package testfake

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// FakeGraphQL implements `github.GraphQLClient` for tests. Each call is
// matched against responses keyed by query substring, in registration order.
// To disambiguate prefix-overlapping operations (e.g. ListRepoIssues vs
// ListRepoIssuesWithLabels), MatchSubstring values use the
// `query <Name>(` form.
//
// Once a response is consumed, the internal cursor advances so the next call
// can only match a later entry — this lets tests pin both the expected call
// order and the expected count.
type FakeGraphQL struct {
	Responses []FakeResponse
	idx       int
}

// FakeResponse pairs a substring matcher with either a canned data payload
// (encoded via json.Marshal then json.Unmarshal into out) or an Err to
// return verbatim from Do.
type FakeResponse struct {
	MatchSubstring string
	Data           any
	Err            error
}

// Do replays the first unconsumed response whose MatchSubstring is contained
// in query. Returns an error if no entry matches — tests should never hit
// this branch in steady state (it indicates a missing response fixture).
func (f *FakeGraphQL) Do(_ context.Context, query string, _ map[string]any, out any) error {
	for i := f.idx; i < len(f.Responses); i++ {
		r := f.Responses[i]
		if !strings.Contains(query, r.MatchSubstring) {
			continue
		}
		f.idx = i + 1
		if r.Err != nil {
			return r.Err
		}
		buf, err := json.Marshal(r.Data)
		if err != nil {
			return fmt.Errorf("marshal fake response: %w", err)
		}
		return json.Unmarshal(buf, out)
	}
	return fmt.Errorf("no fake response matched query: %q", query)
}

// RecordingGraphQL captures every Do call so adapter tests can assert on the
// exact (query, variables, out-type) tuple delivered to the underlying
// transport. Implements `github.GraphQLClient`.
//
// Unlike FakeGraphQL, RecordingGraphQL replays the same single Resp / Err on
// every call — it's optimised for tests that issue exactly one call and want
// to inspect what crossed the boundary, not for multi-call flows.
type RecordingGraphQL struct {
	Calls []RecordedCall
	Resp  any
	Err   error
}

// RecordedCall is a snapshot of one Do invocation for later inspection.
type RecordedCall struct {
	Query string
	Vars  map[string]any
}

// Do appends the call to Calls and replays Resp / Err. A nil Resp + nil out
// is treated as a valid empty response (no marshal/unmarshal performed).
func (r *RecordingGraphQL) Do(_ context.Context, query string, vars map[string]any, out any) error {
	r.Calls = append(r.Calls, RecordedCall{Query: query, Vars: vars})
	if r.Err != nil {
		return r.Err
	}
	if r.Resp == nil || out == nil {
		return nil
	}
	buf, err := json.Marshal(r.Resp)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, out)
}
