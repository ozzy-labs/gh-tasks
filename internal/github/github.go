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

// Error satisfies the error interface.
func (e *AuthError) Error() string { return e.Key }

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

// GraphQLClient is the minimal surface used by domain packages. The
// production implementation uses cli/go-gh; tests inject a fake.
type GraphQLClient interface {
	Do(ctx context.Context, query string, vars map[string]any, out any) error
}

// RESTClient is the minimal surface for REST endpoints not covered by
// GraphQL (e.g. POST /repos/{owner}/{repo}/milestones).
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
}

// NewClients constructs both clients with auth resolved through go-gh.
func NewClients(opts ClientOptions) (*Clients, error) {
	host := opts.Host
	if host == "" {
		host, _ = auth.DefaultHost()
	}
	if host == "" {
		host = "github.com"
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
		return fmt.Errorf("graphql request: %w", err)
	}
	return nil
}

type restAdapter struct{ c *api.RESTClient }

func (a *restAdapter) Do(ctx context.Context, method, path string, body any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
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
