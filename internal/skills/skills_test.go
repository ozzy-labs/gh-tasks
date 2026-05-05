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

	t.Run("rejects-blank-keys", func(t *testing.T) {
		t.Parallel()
		// gopkg.in/yaml.v3 parses `: value` as a malformed mapping key.
		// The legacy line-based parser silently dropped the entry; the
		// stricter YAML-based parser surfaces it as a parse error so a
		// genuinely broken frontmatter cannot reach the adapter pipeline.
		text := "---\n: empty-key-value\nname: ok\n---\nbody\n"
		_, _, err := skills.ParseDocument(text, "label")
		if err == nil {
			t.Fatal("expected parse error for blank key, got nil")
		}
		if !strings.Contains(err.Error(), "frontmatter YAML parse failed") {
			t.Errorf("expected parse-failure message, got %v", err)
		}
	})

	t.Run("value-with-colon", func(t *testing.T) {
		t.Parallel()
		// Regression: the legacy parser used strings.Index(line, ":") and
		// truncated everything after the first `:` in the value, silently
		// corrupting `allowed-tools: Bash(gh:*)` to `Bash(gh`. yaml.v3
		// preserves the full scalar.
		text := "---\nallowed-tools: Bash(gh:*)\n---\nbody\n"
		fm, _, err := skills.ParseDocument(text, "label")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got, want := fm["allowed-tools"], "Bash(gh:*)"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("crlf-line-endings", func(t *testing.T) {
		t.Parallel()
		text := "---\r\nname: foo\r\n---\r\nbody\r\n"
		fm, body, err := skills.ParseDocument(text, "label")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if fm["name"] != "foo" {
			t.Errorf("got %q", fm["name"])
		}
		if !strings.Contains(body, "body") {
			t.Errorf("body=%q", body)
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
