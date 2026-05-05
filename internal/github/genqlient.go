package github

import (
	"context"
	"encoding/json"
	"fmt"

	gqlclient "github.com/Khan/genqlient/graphql"
)

// AsGenqlientClient returns a [gqlclient.Client] that funnels every
// genqlient-generated operation through [Clients.GraphQL]. Wrapping the
// interface (rather than the concrete go-gh client beneath it) lets test
// fakes intercept generated operations the same way they intercept
// hand-written ones via [GraphQLClient.Do].
//
// This adapter is the bridge for the incremental genqlient migration
// (#229 / #230 / #231): call sites are flipped to the typed generated
// functions one operation at a time without touching transport plumbing
// or test helpers.
func (c *Clients) AsGenqlientClient() gqlclient.Client {
	if c == nil {
		return nil
	}
	return &genqlientAdapter{inner: c.GraphQL}
}

// AsGenqlientClientFor wraps any [GraphQLClient] (production or fake) into
// a [gqlclient.Client] that genqlient-generated operations can consume.
// Used by domain helpers (e.g. internal/projectitem) that already accept
// a GraphQLClient injected by their caller and need to call generated
// functions without re-plumbing the [Clients] aggregate down through
// every signature. A nil-inner adapter still returns a non-nil
// [gqlclient.Client]; the wrapped MakeRequest reports the nil inner via
// a normal error rather than panicking, mirroring the behaviour expected
// by [TestGenqlientAdapter_NilInner].
func AsGenqlientClientFor(g GraphQLClient) gqlclient.Client {
	return &genqlientAdapter{inner: g}
}

// genqlientAdapter implements [gqlclient.Client] on top of the
// [GraphQLClient] surface. Variables are converted from genqlient's
// `any` shape to the `map[string]any` shape expected by go-gh; the
// response is decoded straight into `resp.Data`, which genqlient
// pre-populates with a pointer to the generated response struct.
type genqlientAdapter struct {
	inner GraphQLClient
}

// MakeRequest satisfies [gqlclient.Client]. Errors propagate verbatim so
// callers (including [errors.As] against [*GraphQLClientError]) keep
// working unchanged.
func (a *genqlientAdapter) MakeRequest(ctx context.Context, req *gqlclient.Request, resp *gqlclient.Response) error {
	if a == nil || a.inner == nil {
		return fmt.Errorf("genqlient adapter: nil graphql client")
	}
	vars, err := marshalVars(req.Variables)
	if err != nil {
		return fmt.Errorf("genqlient adapter: marshal variables: %w", err)
	}
	return a.inner.Do(ctx, req.Query, vars, resp.Data)
}

// marshalVars converts genqlient's `any` Variables into the
// `map[string]any` shape that [GraphQLClient.Do] expects. genqlient
// always passes a struct (or nil); we round-trip through JSON to
// preserve `json:"..."` tags so field names line up with the GraphQL
// variable names.
func marshalVars(v any) (map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	buf, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// `null` (e.g. a typed nil pointer) round-trips to nil so callers
	// don't pass an empty `{}` payload.
	if string(buf) == "null" {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal(buf, &out); err != nil {
		return nil, err
	}
	return out, nil
}
