package projectitem_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
)

func issueItem(num int, title, url string, fieldValues ...queries.ProjectV2FieldValue) queries.ProjectV2ItemNode {
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

func statusValue(name string) queries.ProjectV2FieldValue {
	return queries.ProjectV2FieldValue{
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
		got := projectitem.FindStatus([]queries.ProjectV2FieldValue{v})
		if got != "Done" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-non-single-select", func(t *testing.T) {
		t.Parallel()
		v := queries.ProjectV2FieldValue{
			Typename: "ProjectV2ItemFieldTextValue",
			Text:     "Note",
			Field:    queries.ProjectV2FieldRef{ID: "F_status", Name: "Status"},
		}
		got := projectitem.FindStatus([]queries.ProjectV2FieldValue{v})
		if got != "" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("ignores-non-status-fields", func(t *testing.T) {
		t.Parallel()
		v := statusValue("High")
		v.Field.Name = "Priority"
		got := projectitem.FindStatus([]queries.ProjectV2FieldValue{v})
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
			name: "draft-no-status",
			item: draftItem("Plan onboarding"),
			want: "(draft)  Plan onboarding\n",
		},
		{
			name: "no-content-with-status",
			item: func() queries.ProjectV2ItemNode {
				it := emptyItem()
				it.FieldValues.Nodes = []queries.ProjectV2FieldValue{statusValue("Backlog")}
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
