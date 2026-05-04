package github_test

import (
	"errors"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/ozzy-labs/gh-tasks/internal/github"
)

func TestGraphQLClientErrorUnwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")
	err := &github.GraphQLClientError{Status: 401, Cause: cause}

	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is should succeed against the wrapped cause")
	}
	if got := err.Unwrap(); !errors.Is(got, cause) {
		t.Fatalf("Unwrap = %v, want %v", got, cause)
	}
}

func TestGraphQLClientErrorAsHTTPError(t *testing.T) {
	t.Parallel()

	httpErr := &api.HTTPError{StatusCode: 404, Message: "not found"}
	err := &github.GraphQLClientError{Status: httpErr.StatusCode, Cause: httpErr}

	var got *api.HTTPError
	if !errors.As(err, &got) {
		t.Fatalf("errors.As should succeed against *api.HTTPError")
	}
	if got.StatusCode != 404 {
		t.Fatalf("StatusCode = %d, want 404", got.StatusCode)
	}
	if err.Status != 404 {
		t.Fatalf("Status = %d, want 404", err.Status)
	}
}

func TestGraphQLClientErrorMessageFormat(t *testing.T) {
	t.Parallel()

	cause := errors.New("something failed")

	withStatus := &github.GraphQLClientError{Status: 500, Cause: cause}
	if msg := withStatus.Error(); msg != "graphql request: HTTP 500: something failed" {
		t.Fatalf("Error() with status mismatch: %q", msg)
	}

	noStatus := &github.GraphQLClientError{Cause: cause}
	if msg := noStatus.Error(); msg != "graphql request: something failed" {
		t.Fatalf("Error() without status mismatch: %q", msg)
	}
}
