package projectitem_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/project"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
	"github.com/ozzy-labs/gh-tasks/internal/scope"
)

// fakeGraphQL implements github.GraphQLClient. Each call is matched against
// responses keyed by query substring, in registration order. Mirrors the
// helper used in cmd/cmd_test.go.
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

func issueItem(num int, title, url string, fieldValues ...queries.ProjectV2ItemFieldValue) queries.ProjectV2ItemNode {
	item := queries.ProjectV2ItemNode{
		ID: "ITEM_1",
		Content: &queries.ProjectV2ItemContent{
			Typename: "Issue",
			Number:   num,
			Title:    title,
			URL:      url,
		},
	}
	item.FieldValues.Nodes = fieldValues
	return item
}

func prItem(num int, title, url string) queries.ProjectV2ItemNode {
	return queries.ProjectV2ItemNode{
		ID: "ITEM_2",
		Content: &queries.ProjectV2ItemContent{
			Typename: "PullRequest",
			Number:   num,
			Title:    title,
			URL:      url,
		},
	}
}

func draftItem(title string) queries.ProjectV2ItemNode {
	return queries.ProjectV2ItemNode{
		ID: "ITEM_3",
		Content: &queries.ProjectV2ItemContent{
			Typename: "DraftIssue",
			Title:    title,
		},
	}
}

func emptyItem() queries.ProjectV2ItemNode {
	return queries.ProjectV2ItemNode{ID: "ITEM_4"}
}

func statusValue(name string) queries.ProjectV2ItemFieldValue {
	return queries.ProjectV2ItemFieldValue{
		Typename: "ProjectV2ItemFieldSingleSelectValue",
		Name:     name,
		Field:    queries.ProjectV2FieldRef{ID: "F_status", Name: "Status"},
	}
}

func TestFindStatus(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		got := projectitem.FindStatus(nil)
		if got != "" {
			t.Errorf("got %q want empty", got)
		}
	})

	t.Run("status-found-case-insensitive-name", func(t *testing.T) {
		t.Parallel()
		v := statusValue("Done")
		v.Field.Name = "status" // lowercase field name
		got := projectitem.FindStatus([]queries.ProjectV2ItemFieldValue{v})
		if got != "Done" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-non-single-select", func(t *testing.T) {
		t.Parallel()
		v := queries.ProjectV2ItemFieldValue{
			Typename: "ProjectV2ItemFieldTextValue",
			Text:     "Note",
			Field:    queries.ProjectV2FieldRef{ID: "F_status", Name: "Status"},
		}
		got := projectitem.FindStatus([]queries.ProjectV2ItemFieldValue{v})
		if got != "" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-iteration-value-on-status-field", func(t *testing.T) {
		t.Parallel()
		// A field literally named "Status" but typed as Iteration must not
		// be picked up — FindStatus must guard on Typename, not just Name.
		v := queries.ProjectV2ItemFieldValue{
			Typename:  "ProjectV2ItemFieldIterationValue",
			Title:     "Sprint 12",
			StartDate: "2026-05-04",
			Field:     queries.ProjectV2FieldRef{ID: "F_status", Name: "Status"},
		}
		got := projectitem.FindStatus([]queries.ProjectV2ItemFieldValue{v})
		if got != "" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-date-value-on-status-field", func(t *testing.T) {
		t.Parallel()
		v := queries.ProjectV2ItemFieldValue{
			Typename: "ProjectV2ItemFieldDateValue",
			Date:     "2026-05-04",
			Field:    queries.ProjectV2FieldRef{ID: "F_status", Name: "Status"},
		}
		got := projectitem.FindStatus([]queries.ProjectV2ItemFieldValue{v})
		if got != "" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-text-value-on-status-field", func(t *testing.T) {
		t.Parallel()
		v := queries.ProjectV2ItemFieldValue{
			Typename: "ProjectV2ItemFieldTextValue",
			Text:     "In Progress",
			Field:    queries.ProjectV2FieldRef{ID: "F_status", Name: "Status"},
		}
		got := projectitem.FindStatus([]queries.ProjectV2ItemFieldValue{v})
		if got != "" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-non-status-fields", func(t *testing.T) {
		t.Parallel()
		v := statusValue("High")
		v.Field.Name = "Priority"
		got := projectitem.FindStatus([]queries.ProjectV2ItemFieldValue{v})
		if got != "" {
			t.Errorf("got %q", got)
		}
	})
}

func TestFormatItem(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		item queries.ProjectV2ItemNode
		want string
	}{
		{
			name: "issue-no-status",
			item: issueItem(42, "Fix login", "https://example.com/i/42"),
			want: "#42  Fix login\n  https://example.com/i/42\n",
		},
		{
			name: "issue-with-status",
			item: issueItem(42, "Fix login", "https://example.com/i/42", statusValue("In Progress")),
			want: "#42  Fix login  [In Progress]\n  https://example.com/i/42\n",
		},
		{
			name: "pr",
			item: prItem(7, "Add cache", "https://example.com/p/7"),
			want: "PR#7  Add cache\n  https://example.com/p/7\n",
		},
		{
			name: "pr-with-status",
			item: func() queries.ProjectV2ItemNode {
				it := prItem(7, "Add cache", "https://example.com/p/7")
				it.FieldValues.Nodes = []queries.ProjectV2ItemFieldValue{statusValue("In Review")}
				return it
			}(),
			want: "PR#7  Add cache  [In Review]\n  https://example.com/p/7\n",
		},
		{
			name: "draft-no-status",
			item: draftItem("Plan onboarding"),
			want: "(draft)  Plan onboarding\n",
		},
		{
			name: "draft-with-status",
			item: func() queries.ProjectV2ItemNode {
				it := draftItem("Plan onboarding")
				it.FieldValues.Nodes = []queries.ProjectV2ItemFieldValue{statusValue("Backlog")}
				return it
			}(),
			want: "(draft)  Plan onboarding  [Backlog]\n",
		},
		{
			name: "no-content-with-status",
			item: func() queries.ProjectV2ItemNode {
				it := emptyItem()
				it.FieldValues.Nodes = []queries.ProjectV2ItemFieldValue{statusValue("Backlog")}
				return it
			}(),
			want: "(no content)  [Backlog]\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := projectitem.FormatItem(tc.item)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("FormatItem (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFormatItemLineCompact(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		item queries.ProjectV2ItemNode
		want string
	}{
		{
			name: "issue",
			item: issueItem(42, "Fix login", "https://example.com/i/42", statusValue("Done")),
			want: "#42 Fix login (https://example.com/i/42)",
		},
		{
			name: "pr",
			item: prItem(7, "Add cache", "https://example.com/p/7"),
			want: "PR#7 Add cache (https://example.com/p/7)",
		},
		{
			name: "draft",
			item: draftItem("Plan onboarding"),
			want: "(draft) Plan onboarding",
		},
		{
			name: "no-content",
			item: emptyItem(),
			want: "(no content)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := projectitem.FormatItemLineCompact(tc.item)
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestResolveProjectNodeID(t *testing.T) {
	t.Parallel()

	ref := project.Ref{Owner: "acme", Number: 7}

	t.Run("org-success", func(t *testing.T) {
		t.Parallel()
		g := &fakeGraphQL{responses: []fakeResponse{
			{
				matchSubstring: "GetOrgProjectV2",
				data: map[string]any{
					"organization": map[string]any{
						"projectV2": map[string]any{"id": "PVT_org", "number": 7, "title": "Roadmap"},
					},
				},
			},
		}}
		id, err := projectitem.ResolveProjectNodeID(context.Background(), g, scope.Org, ref)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if id != "PVT_org" {
			t.Errorf("got id %q want %q", id, "PVT_org")
		}
	})

	t.Run("user-success", func(t *testing.T) {
		t.Parallel()
		g := &fakeGraphQL{responses: []fakeResponse{
			{
				matchSubstring: "GetUserProjectV2",
				data: map[string]any{
					"user": map[string]any{
						"projectV2": map[string]any{"id": "PVT_user", "number": 7, "title": "Personal"},
					},
				},
			},
		}}
		id, err := projectitem.ResolveProjectNodeID(context.Background(), g, scope.User, ref)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if id != "PVT_user" {
			t.Errorf("got id %q want %q", id, "PVT_user")
		}
	})

	t.Run("org-not-found-nil-organization", func(t *testing.T) {
		t.Parallel()
		g := &fakeGraphQL{responses: []fakeResponse{
			{
				matchSubstring: "GetOrgProjectV2",
				data:           map[string]any{"organization": nil},
			},
		}}
		id, err := projectitem.ResolveProjectNodeID(context.Background(), g, scope.Org, ref)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if id != "" {
			t.Errorf("got id %q, want empty when project not found", id)
		}
	})

	t.Run("org-not-found-nil-project", func(t *testing.T) {
		t.Parallel()
		g := &fakeGraphQL{responses: []fakeResponse{
			{
				matchSubstring: "GetOrgProjectV2",
				data: map[string]any{
					"organization": map[string]any{"projectV2": nil},
				},
			},
		}}
		id, err := projectitem.ResolveProjectNodeID(context.Background(), g, scope.Org, ref)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if id != "" {
			t.Errorf("got id %q, want empty when project not found", id)
		}
	})

	t.Run("user-not-found-nil-user", func(t *testing.T) {
		t.Parallel()
		g := &fakeGraphQL{responses: []fakeResponse{
			{
				matchSubstring: "GetUserProjectV2",
				data:           map[string]any{"user": nil},
			},
		}}
		id, err := projectitem.ResolveProjectNodeID(context.Background(), g, scope.User, ref)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if id != "" {
			t.Errorf("got id %q, want empty when user not found", id)
		}
	})

	t.Run("user-not-found-nil-project", func(t *testing.T) {
		t.Parallel()
		g := &fakeGraphQL{responses: []fakeResponse{
			{
				matchSubstring: "GetUserProjectV2",
				data: map[string]any{
					"user": map[string]any{"projectV2": nil},
				},
			},
		}}
		id, err := projectitem.ResolveProjectNodeID(context.Background(), g, scope.User, ref)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if id != "" {
			t.Errorf("got id %q, want empty when project not found", id)
		}
	})

	t.Run("repo-scope-error", func(t *testing.T) {
		t.Parallel()
		g := &fakeGraphQL{} // must not be called
		id, err := projectitem.ResolveProjectNodeID(context.Background(), g, scope.Repo, ref)
		if err == nil {
			t.Fatalf("expected error for repo scope, got id %q", id)
		}
		if id != "" {
			t.Errorf("got id %q, want empty on error", id)
		}
		var se *scope.ScopeError
		if !errors.As(err, &se) {
			t.Fatalf("expected scope.ScopeError, got %T: %v", err, err)
		}
		if se.I18nKey() != "error.scope.invalidForProjectResolution" {
			t.Errorf("got key %q, want %q", se.I18nKey(), "error.scope.invalidForProjectResolution")
		}
	})

	t.Run("org-graphql-transport-error", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("network down")
		g := &fakeGraphQL{responses: []fakeResponse{
			{matchSubstring: "GetOrgProjectV2", err: boom},
		}}
		id, err := projectitem.ResolveProjectNodeID(context.Background(), g, scope.Org, ref)
		if err == nil {
			t.Fatalf("expected error, got id %q", id)
		}
		if id != "" {
			t.Errorf("got id %q, want empty on error", id)
		}
		if !errors.Is(err, boom) {
			t.Errorf("expected wrapped %v, got %v", boom, err)
		}
		if !strings.Contains(err.Error(), "get org project") {
			t.Errorf("expected wrap prefix in %q", err.Error())
		}
	})

	t.Run("user-graphql-transport-error", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("rate limited")
		g := &fakeGraphQL{responses: []fakeResponse{
			{matchSubstring: "GetUserProjectV2", err: boom},
		}}
		id, err := projectitem.ResolveProjectNodeID(context.Background(), g, scope.User, ref)
		if err == nil {
			t.Fatalf("expected error, got id %q", id)
		}
		if id != "" {
			t.Errorf("got id %q, want empty on error", id)
		}
		if !errors.Is(err, boom) {
			t.Errorf("expected wrapped %v, got %v", boom, err)
		}
		if !strings.Contains(err.Error(), "get user project") {
			t.Errorf("expected wrap prefix in %q", err.Error())
		}
	})
}
