// Internal-package tests for the unexported helpers that back
// `gh tasks projects init`. The flow / cobra-rooted scenarios live in
// cmd_flow_test.go (package cmd_test); this file targets the four pure
// helpers (loadTemplateRaw, parseTemplateBytes, templateTypeToDataType,
// resolveOwnerID) plus the genqlient-interface type-switch
// projectV2FieldDescriptor — none of which are reachable from external
// tests because they accept generated wrapper types unexported by the
// queries package only via fully-qualified names.
package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
)

// TestProjectV2FieldDescriptor exercises the 3 concrete genqlient subtypes
// that satisfy ProjectV2FieldConfiguration plus the nil + unset cases. The
// type-switch is `0.0%` until something staged like this calls it directly.
func TestProjectV2FieldDescriptor(t *testing.T) {
	t.Parallel()

	common := &queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2Field{
		Name:     "Status",
		DataType: "TEXT",
	}
	iter := &queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2FieldProjectV2IterationField{
		Name:     "Iteration",
		DataType: "ITERATION",
	}
	single := &queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2FieldProjectV2SingleSelectField{
		Name:     "Priority",
		DataType: "SINGLE_SELECT",
	}

	asIface := func(v queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2FieldProjectV2FieldConfiguration) *queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2FieldProjectV2FieldConfiguration {
		return &v
	}

	cases := []struct {
		name         string
		in           *queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2FieldProjectV2FieldConfiguration
		wantName     string
		wantDataType string
	}{
		{name: "nil-pointer", in: nil, wantName: "", wantDataType: ""},
		{name: "common-field", in: asIface(common), wantName: "Status", wantDataType: "TEXT"},
		{name: "iteration-field", in: asIface(iter), wantName: "Iteration", wantDataType: "ITERATION"},
		{name: "single-select-field", in: asIface(single), wantName: "Priority", wantDataType: "SINGLE_SELECT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotName, gotData := projectV2FieldDescriptor(tc.in)
			if gotName != tc.wantName || gotData != tc.wantDataType {
				t.Errorf("projectV2FieldDescriptor() = (%q, %q), want (%q, %q)",
					gotName, gotData, tc.wantName, tc.wantDataType)
			}
		})
	}

	t.Run("unhandled-default-branch", func(t *testing.T) {
		t.Parallel()
		// An iface holding a nil concrete pointer is not handled by any of
		// the three case arms (the type-switch matches *T even when T is
		// nil, so we can't easily reach the trailing `return "", ""` from
		// a typed-nil; the only realistic path is the leading `if v ==
		// nil` guard, which is already exercised above. We still call the
		// switch with each non-nil concrete to lock in the descriptor
		// values pinned in the table-driven cases above).
		var iface queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2FieldProjectV2FieldConfiguration = common
		gotName, gotData := projectV2FieldDescriptor(&iface)
		if gotName != "Status" || gotData != "TEXT" {
			t.Errorf("smoke check on common: got (%q, %q)", gotName, gotData)
		}
	})
}

// TestLoadTemplateRaw covers the three paths in loadTemplateRaw:
//   - --template user → bundledUserTemplate
//   - --template org  → bundledOrgTemplate
//   - YAML path read  → file contents (and IO error on missing path)
//   - both empty      → "no source" sentinel
func TestLoadTemplateRaw(t *testing.T) {
	t.Parallel()

	t.Run("template-user-returns-bundled", func(t *testing.T) {
		t.Parallel()
		raw, src, err := loadTemplateRaw("", "user")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if src != "--template user" {
			t.Errorf("source = %q, want --template user", src)
		}
		if !strings.Contains(string(raw), "gh-tasks user scope") {
			t.Errorf("expected bundled user template marker, got:\n%s", raw)
		}
	})

	t.Run("template-org-returns-bundled", func(t *testing.T) {
		t.Parallel()
		raw, src, err := loadTemplateRaw("", "org")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if src != "--template org" {
			t.Errorf("source = %q, want --template org", src)
		}
		if !strings.Contains(string(raw), "gh-tasks org scope") {
			t.Errorf("expected bundled org template marker, got:\n%s", raw)
		}
	})

	t.Run("yaml-path-reads-file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		p := filepath.Join(dir, "fields.yaml")
		body := "fields:\n  - name: F\n    type: text\n"
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		raw, src, err := loadTemplateRaw(p, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(raw) != body {
			t.Errorf("raw = %q, want %q", raw, body)
		}
		// path is filepath.Clean'd; on Linux that's the same path, but
		// don't pin the exact bytes — only the substring.
		if !strings.Contains(src, "fields.yaml") {
			t.Errorf("source = %q, want to contain fields.yaml", src)
		}
	})

	t.Run("yaml-path-missing-returns-error", func(t *testing.T) {
		t.Parallel()
		_, src, err := loadTemplateRaw(filepath.Join(t.TempDir(), "does-not-exist.yaml"), "")
		if err == nil {
			t.Fatalf("expected error for missing file")
		}
		if !strings.Contains(src, "does-not-exist.yaml") {
			t.Errorf("source on error = %q, want to surface the path", src)
		}
	})

	t.Run("both-empty-no-source", func(t *testing.T) {
		t.Parallel()
		_, _, err := loadTemplateRaw("", "")
		if err == nil {
			t.Fatalf("expected error when both inputs are empty")
		}
		if !strings.Contains(err.Error(), "no source") {
			t.Errorf("error = %q, want to mention 'no source'", err.Error())
		}
	})
}

// TestTemplateTypeToDataType pins each branch of the YAML-type → genqlient
// dataType mapping, including the deliberately-unreachable `repository`
// case (callers strip those before invocation, but the internal contract
// still surfaces a clear error rather than a bogus data type).
func TestTemplateTypeToDataType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "text", want: "TEXT"},
		{in: "number", want: "NUMBER"},
		{in: "date", want: "DATE"},
		{in: "single_select", want: "SINGLE_SELECT"},
		{in: "iteration", want: "ITERATION"},
		{in: "repository", wantErr: true},
		{in: "unknown", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := templateTypeToDataType(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Errorf("templateTypeToDataType(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("templateTypeToDataType(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestParseTemplateBytes exercises the YAML parse + validation rules:
// every field needs name + type, only the documented type set is accepted,
// and single_select demands a non-empty options list. Each error path
// mentions enough context to be actionable in the CLI error message.
func TestParseTemplateBytes(t *testing.T) {
	t.Parallel()

	t.Run("valid-template", func(t *testing.T) {
		t.Parallel()
		body := `fields:
  - name: Status
    type: single_select
    options:
      - Triage
      - Done
  - name: Iteration
    type: iteration
`
		got, err := parseTemplateBytes([]byte(body))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Fields) != 2 {
			t.Fatalf("expected 2 fields, got %d", len(got.Fields))
		}
		if got.Fields[0].Name != "Status" || got.Fields[0].Type != "single_select" {
			t.Errorf("fields[0] = %+v", got.Fields[0])
		}
		if len(got.Fields[0].Options) != 2 {
			t.Errorf("fields[0].Options = %v, want 2 entries", got.Fields[0].Options)
		}
	})

	t.Run("invalid-yaml", func(t *testing.T) {
		t.Parallel()
		_, err := parseTemplateBytes([]byte("fields: ["))
		if err == nil {
			t.Fatalf("expected error for malformed YAML")
		}
	})

	t.Run("missing-name", func(t *testing.T) {
		t.Parallel()
		_, err := parseTemplateBytes([]byte("fields:\n  - type: text\n"))
		if err == nil || !strings.Contains(err.Error(), "name and type") {
			t.Errorf("expected 'name and type' error, got %v", err)
		}
	})

	t.Run("missing-type", func(t *testing.T) {
		t.Parallel()
		_, err := parseTemplateBytes([]byte("fields:\n  - name: F\n"))
		if err == nil || !strings.Contains(err.Error(), "name and type") {
			t.Errorf("expected 'name and type' error, got %v", err)
		}
	})

	t.Run("unsupported-type", func(t *testing.T) {
		t.Parallel()
		_, err := parseTemplateBytes([]byte("fields:\n  - name: F\n    type: bogus\n"))
		if err == nil || !strings.Contains(err.Error(), "unsupported field type") {
			t.Errorf("expected 'unsupported field type' error, got %v", err)
		}
	})

	t.Run("single-select-missing-options", func(t *testing.T) {
		t.Parallel()
		_, err := parseTemplateBytes([]byte("fields:\n  - name: F\n    type: single_select\n"))
		if err == nil || !strings.Contains(err.Error(), "options") {
			t.Errorf("expected options error, got %v", err)
		}
	})

	t.Run("repository-type-allowed", func(t *testing.T) {
		t.Parallel()
		// `repository` parses fine; runProjectsInit strips it before
		// toFieldInput is called.
		got, err := parseTemplateBytes([]byte("fields:\n  - name: Repository\n    type: repository\n"))
		if err != nil {
			t.Fatalf("repository parse error: %v", err)
		}
		if len(got.Fields) != 1 || got.Fields[0].Type != "repository" {
			t.Errorf("unexpected fields: %+v", got.Fields)
		}
	})
}

// stubGraphQL is a tiny inline GraphQL fake reused only by
// TestResolveOwnerID. It returns the supplied response unmarshalled into
// the genqlient-shaped `out`, or the supplied error.
type stubGraphQL struct {
	respond func(query string, out any) error
}

func (s *stubGraphQL) Do(_ context.Context, query string, _ map[string]any, out any) error {
	return s.respond(query, out)
}

// TestResolveOwnerID covers the three paths of resolveOwnerID:
//   - "@me" → GetViewerID happy + error
//   - login → GetOwnerID happy + nil RepositoryOwner (login not found) + error
func TestResolveOwnerID(t *testing.T) {
	t.Parallel()

	t.Run("at-me-returns-viewer-id", func(t *testing.T) {
		t.Parallel()
		gql := &stubGraphQL{respond: func(query string, out any) error {
			if !strings.Contains(query, "GetViewerID") {
				t.Fatalf("expected GetViewerID query, got: %s", query)
			}
			r := out.(*queries.GetViewerIDResponse)
			r.Viewer = &queries.GetViewerIDViewerUser{Id: "U_viewer", Login: "me"}
			return nil
		}}
		got, err := resolveOwnerID(context.Background(), gql, "@me")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "U_viewer" {
			t.Errorf("ownerID = %q, want U_viewer", got)
		}
	})

	t.Run("at-me-error-propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("graphql boom")
		gql := &stubGraphQL{respond: func(_ string, _ any) error { return sentinel }}
		_, err := resolveOwnerID(context.Background(), gql, "@me")
		if !errors.Is(err, sentinel) {
			t.Errorf("error = %v, want %v", err, sentinel)
		}
	})

	t.Run("login-resolves-via-repository-owner", func(t *testing.T) {
		t.Parallel()
		gql := &stubGraphQL{respond: func(query string, out any) error {
			if !strings.Contains(query, "GetOwnerID") {
				t.Fatalf("expected GetOwnerID query, got: %s", query)
			}
			// Mirror the genqlient unmarshal path: populate via the
			// concrete User type, then box into the iface pointer.
			user := &queries.GetOwnerIDRepositoryOwnerUser{Id: "U_other", Login: "octocat"}
			var iface queries.GetOwnerIDRepositoryOwner = user
			r := out.(*queries.GetOwnerIDResponse)
			r.RepositoryOwner = &iface
			return nil
		}}
		got, err := resolveOwnerID(context.Background(), gql, "octocat")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "U_other" {
			t.Errorf("ownerID = %q, want U_other", got)
		}
	})

	t.Run("login-not-found-returns-empty-string", func(t *testing.T) {
		t.Parallel()
		gql := &stubGraphQL{respond: func(_ string, out any) error {
			r := out.(*queries.GetOwnerIDResponse)
			r.RepositoryOwner = nil
			return nil
		}}
		got, err := resolveOwnerID(context.Background(), gql, "ghost")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("ownerID = %q, want empty (unknown owner)", got)
		}
	})

	t.Run("login-error-propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("graphql owner boom")
		gql := &stubGraphQL{respond: func(_ string, _ any) error { return sentinel }}
		_, err := resolveOwnerID(context.Background(), gql, "octocat")
		if !errors.Is(err, sentinel) {
			t.Errorf("error = %v, want %v", err, sentinel)
		}
	})
}
