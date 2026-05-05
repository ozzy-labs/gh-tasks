package adapters

// Internal tests for sortByName, the Unicode-aware collator used to
// produce deterministic adapter output ordering. The helper is unexported
// so this file lives in `package adapters` (not `_test`); the rest of
// adapters_test.go uses the external test package, but a sort helper
// regression would be invisible from there.

import (
	"sort"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func names(in []skills.Skill) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = s.Name
	}
	return out
}

func mkSkills(names ...string) []skills.Skill {
	out := make([]skills.Skill, len(names))
	for i, n := range names {
		out[i] = skills.Skill{Name: n}
	}
	return out
}

func TestSortByName_AsciiAlphabetical(t *testing.T) {
	t.Parallel()

	// Pin the basic contract: pure ASCII names sort lexicographically. Any
	// locale tweak that broke this baseline would surface here first.
	in := mkSkills("zebra", "apple", "mango", "banana")
	got := names(sortByName(in))
	want := []string{"apple", "banana", "mango", "zebra"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}

func TestSortByName_CaseInsensitiveOrdering(t *testing.T) {
	t.Parallel()

	// language.English collation is case-insensitive at the primary level.
	// Pin that "Apple" / "apple" / "APPLE" all sort together rather than
	// separating into upper-block / lower-block ASCII cohorts (which would
	// happen with bytewise `<`).
	in := mkSkills("Banana", "apple", "ZEBRA", "Apple")
	got := names(sortByName(in))
	if got[0] != "apple" && got[0] != "Apple" {
		t.Errorf("apple variants must lead, got %v", got)
	}
	if got[len(got)-1] != "ZEBRA" {
		t.Errorf("zebra must be last, got %v", got)
	}
	// "apple" / "Apple" must be adjacent — collation groups equal-fold
	// strings, and SliceStable preserves their relative order.
	leadIsApple := (got[0] == "apple" && got[1] == "Apple") ||
		(got[0] == "Apple" && got[1] == "apple")
	if !leadIsApple {
		t.Errorf("apple variants must be adjacent, got %v", got)
	}
}

func TestSortByName_StableForEqualKeys(t *testing.T) {
	t.Parallel()

	// SliceStable contract: equal-keyed entries keep their input order.
	// We can't observe stability with Name alone (collation maps "apple"
	// and "Apple" to the same primary weight, but they're still distinct
	// strings), so we pair Name with a marker field via Description and
	// assert the marker order for case-equivalent names.
	in := []skills.Skill{
		{Name: "apple", Description: "first"},
		{Name: "Apple", Description: "second"},
		{Name: "APPLE", Description: "third"},
	}
	got := sortByName(in)
	if got[0].Description != "first" ||
		got[1].Description != "second" ||
		got[2].Description != "third" {
		t.Errorf("stable order broken: got %+v", got)
	}
}

func TestSortByName_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	// The helper allocates a copy and operates on it. A regression that
	// switched to in-place sort.Slice on `in` would corrupt the caller's
	// slice — pin that the input survives unchanged.
	in := mkSkills("zebra", "apple")
	original := append([]skills.Skill(nil), in...)
	_ = sortByName(in)
	for i := range original {
		if in[i].Name != original[i].Name {
			t.Errorf("input mutated at %d: got %q, want %q",
				i, in[i].Name, original[i].Name)
		}
	}
}

func TestSortByName_EmptyAndSingleton(t *testing.T) {
	t.Parallel()

	if got := sortByName(nil); len(got) != 0 {
		t.Errorf("nil input: expected empty, got %v", names(got))
	}
	if got := sortByName([]skills.Skill{}); len(got) != 0 {
		t.Errorf("empty input: expected empty, got %v", names(got))
	}
	one := mkSkills("solo")
	got := sortByName(one)
	if len(got) != 1 || got[0].Name != "solo" {
		t.Errorf("singleton: got %v", names(got))
	}
}

func TestSortByName_HyphensAndUnderscores(t *testing.T) {
	t.Parallel()

	// Real skill names use both hyphens and underscores ("task-add",
	// "build_skills"). Pin that adjacent-variant names sort consistently
	// so a future refactor that switches separators on a few skills doesn't
	// silently scramble the AGENTS.md snippet.
	in := mkSkills("task-add", "task-plan", "task-review", "task-link-pr")
	got := names(sortByName(in))
	if !sort.StringsAreSorted(got) {
		t.Errorf("expected ASCII-sorted (collator preserves hyphen order), got %v", got)
	}
}

func TestSortByName_MixedASCIIAndNonASCII(t *testing.T) {
	t.Parallel()

	// Pin determinism for mixed scripts. We don't pin the exact relative
	// position of "日本語" vs ASCII because collation locale (English)
	// places it after Latin, but two consecutive sortByName calls must
	// produce identical orderings — i.e. the helper is a stable, total
	// order rather than randomized.
	in := mkSkills("日本語", "alpha", "Über", "café", "zeta")
	a := names(sortByName(in))
	b := names(sortByName(in))
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("non-deterministic sort: %v vs %v", a, b)
			break
		}
	}
	// Sanity: ASCII subset must remain sorted relative to itself even when
	// non-ASCII names are interleaved.
	asciiSubset := []string{}
	for _, n := range a {
		switch n {
		case "alpha", "zeta":
			asciiSubset = append(asciiSubset, n)
		}
	}
	if len(asciiSubset) != 2 || asciiSubset[0] != "alpha" || asciiSubset[1] != "zeta" {
		t.Errorf("ASCII subset order broken: %v", asciiSubset)
	}
}
