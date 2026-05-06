package install

import (
	"strings"
	"testing"
)

func TestMergeMarkerBlock_EmptyExisting(t *testing.T) {
	t.Parallel()
	got := MergeMarkerBlock("", "BODY")
	if !strings.Contains(got, MarkerBeginLine) || !strings.Contains(got, MarkerEndLine) {
		t.Errorf("missing markers: %q", got)
	}
	if !strings.Contains(got, "BODY") {
		t.Errorf("body not present: %q", got)
	}
}

func TestMergeMarkerBlock_AppendsToExistingWithoutMarker(t *testing.T) {
	t.Parallel()
	existing := "# my AGENTS.md\n\nUser content here.\n"
	got := MergeMarkerBlock(existing, "BODY")
	if !strings.HasPrefix(got, "# my AGENTS.md") {
		t.Errorf("preamble not preserved: %q", got)
	}
	if !strings.Contains(got, MarkerBeginLine) {
		t.Errorf("begin marker missing: %q", got)
	}
	if !strings.Contains(got, MarkerEndLine) {
		t.Errorf("end marker missing: %q", got)
	}
	if !strings.Contains(got, "BODY") {
		t.Errorf("body missing: %q", got)
	}
	// Single blank line between consumer content and our block.
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("more than one blank-line separator: %q", got)
	}
}

func TestMergeMarkerBlock_ReplacesExistingMarker(t *testing.T) {
	t.Parallel()
	existing := "# AGENTS.md\n\n" + MarkerBeginLine + "\n\nOLD\n\n" + MarkerEndLine + "\n"
	got := MergeMarkerBlock(existing, "NEW")
	if strings.Contains(got, "OLD") {
		t.Errorf("old body leaked: %q", got)
	}
	if !strings.Contains(got, "NEW") {
		t.Errorf("new body missing: %q", got)
	}
	if !strings.HasPrefix(got, "# AGENTS.md") {
		t.Errorf("preamble lost: %q", got)
	}
}

func TestMergeMarkerBlock_PreservesContentAfterMarker(t *testing.T) {
	t.Parallel()
	existing := "PRE\n\n" + MarkerBeginLine + "\n\nOLD\n\n" + MarkerEndLine + "\nPOST\n"
	got := MergeMarkerBlock(existing, "NEW")
	if !strings.HasPrefix(got, "PRE") {
		t.Errorf("PRE lost: %q", got)
	}
	if !strings.HasSuffix(got, "POST\n") {
		t.Errorf("POST lost: %q", got)
	}
	if !strings.Contains(got, "NEW") {
		t.Errorf("NEW missing: %q", got)
	}
	if strings.Contains(got, "OLD") {
		t.Errorf("OLD leaked: %q", got)
	}
}

func TestMergeMarkerBlock_Idempotent(t *testing.T) {
	t.Parallel()
	step1 := MergeMarkerBlock("EXISTING\n", "BODY")
	step2 := MergeMarkerBlock(step1, "BODY")
	if step1 != step2 {
		t.Errorf("not idempotent\nstep1=%q\nstep2=%q", step1, step2)
	}
}

func TestHasMarkerBlock(t *testing.T) {
	t.Parallel()
	if HasMarkerBlock("") {
		t.Errorf("empty should not have marker")
	}
	if HasMarkerBlock("only " + MarkerBeginLine + " no end") {
		t.Errorf("begin-only should not be considered complete")
	}
	good := MarkerBeginLine + "\n\nBODY\n\n" + MarkerEndLine + "\n"
	if !HasMarkerBlock(good) {
		t.Errorf("complete block not detected: %q", good)
	}
}
