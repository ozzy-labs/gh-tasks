// Package github wraps cli/go-gh/v2 to expose REST + GraphQL clients with
// gh-native auth resolution. The wrapper deliberately stays thin so commands
// can mock [GraphQLClient] / [RESTClient] in tests without dragging the
// transport in.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/auth"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

// AuthError is returned when no GitHub token can be resolved.
type AuthError struct{ i18n.Payload }

// Error satisfies the error interface. Returns the en-locale rendered
// message rather than the raw key, so wrap chains and other paths that
// bypass localizedError still surface a human-readable string.
func (e *AuthError) Error() string { return e.Localize(i18n.LocaleEN) }

// AsAuthError unwraps err into an AuthError.
func AsAuthError(err error) (*AuthError, bool) {
	var ae *AuthError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

func newAuthError(key string, args ...any) *AuthError {
	return &AuthError{Payload: i18n.NewPayload(key, args...)}
}

// GraphQLClientError wraps an underlying go-gh error with the HTTP status
// code (when available) so callers can branch on 401 / 404 / etc. without
// reaching into go-gh internals via [errors.As].
//
// Status is 0 when the underlying error is not an [*api.HTTPError] (e.g. a
// transport / network failure or a [*api.GraphQLError] without HTTP context).
// In that case callers should fall back to inspecting Cause directly.
type GraphQLClientError struct {
	Status int
	Cause  error
}

// Error satisfies the error interface.
func (e *GraphQLClientError) Error() string {
	if e == nil || e.Cause == nil {
		return "graphql request"
	}
	if e.Status != 0 {
		return fmt.Sprintf("graphql request: HTTP %d: %s", e.Status, e.Cause.Error())
	}
	return fmt.Sprintf("graphql request: %s", e.Cause.Error())
}

// Unwrap exposes Cause to [errors.Is] / [errors.As], allowing callers to
// keep using e.g. errors.As against [*api.HTTPError] when needed.
func (e *GraphQLClientError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// GraphQLClient is the minimal surface used by domain packages. The
// production implementation uses cli/go-gh; tests inject a fake.
type GraphQLClient interface {
	Do(ctx context.Context, query string, vars map[string]any, out any) error
}

// RESTClient is the minimal surface for REST endpoints not covered by
// GraphQL (e.g. POST repos/{owner}/{repo}/milestones). path MUST NOT start
// with "/" — go-gh's restPrefix already includes the trailing slash, so a
// leading "/" yields `https://api.github.com//repos/...` and HTTP 404.
type RESTClient interface {
	Do(ctx context.Context, method, path string, body any, out any) error
}

// Clients bundles the GraphQL and REST clients along with the resolved host
// (mostly for diagnostics / logging).
type Clients struct {
	Host    string
	GraphQL GraphQLClient
	REST    RESTClient
}

// ClientOptions configures NewClients.
type ClientOptions struct {
	Host     string        // override for the resolved host (defaults to auth.DefaultHost)
	Timeout  time.Duration // per-request timeout, default 30s
	Cache    bool          // enable go-gh in-memory cache
	CacheTTL time.Duration
	// CacheDir overrides the directory go-gh uses for cached API responses
	// (default: gh's own cache dir, ~/.cache/gh/api). Exposed primarily so
	// tests can redirect cache writes to a temp directory.
	CacheDir string
}

// NewClients constructs both clients with auth resolved through go-gh.
func NewClients(opts ClientOptions) (*Clients, error) {
	host := opts.Host
	if host == "" {
		// auth.DefaultHost already falls back to "github.com" when no
		// config is present, so no further fallback is needed here.
		host, _ = auth.DefaultHost()
	}
	token, _ := auth.TokenForHost(host)
	if token == "" {
		return nil, newAuthError("error.auth.tokenMissing")
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	apiOpts := api.ClientOptions{
		Host:        host,
		AuthToken:   token,
		EnableCache: opts.Cache,
		CacheTTL:    opts.CacheTTL,
		CacheDir:    opts.CacheDir,
		Timeout:     timeout,
	}
	gqlClient, err := api.NewGraphQLClient(apiOpts)
	if err != nil {
		return nil, fmt.Errorf("create graphql client: %w", err)
	}
	restClient, err := api.NewRESTClient(apiOpts)
	if err != nil {
		return nil, fmt.Errorf("create rest client: %w", err)
	}
	return &Clients{
		Host:    host,
		GraphQL: &graphqlAdapter{c: gqlClient},
		REST:    &restAdapter{c: restClient},
	}, nil
}

type graphqlAdapter struct{ c *api.GraphQLClient }

func (a *graphqlAdapter) Do(ctx context.Context, query string, vars map[string]any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := a.c.DoWithContext(ctx, query, vars, out); err != nil {
		gqlErr := &GraphQLClientError{Cause: err}
		var apiErr *api.HTTPError
		if errors.As(err, &apiErr) {
			gqlErr.Status = apiErr.StatusCode
		}
		return gqlErr
	}
	return nil
}

type restAdapter struct{ c *api.RESTClient }

func (a *restAdapter) Do(ctx context.Context, method, path string, body any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	// Skip JSON encoding entirely when no body is supplied (e.g. GET, DELETE
	// without payload). go-gh treats a nil reader as "no request body".
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	if err := a.c.DoWithContext(ctx, method, path, reader, out); err != nil {
		return fmt.Errorf("rest request: %w", err)
	}
	return nil
}
