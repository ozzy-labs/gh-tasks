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

// issueItem builds a minimal ProjectV2ItemNode whose union content is an
// Issue (number, title, URL). The fieldValues slice is loaded with the
// supplied entries.
func issueItem(num int, title, url string, fieldValues ...queries.ProjectV2ItemFieldValue) *queries.ProjectV2ItemNode {
	var content queries.ProjectV2ItemContent = &queries.ProjectV2ItemContentIssue{
		Id:     "I_x",
		Number: num,
		Title:  title,
		Url:    url,
	}
	item := &queries.ProjectV2ItemNode{
		Id:      "ITEM_1",
		Content: &content,
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{
			Nodes: make([]*queries.ProjectV2ItemFieldValue, 0, len(fieldValues)),
		},
	}
	for i := range fieldValues {
		v := fieldValues[i]
		item.FieldValues.Nodes = append(item.FieldValues.Nodes, &v)
	}
	return item
}

// prItem builds a minimal ProjectV2ItemNode whose union content is a
// PullRequest (number, title, URL).
func prItem(num int, title, url string) *queries.ProjectV2ItemNode {
	var content queries.ProjectV2ItemContent = &queries.ProjectV2ItemContentPullRequest{
		Id:     "P_x",
		Number: num,
		Title:  title,
		Url:    url,
	}
	return &queries.ProjectV2ItemNode{
		Id:          "ITEM_2",
		Content:     &content,
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{},
	}
}

// draftItem builds a minimal ProjectV2ItemNode whose union content is a
// DraftIssue (title only).
func draftItem(title string) *queries.ProjectV2ItemNode {
	var content queries.ProjectV2ItemContent = &queries.ProjectV2ItemContentDraftIssue{
		Id:    "DI_x",
		Title: title,
	}
	return &queries.ProjectV2ItemNode{
		Id:          "ITEM_3",
		Content:     &content,
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{},
	}
}

// emptyItem builds a ProjectV2ItemNode with no Content union resolved
// (i.e. the GraphQL response had `content: null`).
func emptyItem() *queries.ProjectV2ItemNode {
	return &queries.ProjectV2ItemNode{
		Id:          "ITEM_4",
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{},
	}
}

// statusValue builds a single-select fieldValue node naming the "Status"
// field with the supplied option name.
func statusValue(name string) queries.ProjectV2ItemFieldValue {
	field := queries.ProjectV2ItemFieldValueFieldRef(&queries.ProjectV2ItemFieldValueFieldRefProjectV2SingleSelectField{
		Id:   "F_status",
		Name: "Status",
	})
	v := &queries.ProjectV2ItemFieldValueProjectV2ItemFieldSingleSelectValue{
		Name:  &name,
		Field: field,
	}
	return queries.ProjectV2ItemFieldValue(v)
}

// dateValue builds an iteration fieldValue node naming the supplied field.
func iterationValue(fieldName, title string) queries.ProjectV2ItemFieldValue {
	field := queries.ProjectV2ItemFieldValueFieldRef(&queries.ProjectV2ItemFieldValueFieldRefProjectV2IterationField{
		Id:   "F_iter",
		Name: fieldName,
	})
	return queries.ProjectV2ItemFieldValue(&queries.ProjectV2ItemFieldValueProjectV2ItemFieldIterationValue{
		Title: title,
		Field: field,
	})
}

// textValue builds a text fieldValue node naming the supplied field.
func textValue(fieldName, text string) queries.ProjectV2ItemFieldValue {
	field := queries.ProjectV2ItemFieldValueFieldRef(&queries.ProjectV2ItemFieldValueFieldRefProjectV2Field{
		Id:   "F_text",
		Name: fieldName,
	})
	return queries.ProjectV2ItemFieldValue(&queries.ProjectV2ItemFieldValueProjectV2ItemFieldTextValue{
		Text:  &text,
		Field: field,
	})
}

// dateValue builds a date fieldValue node naming the supplied field.
func dateValue(fieldName, date string) queries.ProjectV2ItemFieldValue {
	field := queries.ProjectV2ItemFieldValueFieldRef(&queries.ProjectV2ItemFieldValueFieldRefProjectV2Field{
		Id:   "F_date",
		Name: fieldName,
	})
	return queries.ProjectV2ItemFieldValue(&queries.ProjectV2ItemFieldValueProjectV2ItemFieldDateValue{
		Date:  &date,
		Field: field,
	})
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
		// Case-insensitive match on the field name (`status` vs `Status`).
		item := issueItem(1, "x", "u/1", statusValue("Done"))
		// Replace the ref's name to lowercase to exercise EqualFold.
		(*item.FieldValues.Nodes[0]) = func() queries.ProjectV2ItemFieldValue {
			field := queries.ProjectV2ItemFieldValueFieldRef(&queries.ProjectV2ItemFieldValueFieldRefProjectV2SingleSelectField{
				Id:   "F_status",
				Name: "status", // lowercase
			})
			done := "Done"
			return queries.ProjectV2ItemFieldValue(&queries.ProjectV2ItemFieldValueProjectV2ItemFieldSingleSelectValue{
				Name:  &done,
				Field: field,
			})
		}()
		got := projectitem.FindStatus(projectitem.FieldValuesOf(item))
		if got != "Done" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-non-single-select", func(t *testing.T) {
		t.Parallel()
		// A text value on the Status field must not be picked up.
		item := issueItem(1, "x", "u/1", textValue("Status", "Note"))
		got := projectitem.FindStatus(projectitem.FieldValuesOf(item))
		if got != "" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-iteration-value-on-status-field", func(t *testing.T) {
		t.Parallel()
		// A field literally named "Status" but typed as Iteration must not
		// be picked up — FindStatus must guard on Typename, not just Name.
		item := issueItem(1, "x", "u/1", iterationValue("Status", "Sprint 12"))
		got := projectitem.FindStatus(projectitem.FieldValuesOf(item))
		if got != "" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-date-value-on-status-field", func(t *testing.T) {
		t.Parallel()
		item := issueItem(1, "x", "u/1", dateValue("Status", "2026-05-04"))
		got := projectitem.FindStatus(projectitem.FieldValuesOf(item))
		if got != "" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-text-value-on-status-field", func(t *testing.T) {
		t.Parallel()
		item := issueItem(1, "x", "u/1", textValue("Status", "In Progress"))
		got := projectitem.FindStatus(projectitem.FieldValuesOf(item))
		if got != "" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-non-status-fields", func(t *testing.T) {
		t.Parallel()
		// Single-select named "Priority" — not the Status field.
		field := queries.ProjectV2ItemFieldValueFieldRef(&queries.ProjectV2ItemFieldValueFieldRefProjectV2SingleSelectField{
			Id:   "F_priority",
			Name: "Priority",
		})
		high := "High"
		v := queries.ProjectV2ItemFieldValue(&queries.ProjectV2ItemFieldValueProjectV2ItemFieldSingleSelectValue{
			Name:  &high,
			Field: field,
		})
		item := issueItem(1, "x", "u/1", v)
		got := projectitem.FindStatus(projectitem.FieldValuesOf(item))
		if got != "" {
			t.Errorf("got %q", got)
		}
	})
}

func TestFormatItem(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		item *queries.ProjectV2ItemNode
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
			item: func() *queries.ProjectV2ItemNode {
				it := prItem(7, "Add cache", "https://example.com/p/7")
				v := statusValue("In Review")
				it.FieldValues.Nodes = []*queries.ProjectV2ItemFieldValue{&v}
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
			item: func() *queries.ProjectV2ItemNode {
				it := draftItem("Plan onboarding")
				v := statusValue("Backlog")
				it.FieldValues.Nodes = []*queries.ProjectV2ItemFieldValue{&v}
				return it
			}(),
			want: "(draft)  Plan onboarding  [Backlog]\n",
		},
		{
			name: "no-content-with-status",
			item: func() *queries.ProjectV2ItemNode {
				it := emptyItem()
				v := statusValue("Backlog")
				it.FieldValues.Nodes = []*queries.ProjectV2ItemFieldValue{&v}
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
		item *queries.ProjectV2ItemNode
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
		var pe *projectitem.ProjectItemError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *ProjectItemError, got %T: %v", err, err)
		}
		if pe.I18nKey() != "error.projectItem.getOrgProjectFailed" {
			t.Errorf("got key %q, want %q", pe.I18nKey(), "error.projectItem.getOrgProjectFailed")
		}
		if !strings.Contains(err.Error(), "Could not load org project") {
			t.Errorf("expected localized message in %q", err.Error())
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
		var pe *projectitem.ProjectItemError
		if !errors.As(err, &pe) {
			t.Fatalf("expected *ProjectItemError, got %T: %v", err, err)
		}
		if pe.I18nKey() != "error.projectItem.getUserProjectFailed" {
			t.Errorf("got key %q, want %q", pe.I18nKey(), "error.projectItem.getUserProjectFailed")
		}
		if !strings.Contains(err.Error(), "Could not load user project") {
			t.Errorf("expected localized message in %q", err.Error())
		}
	})
}

// issueItemWithAssignees builds an Issue-content item whose Assignees
// connection is populated with the supplied logins (and optionally a nil
// node, to exercise assigneeLogins's nil-skip branch).
func issueItemWithAssignees(logins []string, includeNilNode bool) *queries.ProjectV2ItemNode {
	nodes := make([]*queries.ProjectV2ItemContentAssigneeLogin, 0, len(logins)+1)
	if includeNilNode {
		nodes = append(nodes, nil)
	}
	for _, l := range logins {
		nodes = append(nodes, &queries.ProjectV2ItemContentAssigneeLogin{Login: l})
	}
	var content queries.ProjectV2ItemContent = &queries.ProjectV2ItemContentIssue{
		Id:        "I_x",
		Number:    1,
		Title:     "t",
		Url:       "u",
		Assignees: &queries.ProjectV2ItemContentAssignees{Nodes: nodes},
	}
	return &queries.ProjectV2ItemNode{
		Id:          "ITEM_A",
		Content:     &content,
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{},
	}
}

// prItemWithAssignees mirrors issueItemWithAssignees but via the PullRequest
// content variant. assigneeLogins is shared by both content branches in
// ContentOf, so this exercises the same helper from the PR call site.
func prItemWithAssignees(logins []string) *queries.ProjectV2ItemNode {
	nodes := make([]*queries.ProjectV2ItemContentAssigneeLogin, 0, len(logins))
	for _, l := range logins {
		nodes = append(nodes, &queries.ProjectV2ItemContentAssigneeLogin{Login: l})
	}
	var content queries.ProjectV2ItemContent = &queries.ProjectV2ItemContentPullRequest{
		Id:        "P_x",
		Number:    2,
		Title:     "t",
		Url:       "u",
		Assignees: &queries.ProjectV2ItemContentAssignees{Nodes: nodes},
	}
	return &queries.ProjectV2ItemNode{
		Id:          "ITEM_B",
		Content:     &content,
		FieldValues: &queries.ProjectV2ItemNodeFieldValuesProjectV2ItemFieldValueConnection{},
	}
}

// TestContentOfAssignees exercises the unexported assigneeLogins helper
// indirectly through ContentOf. It pins the {0,1,N,nil-skip} cases plus
// the nil-Assignees-pointer branch (issue without assignees connection).
func TestContentOfAssignees(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		item *queries.ProjectV2ItemNode
		want []string
	}{
		{
			name: "issue-nil-assignees-connection",
			// Plain issueItem fixture leaves Assignees == nil; assigneeLogins
			// must return nil rather than panicking.
			item: issueItem(1, "t", "u"),
			want: nil,
		},
		{
			name: "issue-zero-assignees",
			item: issueItemWithAssignees(nil, false),
			want: []string{},
		},
		{
			name: "issue-one-assignee",
			item: issueItemWithAssignees([]string{"alice"}, false),
			want: []string{"alice"},
		},
		{
			name: "issue-many-assignees-preserve-order",
			item: issueItemWithAssignees([]string{"alice", "bob", "carol"}, false),
			want: []string{"alice", "bob", "carol"},
		},
		{
			name: "issue-skips-nil-node",
			// A nil entry in the connection's Nodes slice must be skipped,
			// not dereferenced.
			item: issueItemWithAssignees([]string{"alice", "bob"}, true),
			want: []string{"alice", "bob"},
		},
		{
			name: "pr-many-assignees-preserve-order",
			item: prItemWithAssignees([]string{"x", "y", "z"}),
			want: []string{"x", "y", "z"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := projectitem.ContentOf(tc.item).Assignees
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ContentOf(...).Assignees (-want +got):\n%s", diff)
			}
		})
	}
}

// fieldCommon builds a ProjectV2FieldNode of the "Common" (plain
// ProjectV2Field) variant — the one used for TEXT / DATE / NUMBER etc.
func fieldCommon(id, name string, dt queries.ProjectV2FieldType) queries.ProjectV2FieldNode {
	return &queries.ProjectV2FieldNodeProjectV2Field{
		Id:       id,
		Name:     name,
		DataType: dt,
	}
}

// fieldSingleSelect builds a ProjectV2FieldNode of the SingleSelect variant
// with the supplied options. Pass options=nil to verify the empty-options
// branch still produces a non-nil (but empty) Options slice.
func fieldSingleSelect(id, name string, options []projectitem.FieldOption, includeNilOption bool) queries.ProjectV2FieldNode {
	opts := make([]*queries.ProjectV2FieldNodeOptionsProjectV2SingleSelectFieldOption, 0, len(options)+1)
	if includeNilOption {
		opts = append(opts, nil)
	}
	for _, o := range options {
		opts = append(opts, &queries.ProjectV2FieldNodeOptionsProjectV2SingleSelectFieldOption{
			Id: o.ID, Name: o.Name,
		})
	}
	return &queries.ProjectV2FieldNodeProjectV2SingleSelectField{
		Id:       id,
		Name:     name,
		DataType: queries.ProjectV2FieldTypeSingleSelect,
		Options:  opts,
	}
}

// fieldIteration builds a ProjectV2FieldNode of the Iteration variant.
// Pass cfg=nil to exercise the FieldsOf branch that leaves Configuration
// unset (Configuration field stays nil on the resulting FieldDescriptor).
func fieldIteration(id, name string, cfg *queries.ProjectV2IterationConfig) queries.ProjectV2FieldNode {
	return &queries.ProjectV2FieldNodeProjectV2IterationField{
		Id:            id,
		Name:          name,
		DataType:      queries.ProjectV2FieldTypeIteration,
		Configuration: cfg,
	}
}

func TestFieldsOf(t *testing.T) {
	t.Parallel()

	t.Run("nil-slice", func(t *testing.T) {
		t.Parallel()
		got := projectitem.FieldsOf(nil)
		if len(got) != 0 {
			t.Errorf("got %#v, want empty slice", got)
		}
	})

	t.Run("empty-slice", func(t *testing.T) {
		t.Parallel()
		got := projectitem.FieldsOf([]queries.ProjectV2FieldNode{})
		if len(got) != 0 {
			t.Errorf("got %#v, want empty slice", got)
		}
	})

	t.Run("skips-nil-element", func(t *testing.T) {
		t.Parallel()
		got := projectitem.FieldsOf([]queries.ProjectV2FieldNode{nil})
		if len(got) != 0 {
			t.Errorf("got %#v, want empty slice when only nil entries", got)
		}
	})

	t.Run("mixed-three-variants-preserve-order", func(t *testing.T) {
		t.Parallel()
		// Sprint 11 + Sprint 12 active, Sprint 10 completed — order matters
		// in the response and must be preserved end-to-end.
		cfg := &queries.ProjectV2IterationConfig{
			Iterations: []*queries.ProjectV2IterationOption{
				{Id: "I_11", Title: "Sprint 11", StartDate: "2026-04-21", Duration: 7},
				{Id: "I_12", Title: "Sprint 12", StartDate: "2026-04-28", Duration: 7},
			},
			CompletedIterations: []*queries.ProjectV2IterationOption{
				{Id: "I_10", Title: "Sprint 10", StartDate: "2026-04-14", Duration: 7},
			},
		}
		fields := []queries.ProjectV2FieldNode{
			fieldCommon("F_text", "Notes", queries.ProjectV2FieldTypeText),
			nil, // must be skipped, must not break ordering of the rest
			fieldSingleSelect("F_status", "Status",
				[]projectitem.FieldOption{
					{ID: "O_todo", Name: "Todo"},
					{ID: "O_done", Name: "Done"},
				},
				true, // include a nil option entry to exercise the inner skip
			),
			fieldIteration("F_iter", "Sprint", cfg),
		}
		want := []projectitem.FieldDescriptor{
			{ID: "F_text", Name: "Notes", DataType: "TEXT", Options: nil, Configuration: nil},
			{
				ID: "F_status", Name: "Status", DataType: "SINGLE_SELECT",
				Options: []projectitem.FieldOption{
					{ID: "O_todo", Name: "Todo"},
					{ID: "O_done", Name: "Done"},
				},
			},
			{
				ID: "F_iter", Name: "Sprint", DataType: "ITERATION",
				Configuration: &projectitem.IterationConfiguration{
					Iterations: []projectitem.IterationOption{
						{ID: "I_11", Title: "Sprint 11", StartDate: "2026-04-21", Duration: 7},
						{ID: "I_12", Title: "Sprint 12", StartDate: "2026-04-28", Duration: 7},
					},
					CompletedIterations: []projectitem.IterationOption{
						{ID: "I_10", Title: "Sprint 10", StartDate: "2026-04-14", Duration: 7},
					},
				},
			},
		}
		got := projectitem.FieldsOf(fields)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("FieldsOf (-want +got):\n%s", diff)
		}
	})

	t.Run("single-select-empty-options", func(t *testing.T) {
		t.Parallel()
		// FieldsOf always allocates an Options slice for SINGLE_SELECT; with
		// zero source options it must remain non-nil (callers can iterate
		// without nil-guarding) but length 0.
		fields := []queries.ProjectV2FieldNode{
			fieldSingleSelect("F_ss", "Empty", nil, false),
		}
		got := projectitem.FieldsOf(fields)
		if len(got) != 1 {
			t.Fatalf("got %d descriptors, want 1", len(got))
		}
		if got[0].Options == nil {
			t.Errorf("Options is nil, want non-nil empty slice")
		}
		if len(got[0].Options) != 0 {
			t.Errorf("Options len = %d, want 0", len(got[0].Options))
		}
	})

	t.Run("iteration-nil-configuration", func(t *testing.T) {
		t.Parallel()
		// When Configuration is nil on the source node, FieldsOf must leave
		// the descriptor's Configuration unset (zero value) and NOT call
		// iterationConfigOf with a nil pointer.
		fields := []queries.ProjectV2FieldNode{
			fieldIteration("F_iter", "Sprint", nil),
		}
		got := projectitem.FieldsOf(fields)
		want := []projectitem.FieldDescriptor{
			{ID: "F_iter", Name: "Sprint", DataType: "ITERATION", Configuration: nil},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("FieldsOf (-want +got):\n%s", diff)
		}
	})
}

// TestFieldsOfIterationConfigShape exercises iterationConfigOf branches
// (active-only, completed-only, nil-element skip) indirectly through
// FieldsOf, which is the only public entry that reaches it.
func TestFieldsOfIterationConfigShape(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  *queries.ProjectV2IterationConfig
		want *projectitem.IterationConfiguration
	}{
		{
			name: "active-only",
			cfg: &queries.ProjectV2IterationConfig{
				Iterations: []*queries.ProjectV2IterationOption{
					{Id: "I_1", Title: "S1", StartDate: "2026-01-01", Duration: 7},
					{Id: "I_2", Title: "S2", StartDate: "2026-01-08", Duration: 7},
				},
			},
			want: &projectitem.IterationConfiguration{
				Iterations: []projectitem.IterationOption{
					{ID: "I_1", Title: "S1", StartDate: "2026-01-01", Duration: 7},
					{ID: "I_2", Title: "S2", StartDate: "2026-01-08", Duration: 7},
				},
				CompletedIterations: []projectitem.IterationOption{},
			},
		},
		{
			name: "completed-only",
			cfg: &queries.ProjectV2IterationConfig{
				CompletedIterations: []*queries.ProjectV2IterationOption{
					{Id: "I_0", Title: "S0", StartDate: "2025-12-25", Duration: 7},
				},
			},
			want: &projectitem.IterationConfiguration{
				Iterations: []projectitem.IterationOption{},
				CompletedIterations: []projectitem.IterationOption{
					{ID: "I_0", Title: "S0", StartDate: "2025-12-25", Duration: 7},
				},
			},
		},
		{
			name: "both-empty",
			// Empty (but non-nil) input lists must produce empty (non-nil)
			// output lists — callers can iterate safely either way.
			cfg: &queries.ProjectV2IterationConfig{},
			want: &projectitem.IterationConfiguration{
				Iterations:          []projectitem.IterationOption{},
				CompletedIterations: []projectitem.IterationOption{},
			},
		},
		{
			name: "skips-nil-iteration-entries",
			cfg: &queries.ProjectV2IterationConfig{
				Iterations: []*queries.ProjectV2IterationOption{
					nil,
					{Id: "I_a", Title: "Sa", StartDate: "2026-02-01", Duration: 14},
				},
				CompletedIterations: []*queries.ProjectV2IterationOption{
					{Id: "I_b", Title: "Sb", StartDate: "2026-01-18", Duration: 14},
					nil,
				},
			},
			want: &projectitem.IterationConfiguration{
				Iterations: []projectitem.IterationOption{
					{ID: "I_a", Title: "Sa", StartDate: "2026-02-01", Duration: 14},
				},
				CompletedIterations: []projectitem.IterationOption{
					{ID: "I_b", Title: "Sb", StartDate: "2026-01-18", Duration: 14},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fields := []queries.ProjectV2FieldNode{
				fieldIteration("F_iter", "Sprint", tc.cfg),
			}
			got := projectitem.FieldsOf(fields)
			if len(got) != 1 {
				t.Fatalf("got %d descriptors, want 1", len(got))
			}
			if diff := cmp.Diff(tc.want, got[0].Configuration); diff != "" {
				t.Errorf("Configuration (-want +got):\n%s", diff)
			}
		})
	}
}
