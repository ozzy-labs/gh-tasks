package install

import (
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func TestRenderAgentsSnippet_JaSortsByName(t *testing.T) {
	t.Parallel()
	in := []skills.Skill{
		{Name: "task-plan", Description: "計画"},
		{Name: "task-add", Description: "追加"},
	}
	got := RenderAgentsSnippet(in, "ja")
	idxAdd := strings.Index(got, "task-add")
	idxPlan := strings.Index(got, "task-plan")
	if idxAdd < 0 || idxPlan < 0 || idxAdd > idxPlan {
		t.Errorf("expected task-add before task-plan; got:\n%s", got)
	}
	if !strings.HasPrefix(got, AgentsSnippetHeading) {
		t.Errorf("missing heading prefix:\n%s", got)
	}
	if !strings.Contains(got, "計画") {
		t.Errorf("ja description not used:\n%s", got)
	}
}

func TestRenderAgentsSnippet_EnUsesEnglishDescription(t *testing.T) {
	t.Parallel()
	in := []skills.Skill{
		{Name: "task-add", Description: "追加", DescriptionEN: "Add tasks"},
	}
	got := RenderAgentsSnippet(in, "en")
	if !strings.Contains(got, "Add tasks") {
		t.Errorf("en description not used:\n%s", got)
	}
	if strings.Contains(got, "追加") {
		t.Errorf("ja description leaked into en output:\n%s", got)
	}
}

func TestRenderAgentsSnippet_EmptyInput(t *testing.T) {
	t.Parallel()
	got := RenderAgentsSnippet(nil, "ja")
	want := AgentsSnippetHeading + "\n"
	if got != want {
		t.Errorf("empty: got %q, want %q", got, want)
	}
}
