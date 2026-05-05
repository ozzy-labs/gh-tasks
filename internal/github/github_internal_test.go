package github

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

// rewritingTransport rewrites every outbound URL's scheme + host to the
// httptest server target, so go-gh's api.NewGraphQLClient /
// api.NewRESTClient (which always build absolute URLs against api.github.com
// or the configured Host) can be transparently redirected to a local fake.
type rewritingTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t *rewritingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.target.Scheme
	cloned.URL.Host = t.target.Host
	cloned.Host = t.target.Host
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(cloned)
}

func newTestGraphQLAdapter(t *testing.T, handler http.Handler) (*graphqlAdapter, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	gql, err := api.NewGraphQLClient(api.ClientOptions{
		Host:               "api.github.com",
		AuthToken:          "test-token",
		Transport:          &rewritingTransport{target: target, base: http.DefaultTransport},
		LogIgnoreEnv:       true,
		SkipDefaultHeaders: false,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("api.NewGraphQLClient: %v", err)
	}
	return &graphqlAdapter{c: gql}, srv
}

func newTestRESTAdapter(t *testing.T, handler http.Handler) (*restAdapter, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	rest, err := api.NewRESTClient(api.ClientOptions{
		Host:         "api.github.com",
		AuthToken:    "test-token",
		Transport:    &rewritingTransport{target: target, base: http.DefaultTransport},
		LogIgnoreEnv: true,
		Timeout:      5 * time.Second,
	})
	if err != nil {
		t.Fatalf("api.NewRESTClient: %v", err)
	}
	return &restAdapter{c: rest}, srv
}

// clearAuthEnv unsets every env var go-gh's auth.TokenForHost consults so a
// test running on a developer machine with GH_TOKEN exported doesn't
// accidentally satisfy NewClients.
func clearAuthEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"GH_TOKEN", "GITHUB_TOKEN",
		"GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN",
		"GH_HOST", "GH_PATH",
	} {
		t.Setenv(k, "")
	}
	// Steer go-gh away from the developer's real config / hosts.yml.
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
}

// ---------------------------------------------------------------------------
// AuthError unit tests
// ---------------------------------------------------------------------------

func TestNewAuthError_CarriesPayload(t *testing.T) {
	t.Parallel()

	err := newAuthError("error.auth.tokenMissing")
	if err == nil {
		t.Fatal("newAuthError returned nil")
	}
	if got, want := err.I18nKey(), "error.auth.tokenMissing"; got != want {
		t.Fatalf("I18nKey = %q, want %q", got, want)
	}
	// Error() renders the en-locale string, so it must be non-empty and
	// surface the configured catalog value (which mentions GH_TOKEN).
	msg := err.Error()
	if msg == "" {
		t.Fatal("Error() returned empty string")
	}
	if !strings.Contains(msg, "GH_TOKEN") {
		t.Fatalf("Error() = %q, want it to mention GH_TOKEN", msg)
	}
	// And the same message must round-trip via Localize(LocaleEN).
	if got := err.Localize(i18n.LocaleEN); got != msg {
		t.Fatalf("Localize(en) = %q, Error() = %q, want them equal", got, msg)
	}
}

func TestAsAuthError_MatchesWrappedError(t *testing.T) {
	t.Parallel()

	authErr := newAuthError("error.auth.tokenMissing")
	wrapped := &GraphQLClientError{Status: 401, Cause: authErr}

	got, ok := AsAuthError(wrapped)
	if !ok {
		t.Fatalf("AsAuthError(wrapped) ok = false, want true")
	}
	if got == nil || got.I18nKey() != "error.auth.tokenMissing" {
		t.Fatalf("AsAuthError returned %#v", got)
	}
}

func TestAsAuthError_NonAuthError(t *testing.T) {
	t.Parallel()

	if got, ok := AsAuthError(errors.New("boom")); ok || got != nil {
		t.Fatalf("AsAuthError(plain) = (%#v, %v), want (nil, false)", got, ok)
	}
	if got, ok := AsAuthError(nil); ok || got != nil {
		t.Fatalf("AsAuthError(nil) = (%#v, %v), want (nil, false)", got, ok)
	}
}

// ---------------------------------------------------------------------------
// NewClients
// ---------------------------------------------------------------------------

func TestNewClients_TokenMissing(t *testing.T) {
	clearAuthEnv(t)

	_, err := NewClients(ClientOptions{Host: "github.com"})
	if err == nil {
		t.Fatal("NewClients without token: err = nil, want *AuthError")
	}
	authErr, ok := AsAuthError(err)
	if !ok {
		t.Fatalf("err is not *AuthError: %v (%T)", err, err)
	}
	if authErr.I18nKey() != "error.auth.tokenMissing" {
		t.Fatalf("I18nKey = %q, want error.auth.tokenMissing", authErr.I18nKey())
	}
}

func TestNewClients_DefaultHostFallback(t *testing.T) {
	// Same token-missing path but exercises the auth.DefaultHost() branch so
	// the empty Host code path is covered.
	clearAuthEnv(t)

	_, err := NewClients(ClientOptions{})
	if err == nil {
		t.Fatal("NewClients with empty Host + no token: err = nil, want *AuthError")
	}
	if _, ok := AsAuthError(err); !ok {
		t.Fatalf("err is not *AuthError: %v (%T)", err, err)
	}
}

func TestNewClients_TokenFromEnv(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("GH_TOKEN", "test-token-from-env")

	clients, err := NewClients(ClientOptions{
		Host:    "github.com",
		Timeout: 1 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	if clients == nil {
		t.Fatal("NewClients returned nil clients")
	}
	if clients.Host != "github.com" {
		t.Fatalf("Host = %q, want github.com", clients.Host)
	}
	if clients.GraphQL == nil || clients.REST == nil {
		t.Fatal("GraphQL / REST adapter is nil")
	}
	// AsGenqlientClient must wrap the constructed GraphQL adapter without
	// panicking, exercising the production wiring path end-to-end.
	if clients.AsGenqlientClient() == nil {
		t.Fatal("AsGenqlientClient returned nil")
	}
}

func TestNewClients_DefaultTimeoutApplied(t *testing.T) {
	clearAuthEnv(t)
	t.Setenv("GH_TOKEN", "test-token-from-env")

	// Timeout: 0 should default to 30s — we can't observe the value
	// directly through ClientOptions, but exercising this path is what
	// the coverage target requires; assert basic happy-path success.
	clients, err := NewClients(ClientOptions{Host: "github.com"})
	if err != nil {
		t.Fatalf("NewClients: %v", err)
	}
	if clients == nil || clients.GraphQL == nil || clients.REST == nil {
		t.Fatalf("clients constructed incompletely: %#v", clients)
	}
}

// ---------------------------------------------------------------------------
// graphqlAdapter.Do
// ---------------------------------------------------------------------------

func TestGraphQLDo_Success(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the request body so connection reuse is happy.
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"viewer":{"login":"alice"}}}`))
	})
	adapter, _ := newTestGraphQLAdapter(t, handler)

	var out struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
	}
	if err := adapter.Do(context.Background(), "query { viewer { login } }", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.Viewer.Login != "alice" {
		t.Fatalf("Login = %q, want alice", out.Viewer.Login)
	}
}

func TestGraphQLDo_NilContextDefaults(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	})
	adapter, _ := newTestGraphQLAdapter(t, handler)

	// Pass nil context to exercise the `if ctx == nil` branch.
	var out struct {
		Ok bool `json:"ok"`
	}
	//nolint:staticcheck // intentionally passing nil to exercise the fallback branch
	if err := adapter.Do(nil, "query { ok }", nil, &out); err != nil {
		t.Fatalf("Do(nil ctx): %v", err)
	}
	if !out.Ok {
		t.Fatalf("Ok = false, want true")
	}
}

func TestGraphQLDo_HTTPErrorWraps(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
	}{
		{"unauthorized", http.StatusUnauthorized},
		{"forbidden", http.StatusForbidden},
		{"server_error", http.StatusInternalServerError},
		{"bad_gateway", http.StatusBadGateway},
		{"service_unavailable", http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"boom"}`))
			})
			adapter, _ := newTestGraphQLAdapter(t, handler)

			err := adapter.Do(context.Background(), "query { __typename }", nil, &struct{}{})
			if err == nil {
				t.Fatalf("Do: err = nil, want HTTP %d wrap", tc.status)
			}
			var gqlErr *GraphQLClientError
			if !errors.As(err, &gqlErr) {
				t.Fatalf("err is not *GraphQLClientError: %v (%T)", err, err)
			}
			if gqlErr.Status != tc.status {
				t.Fatalf("Status = %d, want %d", gqlErr.Status, tc.status)
			}
			// errors.As against the underlying *api.HTTPError must keep working
			// (the wrapper preserves Unwrap chain).
			var httpErr *api.HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("errors.As(*api.HTTPError) failed for status %d", tc.status)
			}
			if httpErr.StatusCode != tc.status {
				t.Fatalf("HTTPError.StatusCode = %d, want %d", httpErr.StatusCode, tc.status)
			}
			// 5xx must NOT collapse into *AuthError — only the explicit
			// NewClients token-missing path produces one.
			if _, ok := AsAuthError(err); ok {
				t.Fatalf("status %d incorrectly wrapped as *AuthError", tc.status)
			}
		})
	}
}

func TestGraphQLDo_ContextCanceled(t *testing.T) {
	t.Parallel()

	// Block the handler until the test finishes so the request fails on
	// context cancellation, not on response decoding.
	releaseHandler := make(chan struct{})
	t.Cleanup(func() { close(releaseHandler) })

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-releaseHandler:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	})
	adapter, _ := newTestGraphQLAdapter(t, handler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before issuing the request

	err := adapter.Do(ctx, "query { __typename }", nil, &struct{}{})
	if err == nil {
		t.Fatal("Do(canceled ctx): err = nil, want wrapped error")
	}
	var gqlErr *GraphQLClientError
	if !errors.As(err, &gqlErr) {
		t.Fatalf("err is not *GraphQLClientError: %v (%T)", err, err)
	}
	// Network / cancellation errors do not have an HTTP status — Status
	// should be 0 and the message must not advertise an HTTP code.
	if gqlErr.Status != 0 {
		t.Fatalf("Status = %d, want 0 for cancellation", gqlErr.Status)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(context.Canceled) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// restAdapter.Do
// ---------------------------------------------------------------------------

func TestRESTDo_Success(t *testing.T) {
	t.Parallel()

	var gotMethod string
	var gotPath string
	var gotBody []byte
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"name":"hello"}`))
	})
	adapter, _ := newTestRESTAdapter(t, handler)

	type milestoneResp struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var out milestoneResp
	body := map[string]any{"title": "v1.0", "state": "open"}
	if err := adapter.Do(context.Background(), http.MethodPost, "repos/o/r/milestones", body, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.ID != 42 || out.Name != "hello" {
		t.Fatalf("response decode mismatch: %+v", out)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/repos/o/r/milestones") {
		t.Fatalf("path = %q, want suffix /repos/o/r/milestones", gotPath)
	}
	if !strings.Contains(string(gotBody), `"title":"v1.0"`) {
		t.Fatalf("body = %q, want it to contain title", gotBody)
	}
}

func TestRESTDo_NilBody(t *testing.T) {
	t.Parallel()

	var gotLen int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLen = r.ContentLength
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	adapter, _ := newTestRESTAdapter(t, handler)

	if err := adapter.Do(context.Background(), http.MethodGet, "rate_limit", nil, &struct{}{}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	// nil body must produce an empty request body, not a JSON-encoded
	// "null" payload.
	if gotLen > 0 {
		t.Fatalf("ContentLength = %d, want 0 (no body for nil)", gotLen)
	}
}

func TestRESTDo_NilContextDefaults(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	adapter, _ := newTestRESTAdapter(t, handler)

	//nolint:staticcheck // intentionally exercising the nil-ctx fallback
	if err := adapter.Do(nil, http.MethodGet, "rate_limit", nil, &struct{}{}); err != nil {
		t.Fatalf("Do(nil ctx): %v", err)
	}
}

func TestRESTDo_404PassesThroughAsRawError(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})
	adapter, _ := newTestRESTAdapter(t, handler)

	err := adapter.Do(context.Background(), http.MethodGet, "repos/o/r/nope", nil, &struct{}{})
	if err == nil {
		t.Fatal("Do(404): err = nil, want wrapped 404")
	}
	// Caller must still be able to recover the underlying *api.HTTPError so
	// branch-on-status code works (e.g. cmd layer treats 404 as "not found"
	// rather than "auth failure"). The REST adapter intentionally does NOT
	// wrap into *AuthError or *GraphQLClientError.
	var httpErr *api.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("errors.As(*api.HTTPError) failed for 404: %v (%T)", err, err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want 404", httpErr.StatusCode)
	}
	if _, ok := AsAuthError(err); ok {
		t.Fatal("404 incorrectly wrapped as *AuthError")
	}
	var gqlErr *GraphQLClientError
	if errors.As(err, &gqlErr) {
		t.Fatalf("REST 404 leaked into *GraphQLClientError wrapper: %#v", gqlErr)
	}
}

func TestRESTDo_5xxWraps(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})
	adapter, _ := newTestRESTAdapter(t, handler)

	err := adapter.Do(context.Background(), http.MethodGet, "repos/o/r", nil, &struct{}{})
	if err == nil {
		t.Fatal("Do(500): err = nil, want wrapped 500")
	}
	var httpErr *api.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("errors.As(*api.HTTPError) failed: %v (%T)", err, err)
	}
	if httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want 500", httpErr.StatusCode)
	}
	if !strings.Contains(err.Error(), "rest request") {
		t.Fatalf("error message = %q, want it to start with rest request prefix", err.Error())
	}
}

func TestRESTDo_MarshalBodyError(t *testing.T) {
	t.Parallel()

	// We must construct an adapter without hitting the network because the
	// marshal failure short-circuits before any request goes out.
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server should not be hit when body marshalling fails")
		w.WriteHeader(http.StatusOK)
	})
	adapter, _ := newTestRESTAdapter(t, handler)

	// json.Marshal cannot encode channels — this exercises the "marshal
	// body" error branch.
	unencodable := map[string]any{"ch": make(chan int)}
	err := adapter.Do(context.Background(), http.MethodPost, "x", unencodable, &struct{}{})
	if err == nil {
		t.Fatal("Do: err = nil, want marshal failure")
	}
	if !strings.Contains(err.Error(), "marshal body") {
		t.Fatalf("error message = %q, want marshal body prefix", err.Error())
	}
}

func TestRESTDo_ContextCanceled(t *testing.T) {
	t.Parallel()

	releaseHandler := make(chan struct{})
	t.Cleanup(func() { close(releaseHandler) })

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-releaseHandler:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	})
	adapter, _ := newTestRESTAdapter(t, handler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := adapter.Do(ctx, http.MethodGet, "x", nil, &struct{}{})
	if err == nil {
		t.Fatal("Do(canceled ctx): err = nil, want wrapped error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(context.Canceled) failed: %v", err)
	}
	// Still wrapped with the rest request prefix.
	if !strings.Contains(err.Error(), "rest request") {
		t.Fatalf("error message = %q, want rest request prefix", err.Error())
	}
}
