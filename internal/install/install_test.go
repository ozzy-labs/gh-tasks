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

func TestAdapters_PR4_ThreeAdapters(t *testing.T) {
	// PR 4 ships claude-code + codex-cli + gemini-cli; copilot lands in
	// PR 5. This test pins the current registration set so that PR 5
	// deliberately updates it.
	t.Parallel()
	got := Adapters()
	if len(got) != 3 {
		var names []string
		for _, a := range got {
			names = append(names, string(a.Agent()))
		}
		t.Fatalf("Adapters() = [%s] (len %d); PR 4 expects exactly 3 (claude-code, codex-cli, gemini-cli)",
			strings.Join(names, ","), len(got))
	}
	wantOrder := []Agent{AgentClaudeCode, AgentCodexCLI, AgentGeminiCLI}
	for i, want := range wantOrder {
		if got[i].Agent() != want {
			t.Errorf("Adapters()[%d].Agent() = %q, want %q", i, got[i].Agent(), want)
		}
	}
}

func TestAdapterFor(t *testing.T) {
	t.Parallel()
	for _, a := range []Agent{AgentClaudeCode, AgentCodexCLI, AgentGeminiCLI} {
		if _, ok := AdapterFor(a); !ok {
			t.Errorf("AdapterFor(%q) = (_, false); want (impl, true) at PR 4", a)
		}
	}
	// copilot is not yet registered.
	if _, ok := AdapterFor(AgentCopilot); ok {
		t.Errorf("AdapterFor(copilot) = (_, true); PR 4 should NOT yet register copilot")
	}
}
