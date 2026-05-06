package install

import (
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func TestApplyNamespace(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ns, in, want string
	}{
		// Empty namespace is a no-op.
		{"", "task-add", "task-add"},
		{"", "commit", "commit"},
		// `task-` prefix is replaced (the design example in #327).
		{"gh-tasks", "task-add", "gh-tasks-add"},
		{"gh-tasks", "task-link-pr", "gh-tasks-link-pr"},
		// Non-`task-` names get prefixed instead of mangled.
		{"gh-tasks", "commit", "gh-tasks-commit"},
		{"foo", "lint", "foo-lint"},
		// Multi-segment prefix is preserved verbatim.
		{"acme-tasks", "task-add", "acme-tasks-add"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.ns+"/"+c.in, func(t *testing.T) {
			t.Parallel()
			if got := ApplyNamespace(c.ns, c.in); got != c.want {
				t.Errorf("ApplyNamespace(%q, %q) = %q, want %q", c.ns, c.in, got, c.want)
			}
		})
	}
}

func TestRenameSkillContent_RewritesNameLine(t *testing.T) {
	t.Parallel()
	raw := "---\nname: task-add\ndescription: ja desc\nlocale: ja\n---\n\n# body\n"
	got, err := RenameSkillContent(raw, "gh-tasks-add")
	if err != nil {
		t.Fatalf("RenameSkillContent: %v", err)
	}
	if !strings.Contains(got, "name: gh-tasks-add\n") {
		t.Errorf("missing renamed name line:\n%s", got)
	}
	if strings.Contains(got, "name: task-add") {
		t.Errorf("old name line still present:\n%s", got)
	}
	// Body and other frontmatter keys must be preserved verbatim.
	if !strings.Contains(got, "description: ja desc\n") {
		t.Errorf("description line lost:\n%s", got)
	}
	if !strings.Contains(got, "# body\n") {
		t.Errorf("body lost:\n%s", got)
	}
}

func TestRenameSkillContent_TolerantOfCRLF(t *testing.T) {
	t.Parallel()
	raw := "---\r\nname: task-add\r\ndescription: x\r\n---\r\n\r\nbody\r\n"
	got, err := RenameSkillContent(raw, "ns-add")
	if err != nil {
		t.Fatalf("RenameSkillContent: %v", err)
	}
	if !strings.Contains(got, "name: ns-add") {
		t.Errorf("CRLF input not rewritten:\n%q", got)
	}
}

func TestRenameSkillContent_MissingFrontmatter(t *testing.T) {
	t.Parallel()
	if _, err := RenameSkillContent("no frontmatter here\n", "x"); err == nil {
		t.Errorf("expected error for missing frontmatter")
	}
}

func TestRenameSkillContent_MissingNameKey(t *testing.T) {
	t.Parallel()
	raw := "---\ndescription: x\nlocale: ja\n---\n\nbody\n"
	if _, err := RenameSkillContent(raw, "x"); err == nil {
		t.Errorf("expected error when frontmatter has no name key")
	}
}

func TestRenameSkillContent_OnlyTouchesFirstFrontmatter(t *testing.T) {
	// A `name:` line that appears in the body (e.g. as part of a code
	// block) must not be rewritten — only the leading frontmatter
	// region's `name:` should change.
	t.Parallel()
	raw := "---\nname: task-add\ndescription: x\n---\n\nname: task-add (in body)\n"
	got, err := RenameSkillContent(raw, "ns-add")
	if err != nil {
		t.Fatalf("RenameSkillContent: %v", err)
	}
	if !strings.Contains(got, "name: ns-add\n") {
		t.Errorf("frontmatter not rewritten:\n%s", got)
	}
	if !strings.Contains(got, "name: task-add (in body)") {
		t.Errorf("body name was rewritten (should be preserved):\n%s", got)
	}
}

func TestApplyNamespaceToSkills_NoOpForEmptyNamespace(t *testing.T) {
	t.Parallel()
	in := []skills.Skill{{Name: "task-add", Raw: "x"}}
	got, err := ApplyNamespaceToSkills("", in)
	if err != nil {
		t.Fatalf("ApplyNamespaceToSkills: %v", err)
	}
	// Identity expectation — slice header is the same when no rename.
	if len(got) != 1 || got[0].Name != "task-add" {
		t.Errorf("empty namespace mutated input: %+v", got)
	}
}

func TestApplyNamespaceToSkills_RewritesNameRawAndFrontmatter(t *testing.T) {
	t.Parallel()
	raw := "---\nname: task-add\ndescription: ja\n---\n\nbody\n"
	in := []skills.Skill{{
		Name:        "task-add",
		Raw:         raw,
		Description: "ja",
		Frontmatter: map[string]string{"name": "task-add", "description": "ja"},
	}}
	got, err := ApplyNamespaceToSkills("gh-tasks", in)
	if err != nil {
		t.Fatalf("ApplyNamespaceToSkills: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Name != "gh-tasks-add" {
		t.Errorf("Name = %q, want gh-tasks-add", got[0].Name)
	}
	if !strings.Contains(got[0].Raw, "name: gh-tasks-add\n") {
		t.Errorf("Raw not rewritten:\n%s", got[0].Raw)
	}
	if got[0].Frontmatter["name"] != "gh-tasks-add" {
		t.Errorf("Frontmatter[name] = %q, want gh-tasks-add", got[0].Frontmatter["name"])
	}
	// Caller's input must not be mutated (we shallow-copy the
	// Frontmatter map before rewriting).
	if in[0].Frontmatter["name"] != "task-add" {
		t.Errorf("input Frontmatter mutated: %+v", in[0].Frontmatter)
	}
	if in[0].Name != "task-add" {
		t.Errorf("input Name mutated: %s", in[0].Name)
	}
}

func TestApplyNamespaceToSkills_PropagatesRenameError(t *testing.T) {
	t.Parallel()
	in := []skills.Skill{{Name: "broken", Raw: "no frontmatter\n"}}
	if _, err := ApplyNamespaceToSkills("ns", in); err == nil {
		t.Errorf("expected error for raw without frontmatter")
	}
}
