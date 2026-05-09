// Package cmd_test shared fixtures.
//
// All cobra-rooted flow tests in this package share a single mock surface:
// a JSON-driven [testfake.FakeGraphQL] keyed by an operation-name + paren
// substring, and a recording [recordingREST] keyed by `<METHOD>
// <path-substring>`. This file consolidates the [testDeps] / [runCmd]
// boilerplate so individual test files focus on scenarios rather than
// rebuilding scaffolding. The GraphQL fake itself lives in
// `internal/testfake` so the same primitive can serve cmd, projectitem, and
// github tests without three duplicate copies (#284). See
// `docs/design/test-structure.md` for the rationale and naming convention
// (`Test<Cmd>_<Scenario>`).
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
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

// ===== GraphQL fake ========================================================
//
// FakeGraphQL / FakeResponse are now exported from internal/testfake so the
// same fake serves cmd, internal/projectitem, and internal/github test
// suites (#284 Phase 1). captureGraphQL and ctxAwareGraphQL wrap that shared
// type to add per-test concerns (variable capture, ctx-aware short-circuit).

// captureGraphQL wraps a testfake.FakeGraphQL to peek at queries / vars
// without disturbing the canned-response replay. Tests that need to assert
// on outbound variables (e.g. limits, mutation inputs) wrap their inner
// fake with this and inspect the captured value after Execute.
type captureGraphQL struct {
	inner   *testfake.FakeGraphQL
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

// assertNoLeadingOrDoubleSlashInRESTPath fails the test when any recorded REST
// call uses a path starting with "/" or containing "//". go-gh's restPrefix
// already supplies the trailing slash, so a leading "/" produces a malformed
// URL like `https://api.github.com//repos/...` that returns HTTP 404. Callers
// of github.RESTClient.Do MUST pass paths without a leading "/".
func assertNoLeadingOrDoubleSlashInRESTPath(t *testing.T, r *recordingREST) {
	t.Helper()
	for _, c := range r.calls {
		if strings.HasPrefix(c.path, "/") {
			t.Errorf("REST path must not start with %q; got %q (method %s)", "/", c.path, c.method)
		}
		if strings.Contains(c.path, "//") {
			t.Errorf("REST path must not contain %q; got %q (method %s)", "//", c.path, c.method)
		}
	}
}

// ===== Client + Deps factories =============================================

// newClients builds a github.Clients with the supplied GraphQL fake and the
// no-op REST client. Tests that don't exercise REST should use this.
func newClients(g *testfake.FakeGraphQL) *github.Clients {
	return &github.Clients{Host: "github.com", GraphQL: g, REST: fakeREST{}}
}

// newClientsWithREST is a counterpart to newClients that lets a test inject a
// recordingREST in addition to the GraphQL fake.
func newClientsWithREST(g *testfake.FakeGraphQL, r *recordingREST) *github.Clients {
	return &github.Clients{Host: "github.com", GraphQL: g, REST: r}
}

// testDeps returns a baseline cmd.Deps wired to the given GraphQL fake. Opts
// can override individual fields (HasGitRemote, LoadConfig, NewClients ...)
// to drive scope=org/user paths or inject a [captureGraphQL] wrapper.
func testDeps(g *testfake.FakeGraphQL, opts ...func(*cmd.Deps)) cmd.Deps {
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

// ctxAwareGraphQL wraps a testfake.FakeGraphQL so that any Do call
// short-circuits to ctx.Err() when the supplied context is already done.
// The bare FakeGraphQL ignores ctx (responses are pre-canned), which is fine
// for happy-path tests
// but unsuitable for context-cancel scenarios where the paginator's per-page
// Do call must observe the cancellation. Tests that need to simulate a
// Ctrl-C / deadline expiry wrap their fake with this so the cmd layer sees
// context.Canceled (or DeadlineExceeded) propagate up through the paginator.
type ctxAwareGraphQL struct {
	inner *testfake.FakeGraphQL
}

func (c *ctxAwareGraphQL) Do(ctx context.Context, query string, vars map[string]any, out any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.inner.Do(ctx, query, vars, out)
}

// ===== i18n assertion helpers ==============================================

// assertI18nMessage checks that haystack contains the localized rendering of
// the given catalog key for the supplied locale. Replaces the older
// `strings.Contains(stderr, "<hardcoded English>")` pattern: any future
// catalog wording change automatically flows through to the assertion via
// i18n.T, so editing en.json doesn't break a test that doesn't care about
// the exact phrasing — only that the right message reached the user.
//
// args is the same flat (key, value) varargs accepted by i18n.T (e.g.
//
//	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
//	    "error.project.notFound",
//	    "owner", "octo", "number", 7, "scope", "org")
//
// renders the en string with all three placeholders substituted before
// matching).
//
// On failure the helper logs both the rendered expected message and the
// actual haystack so the test failure surfaces the wording mismatch
// directly — diagnosing a stale catalog key is a one-line read.
func assertI18nMessage(t *testing.T, haystack string, loc i18n.Locale, key string, args ...any) {
	t.Helper()
	expected := i18n.T(loc, key, args...)
	if !strings.Contains(haystack, expected) {
		t.Errorf("haystack does not contain expected i18n message %q (key %q):\n%s",
			expected, key, haystack)
	}
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

// ===== JSON output helpers (#367 PR 1) ====================================

// parseJSONArray decodes stdout as a JSON array of objects. Tests use this
// to assert on the structured `--json` output without re-implementing the
// parse boilerplate. Fails the test on parse error so callers can chain
// item-level assertions safely.
func parseJSONArray(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("stdout is not a JSON array: %v\nstdout=%s", err, stdout)
	}
	return rows
}

// assertJSONFieldEquals indexes into a JSON-array stdout and asserts that
// the named field on the given row matches `want`. Comparison goes through
// json.Marshal on both sides so int / float64 quirks (Go's default JSON
// number type is float64 for any) compare structurally rather than by Go
// type. Use this for stable, single-line assertions in flow tests.
func assertJSONFieldEquals(t *testing.T, stdout string, idx int, field string, want any) {
	t.Helper()
	rows := parseJSONArray(t, stdout)
	if idx >= len(rows) {
		t.Fatalf("row index %d out of range (len=%d), stdout=%s", idx, len(rows), stdout)
	}
	gotJSON, _ := json.Marshal(rows[idx][field])
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("rows[%d].%s = %s; want %s", idx, field, gotJSON, wantJSON)
	}
}

// assertJSONLength asserts the JSON-array stdout has the expected length.
func assertJSONLength(t *testing.T, stdout string, want int) {
	t.Helper()
	rows := parseJSONArray(t, stdout)
	if len(rows) != want {
		t.Errorf("len(rows) = %d; want %d, stdout=%s", len(rows), want, stdout)
	}
}

// jsonArrayTail extracts the trailing `[...]` JSON array from a stdout
// stream that mixes text-mode progress lines with a final JSON payload.
// Used by `plan --write --json` tests where the human-readable mutation
// progress is preserved alongside the structured tail. Returns the
// original stdout when no `[` is found so the caller's assertions still
// fail informatively.
func jsonArrayTail(stdout string) string {
	idx := strings.LastIndex(stdout, "[")
	if idx < 0 {
		return stdout
	}
	return stdout[idx:]
}
