package skills_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func TestParseDocument(t *testing.T) {
	t.Parallel()

	t.Run("happy", func(t *testing.T) {
		t.Parallel()
		text := "---\nname: foo\ndescription: bar\n---\nbody here\n"
		fm, body, err := skills.ParseDocument(text, "label")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		want := map[string]string{"name": "foo", "description": "bar"}
		if diff := cmp.Diff(want, fm); diff != "" {
			t.Errorf("frontmatter diff:\n%s", diff)
		}
		if body != "body here\n" {
			t.Errorf("body=%q", body)
		}
	})

	t.Run("missing-frontmatter", func(t *testing.T) {
		t.Parallel()
		_, _, err := skills.ParseDocument("no frontmatter", "label")
		if err == nil || !strings.Contains(err.Error(), "missing frontmatter") {
			t.Errorf("got err=%v", err)
		}
	})

	t.Run("ignores-blank-keys", func(t *testing.T) {
		t.Parallel()
		text := "---\n: empty-key-value\nname: ok\n---\nbody\n"
		fm, _, err := skills.ParseDocument(text, "label")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if _, has := fm[""]; has {
			t.Errorf("blank key should be skipped")
		}
		if fm["name"] != "ok" {
			t.Errorf("got %q", fm["name"])
		}
	})
}

func TestAssertRequiredFields(t *testing.T) {
	t.Parallel()

	fm := map[string]string{"name": "x"}
	if err := skills.AssertRequiredFields(fm, []string{"name"}, "label"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	err := skills.AssertRequiredFields(fm, []string{"name", "description"}, "label")
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Errorf("expected missing description, got %v", err)
	}
}
