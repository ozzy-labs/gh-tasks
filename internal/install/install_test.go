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

func TestAdapters_PR5_AllFourAdapters(t *testing.T) {
	// PR 5 closes the adapter matrix: claude-code, codex-cli, gemini-cli,
	// copilot. Going forward, [Agents] (the design-target list) and the
	// [Adapters] registry should stay in sync.
	t.Parallel()
	got := Adapters()
	if len(got) != 4 {
		var names []string
		for _, a := range got {
			names = append(names, string(a.Agent()))
		}
		t.Fatalf("Adapters() = [%s] (len %d); PR 5 expects all 4 agents",
			strings.Join(names, ","), len(got))
	}
	wantOrder := []Agent{AgentClaudeCode, AgentCodexCLI, AgentGeminiCLI, AgentCopilot}
	for i, want := range wantOrder {
		if got[i].Agent() != want {
			t.Errorf("Adapters()[%d].Agent() = %q, want %q", i, got[i].Agent(), want)
		}
	}
}

func TestAdapterFor(t *testing.T) {
	t.Parallel()
	for _, a := range Agents {
		if _, ok := AdapterFor(a); !ok {
			t.Errorf("AdapterFor(%q) = (_, false); want (impl, true) at PR 5", a)
		}
	}
}

func TestAdapterFor_UnknownReturnsFalse(t *testing.T) {
	// Calling with an Agent value that isn't registered must return
	// (nil, false). The cmd layer's resolveAgents already filters via
	// ValidateAgent, but install.AdapterFor is also called directly in
	// the uninstall flow and must stay defensive.
	t.Parallel()
	if impl, ok := AdapterFor(Agent("not-a-real-agent")); ok || impl != nil {
		t.Errorf("AdapterFor(unknown) = (%v, %v); want (nil, false)", impl, ok)
	}
}
