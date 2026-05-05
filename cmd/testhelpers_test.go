// Package cmd_test shared fixtures.
//
// All cobra-rooted flow tests in this package share a single mock surface:
// a JSON-driven [fakeGraphQL] keyed by an operation-name + paren substring,
// and a recording [recordingREST] keyed by `<METHOD> <path-substring>`.
// This file consolidates those primitives and the [testDeps] / [runCmd]
// boilerplate so individual test files focus on scenarios rather than
// rebuilding scaffolding. See `docs/design/test-structure.md` for the
// rationale and naming convention (`Test<Cmd>_<Scenario>`).
package cmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/config"
	"github.com/ozzy-labs/gh-tasks/internal/github"
)

// ===== GraphQL fake ========================================================

// fakeGraphQL implements github.GraphQLClient for tests. Each call is matched
// against responses keyed by query substring, in registration order. To
// disambiguate prefix-overlapping operations (e.g. ListRepoIssues vs
// ListRepoIssuesWithLabels), matchSubstring values use the
// `query <Name>(` form.
type fakeGraphQL struct {
	responses []fakeResponse
	idx       int
}

type fakeResponse struct {
	matchSubstring string
	data           any
	err            error
}

func (f *fakeGraphQL) Do(_ context.Context, query string, _ map[string]any, out any) error {
	for i := f.idx; i < len(f.responses); i++ {
		r := f.responses[i]
		if !strings.Contains(query, r.matchSubstring) {
			continue
		}
		f.idx = i + 1
		if r.err != nil {
			return r.err
		}
		buf, err := json.Marshal(r.data)
		if err != nil {
			return fmt.Errorf("marshal fake response: %w", err)
		}
		return json.Unmarshal(buf, out)
	}
	return fmt.Errorf("no fake response matched query: %q", query)
}

// captureGraphQL wraps a fakeGraphQL to peek at queries / vars without
// disturbing the canned-response replay. Tests that need to assert on
// outbound variables (e.g. limits, mutation inputs) wrap their inner
// fakeGraphQL with this and inspect the captured value after Execute.
type captureGraphQL struct {
	inner   *fakeGraphQL
	capture func(query string, vars map[string]any)
}

func (c *captureGraphQL) Do(ctx context.Context, query string, vars map[string]any, out any) error {
	if c.capture != nil {
		c.capture(query, vars)
	}
	return c.inner.Do(ctx, query, vars, out)
}

// intFromVar extracts an integer from a variable captured by [captureGraphQL].
// Genqlient-generated calls pass variables through JSON, so numeric values
// are unmarshalled to float64; hand-written call sites still pass ints
// directly. This helper accepts both.
func intFromVar(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case float64:
		return int(x)
	}
	return 0
}

// ===== REST fakes ==========================================================

// fakeREST is the no-op REST client used by tests that don't exercise REST.
type fakeREST struct{}

func (fakeREST) Do(context.Context, string, string, any, any) error { return nil }

// recordingREST captures REST calls and replays canned responses keyed by
// `<METHOD> <path-substring>`. Unmatched calls return a marshalable empty
// body (nil out, nil err).
type recordingREST struct {
	calls     []restCall
	responses []restResponse
}

type restCall struct {
	method string
	path   string
	body   any
}

type restResponse struct {
	matchMethod string
	matchPath   string
	data        any
	err         error
}

func (r *recordingREST) Do(_ context.Context, method, path string, body, out any) error {
	r.calls = append(r.calls, restCall{method: method, path: path, body: body})
	for _, resp := range r.responses {
		if resp.matchMethod != "" && resp.matchMethod != method {
			continue
		}
		if resp.matchPath != "" && !strings.Contains(path, resp.matchPath) {
			continue
		}
		if resp.err != nil {
			return resp.err
		}
		if out == nil || resp.data == nil {
			return nil
		}
		buf, err := json.Marshal(resp.data)
		if err != nil {
			return fmt.Errorf("marshal rest fake: %w", err)
		}
		return json.Unmarshal(buf, out)
	}
	return nil
}

// ===== Client + Deps factories =============================================

// newClients builds a github.Clients with the supplied GraphQL fake and the
// no-op REST client. Tests that don't exercise REST should use this.
func newClients(g *fakeGraphQL) *github.Clients {
	return &github.Clients{Host: "github.com", GraphQL: g, REST: fakeREST{}}
}

// newClientsWithREST is a counterpart to newClients that lets a test inject a
// recordingREST in addition to the GraphQL fake.
func newClientsWithREST(g *fakeGraphQL, r *recordingREST) *github.Clients {
	return &github.Clients{Host: "github.com", GraphQL: g, REST: r}
}

// testDeps returns a baseline cmd.Deps wired to the given GraphQL fake. Opts
// can override individual fields (HasGitRemote, LoadConfig, NewClients ...)
// to drive scope=org/user paths or inject a [captureGraphQL] wrapper.
func testDeps(g *fakeGraphQL, opts ...func(*cmd.Deps)) cmd.Deps {
	d := cmd.Deps{
		Stdout:       new(bytes.Buffer),
		Stderr:       new(bytes.Buffer),
		Now:          func() time.Time { return time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC) },
		Env:          func(string) string { return "" },
		HasGitRemote: func() bool { return true },
		GetRemoteURL: func() (string, bool) { return "git@github.com:ozzy-labs/gh-tasks.git", true },
		NewClients:   func() (*github.Clients, error) { return newClients(g), nil },
		LoadConfig:   func() (config.AppConfig, error) { return config.AppConfig{}, nil },
	}
	for _, opt := range opts {
		opt(&d)
	}
	return d
}

// runCmd is a small helper that bootstraps the cobra root with the supplied
// deps + args, captures stdout/stderr into the bytes.Buffers wired into
// d.Stdout / d.Stderr, and returns the resulting err. It also wires
// SetOut/SetErr on the root command itself, since cmd handlers prefer
// c.OutOrStdout()/c.ErrOrStderr() over deps.Stdout/deps.Stderr.
//
// Flags are parsed entirely by cobra from args; the legacy Deps.Argv field
// has been retired in favour of authoritative cobra flag handling.
func runCmd(t *testing.T, d cmd.Deps, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	stdout = d.Stdout.(*bytes.Buffer)
	stderr = d.Stderr.(*bytes.Buffer)
	root := cmd.RootWithDeps(d)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err = root.Execute()
	return stdout, stderr, err
}

// runCmdWithContext is the context-aware variant of runCmd. It uses cobra's
// ExecuteContext so c.Context() inside the RunE handler reflects the supplied
// context (instead of the implicit context.Background() that Execute() uses).
// Tests that exercise context cancellation / deadline propagation rely on
// this helper to inject a pre-cancelled or short-deadline context.
func runCmdWithContext(t *testing.T, ctx context.Context, d cmd.Deps, args ...string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	stdout = d.Stdout.(*bytes.Buffer)
	stderr = d.Stderr.(*bytes.Buffer)
	root := cmd.RootWithDeps(d)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	err = root.ExecuteContext(ctx)
	return stdout, stderr, err
}

// ctxAwareGraphQL wraps a fakeGraphQL so that any Do call short-circuits to
// ctx.Err() when the supplied context is already done. The bare fakeGraphQL
// ignores ctx (responses are pre-canned), which is fine for happy-path tests
// but unsuitable for context-cancel scenarios where the paginator's per-page
// Do call must observe the cancellation. Tests that need to simulate a
// Ctrl-C / deadline expiry wrap their fake with this so the cmd layer sees
// context.Canceled (or DeadlineExceeded) propagate up through the paginator.
type ctxAwareGraphQL struct {
	inner *fakeGraphQL
}

func (c *ctxAwareGraphQL) Do(ctx context.Context, query string, vars map[string]any, out any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.inner.Do(ctx, query, vars, out)
}

// ===== GraphQL payload builders ============================================

// repoIssuesPayload constructs the `repository.issues.nodes` shape consumed by
// ListRepoIssues across many tests.
func repoIssuesPayload(nodes ...map[string]any) map[string]any {
	return map[string]any{
		"repository": map[string]any{
			"issues": map[string]any{"nodes": append([]any{}, asAnySlice(nodes)...)},
		},
	}
}

func asAnySlice(in []map[string]any) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// emptyOrgProject builds a `nil` projectV2 envelope under organization (used
// to drive the not-found path in scope=org tests).
func emptyOrgProject() map[string]any {
	return map[string]any{"organization": map[string]any{"projectV2": nil}}
}

func orgProject(id string) map[string]any {
	return map[string]any{"organization": map[string]any{"projectV2": map[string]any{
		"id": id, "number": 7, "title": "Org Project",
	}}}
}

func userProject(id string) map[string]any {
	return map[string]any{"user": map[string]any{"projectV2": map[string]any{
		"id": id, "number": 9, "title": "User Project",
	}}}
}
