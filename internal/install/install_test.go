package install

import (
	"strings"
	"testing"
)

func TestValidateAgent(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		want   Agent
		wantOK bool
	}{
		"claude-code": {AgentClaudeCode, true},
		"codex-cli":   {AgentCodexCLI, true},
		"gemini-cli":  {AgentGeminiCLI, true},
		"copilot":     {AgentCopilot, true},
		"":            {"", false},
		"unknown":     {"", false},
		"Claude-Code": {"", false}, // case-sensitive
	}
	for in, tc := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			got, ok := ValidateAgent(in)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ValidateAgent(%q) = (%q, %v), want (%q, %v)",
					in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestActionType_String(t *testing.T) {
	t.Parallel()
	cases := map[ActionType]string{
		ActionCreate:   "create",
		ActionUpdate:   "update",
		ActionSkip:     "skip",
		ActionConflict: "conflict",
		ActionType(99): "unknown",
	}
	for in, want := range cases {
		if got := in.String(); got != want {
			t.Errorf("ActionType(%d).String() = %q, want %q", int(in), got, want)
		}
	}
}

func TestAdapters_PR2_OnlyClaudeCode(t *testing.T) {
	// PR 2 ships claude-code only; codex/gemini/copilot are landed in
	// PR 3-5. This test pins that contract so we notice when the
	// follow-up PRs add registrations (the test will need updating).
	t.Parallel()
	got := Adapters()
	if len(got) != 1 {
		var names []string
		for _, a := range got {
			names = append(names, string(a.Agent()))
		}
		t.Fatalf("Adapters() = [%s] (len %d); PR 2 expects exactly 1 (claude-code)",
			strings.Join(names, ","), len(got))
	}
	if got[0].Agent() != AgentClaudeCode {
		t.Errorf("Adapters()[0].Agent() = %q, want %q", got[0].Agent(), AgentClaudeCode)
	}
}

func TestAdapterFor(t *testing.T) {
	t.Parallel()
	if _, ok := AdapterFor(AgentClaudeCode); !ok {
		t.Errorf("AdapterFor(claude-code) = (_, false); want (impl, true)")
	}
	// codex-cli is not yet registered in PR 2.
	if _, ok := AdapterFor(AgentCodexCLI); ok {
		t.Errorf("AdapterFor(codex-cli) = (_, true); PR 2 should NOT yet register codex-cli")
	}
}
