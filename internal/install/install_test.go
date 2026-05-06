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

func TestAdapters_PR3_ClaudeCodeAndCodexCli(t *testing.T) {
	// PR 3 ships claude-code + codex-cli; gemini-cli/copilot land in
	// PR 4-5. This test pins the current registration set so that future
	// PRs deliberately update it as adapters come online.
	t.Parallel()
	got := Adapters()
	if len(got) != 2 {
		var names []string
		for _, a := range got {
			names = append(names, string(a.Agent()))
		}
		t.Fatalf("Adapters() = [%s] (len %d); PR 3 expects exactly 2 (claude-code, codex-cli)",
			strings.Join(names, ","), len(got))
	}
	wantOrder := []Agent{AgentClaudeCode, AgentCodexCLI}
	for i, want := range wantOrder {
		if got[i].Agent() != want {
			t.Errorf("Adapters()[%d].Agent() = %q, want %q", i, got[i].Agent(), want)
		}
	}
}

func TestAdapterFor(t *testing.T) {
	t.Parallel()
	for _, a := range []Agent{AgentClaudeCode, AgentCodexCLI} {
		if _, ok := AdapterFor(a); !ok {
			t.Errorf("AdapterFor(%q) = (_, false); want (impl, true) at PR 3", a)
		}
	}
	// gemini-cli / copilot are not yet registered.
	for _, a := range []Agent{AgentGeminiCLI, AgentCopilot} {
		if _, ok := AdapterFor(a); ok {
			t.Errorf("AdapterFor(%q) = (_, true); PR 3 should NOT yet register %q", a, a)
		}
	}
}
