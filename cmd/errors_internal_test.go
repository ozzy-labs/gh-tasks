package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
)

// TestClassifyArgError_CmdArgError pins the contract that a cmd-layer
// argument error (e.g. unparseable --since) is classified as arg
// validation, ensuring main maps it to exit 2 / ErrSilentArgs.
func TestClassifyArgError_CmdArgError(t *testing.T) {
	t.Parallel()
	err := newArgError("error.standup.invalidSince", "value", "garbage")
	if !classifyArgError(err) {
		t.Fatal("expected classifyArgError to return true for *cmdArgError")
	}
}

// TestClassifyArgError_CmdRuntimeError pins the inverse: cmd-layer runtime
// errors (e.g. viewer login unresolved) are NOT arg-validation, so main
// maps them to exit 1 / ErrSilentRuntime.
func TestClassifyArgError_CmdRuntimeError(t *testing.T) {
	t.Parallel()
	err := newRuntimeError("error.standup.viewerLoginUnresolved")
	if classifyArgError(err) {
		t.Fatal("expected classifyArgError to return false for *cmdRuntimeError")
	}
}

// TestClassifyArgError_PlainErrorIsNotArg verifies that wholly unknown
// errors fall through as non-arg (preserving the existing default for
// transport / unexpected errors).
func TestClassifyArgError_PlainErrorIsNotArg(t *testing.T) {
	t.Parallel()
	if classifyArgError(errors.New("boom")) {
		t.Fatal("expected classifyArgError to return false for plain error")
	}
}

// TestCmdArgError_LocalizesEnByDefault pins the Error() rendering: the
// raw i18n key must not leak through wrap chains. We render against the
// en catalog so the assertion is locale-stable.
func TestCmdArgError_LocalizesEnByDefault(t *testing.T) {
	t.Parallel()
	err := newArgError("error.standup.invalidSince", "value", "garbage")
	if err.Error() == "error.standup.invalidSince" {
		t.Fatal("Error() returned the raw i18n key; expected en-rendered message")
	}
	if !strings.Contains(err.Error(), "garbage") {
		t.Errorf("Error() %q should embed the placeholder value", err.Error())
	}
}

// TestCmdArgError_SatisfiesLocalized verifies that the cmd-layer error
// types implement i18n.Localized so localizedError() can pick them up.
func TestCmdArgError_SatisfiesLocalized(t *testing.T) {
	t.Parallel()
	var loc i18n.Localized
	if !errors.As(newArgError("error.standup.invalidSince"), &loc) {
		t.Fatal("cmdArgError must satisfy i18n.Localized")
	}
	if !errors.As(newRuntimeError("error.standup.viewerLoginUnresolved"), &loc) {
		t.Fatal("cmdRuntimeError must satisfy i18n.Localized")
	}
}
