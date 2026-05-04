package skills_test

import (
	"os"
	"path/filepath"
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

func TestLoadGuards(t *testing.T) {
	t.Parallel()

	cases := []struct {
		label       string
		dirName     string
		frontmatter string
		wantSubstr  string
	}{
		{
			label:   "directory-name-mismatch",
			dirName: "alpha",
			frontmatter: "---\n" +
				"name: beta\n" +
				"description: d\n" +
				"locale: ja\n" +
				"---\nbody\n",
			wantSubstr: `frontmatter name="beta"`,
		},
		{
			label:   "locale-not-ja",
			dirName: "alpha",
			frontmatter: "---\n" +
				"name: alpha\n" +
				"description: d\n" +
				"locale: en\n" +
				"---\nbody\n",
			wantSubstr: `locale="en"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			t.Parallel()
			srcDir := t.TempDir()
			skillDir := filepath.Join(srcDir, tc.dirName)
			if err := os.MkdirAll(skillDir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			path := filepath.Join(skillDir, "SKILL.md")
			if err := os.WriteFile(path, []byte(tc.frontmatter), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}

			// Use a minimal Required set so the missing-fields gate does not
			// fire before the directory-name / locale guards under test.
			_, err := skills.Load(srcDir, skills.LoadOptions{
				Required: []string{"name", "locale"},
			})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}
