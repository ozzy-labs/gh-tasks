// Package install computes and applies the per-agent file layout that
// `gh tasks install-skills` emits into a consumer repository. It is the
// install-side counterpart to internal/adapters: where adapters/ renders
// the dist/{adapter}/ tree consumed by Renovate-based sync, install/ takes
// the same canonical skills.Skill list and writes directly into a target
// repo, recording its provenance in a manifest so subsequent runs can tell
// "previously installed by gh-tasks" from "third-party file" without
// pestering the user.
//
// The package is split intentionally:
//
//   - install.go: shared types (Agent, Action, Manifest, PlanContext) and
//     the AdapterImpl interface every per-agent installer satisfies.
//   - detect.go: ResolveAgents — combines --agent flag + filesystem auto
//     detection into the final list of adapters to run.
//   - manifest.go: ReadManifest / WriteManifest — JSON round-trip for the
//     `.gh-tasks-manifest.json` file each adapter owns.
//   - executor.go: Execute — applies a planned []Action to disk.
//   - claude_code.go (and future codex_cli.go / gemini_cli.go / copilot.go):
//     per-agent AdapterImpl implementations.
//
// PR 2 of #327 ships claude-code only; codex / gemini / copilot adapters
// land in PR 3 / 4 / 5.
package install

import (
	"io/fs"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

// Agent identifies one of the four supported AI agent integrations. The
// string form is what users pass via `--agent` and what is stored in each
// manifest's `agent` field.
type Agent string

// Supported agents. Order is canonical for help text and output.
const (
	AgentClaudeCode Agent = "claude-code"
	AgentCodexCLI   Agent = "codex-cli"
	AgentGeminiCLI  Agent = "gemini-cli"
	AgentCopilot    Agent = "copilot"
)

// Agents lists every agent the install-skills design targets. The actual
// set of adapters wired up at any given time is what [Adapters] returns —
// PR 2 ships claude-code only, with the other three landing in PR 3-5.
var Agents = []Agent{AgentClaudeCode, AgentCodexCLI, AgentGeminiCLI, AgentCopilot}

// ValidateAgent reports whether v is a supported agent name.
func ValidateAgent(v string) (Agent, bool) {
	for _, a := range Agents {
		if string(a) == v {
			return a, true
		}
	}
	return "", false
}

// ActionType categorizes a single planned filesystem operation.
//
// The four types match the conflict-detection state machine described in
// the design doc on #327: a desired file is either absent (Create), present
// and tracked by the previous manifest (Skip / Update depending on content
// drift), or present but untracked (Conflict — refuse to overwrite without
// --force).
type ActionType int

// Action types in display order.
const (
	// ActionCreate writes a new file at Path.
	ActionCreate ActionType = iota
	// ActionUpdate overwrites an existing file at Path that the previous
	// manifest claims as gh-tasks-owned. Triggered when Content has drifted
	// from what is on disk — typically because the SSOT changed since the
	// last install.
	ActionUpdate
	// ActionSkip is a no-op: the file at Path already exists, is tracked by
	// the previous manifest, and matches Content byte-for-byte.
	ActionSkip
	// ActionConflict marks a desired file whose target Path exists but is
	// NOT in the previous manifest. The executor refuses to overwrite this
	// in default mode; PR 6 introduces --force / --namespace to resolve.
	ActionConflict
)

// String renders an ActionType for debug output. The user-facing rendering
// goes through i18n keys (install.action.*).
func (t ActionType) String() string {
	switch t {
	case ActionCreate:
		return "create"
	case ActionUpdate:
		return "update"
	case ActionSkip:
		return "skip"
	case ActionConflict:
		return "conflict"
	}
	return "unknown"
}

// Action is a single planned filesystem operation produced by an adapter's
// Plan() and applied by Execute().
//
// Path is always absolute (rooted at PlanContext.TargetRoot). RelPath is
// the slash-separated form (relative to TargetRoot) used in user-facing
// messages and in the manifest, so output is portable across OSes. Content
// holds the bytes to write for ActionCreate / ActionUpdate; it is empty for
// ActionSkip and ActionConflict.
//
// Shared marks the action as targeting a consumer-owned aggregator file
// (AGENTS.md, .github/copilot-instructions.md) where gh-tasks contributes
// only an idempotent marker block rather than owning the entire file.
// Shared actions never produce ActionConflict — the marker block is the
// adapter's exclusive zone, while content outside the markers is
// preserved untouched. Manifest accounting tracks Shared and Files
// separately so the eventual --uninstall (PR 7) can reference-count the
// marker block across codex-cli + gemini-cli without orphaning consumer
// content.
type Action struct {
	Type    ActionType
	Path    string
	RelPath string
	Content string
	Shared  bool
}

// PlanContext bundles the inputs an AdapterImpl needs to produce its
// []Action. The caller (cmd/install_skills.go) populates this once per
// adapter; adapters do not load skills themselves.
type PlanContext struct {
	// SkillsFS is the embedded filesystem rooted at the gh-tasks repo root
	// (i.e. the embed.FS produced by `//go:embed all:skills`). The skills
	// SSOT lives at <SkillsRoot>/<name>/SKILL.md.
	SkillsFS fs.FS
	// SkillsRoot is the path inside SkillsFS that contains skill
	// directories ("skills" for the canonical embed.FS).
	SkillsRoot string
	// TargetRoot is the absolute path of the consumer repository root.
	TargetRoot string
	// Skills is the pre-loaded canonical skill list (from skills.LoadFS).
	Skills []skills.Skill
	// Existing is the manifest from a previous install run, or the zero
	// value when no manifest exists yet.
	Existing Manifest
}

// AdapterImpl is the install-side contract every per-agent installer
// implements. The methods are pure: Plan reads SkillsFS + the target
// filesystem and returns a deterministic []Action that Execute then writes.
//
// Detect inspects the target filesystem for tell-tale traces of the agent
// (e.g. claude-code looks for `.claude/` or `CLAUDE.md`). It does NOT
// recurse — it is a cheap O(1) probe meant for auto-detection in
// ResolveAgents.
type AdapterImpl interface {
	Agent() Agent
	Detect(targetRoot string) bool
	Plan(ctx PlanContext) ([]Action, error)
	// ManifestPath returns the absolute path where the adapter's manifest
	// file is written, given a target root. Each adapter owns its manifest
	// independently so an uninstall (PR 7) of one agent does not orphan
	// state for another.
	ManifestPath(targetRoot string) string
}

// Adapters returns every install adapter currently registered with this
// build of gh-tasks. PR 3 ships claude-code + codex-cli; gemini-cli /
// copilot land in PR 4 / 5.
func Adapters() []AdapterImpl {
	return []AdapterImpl{
		ClaudeCodeAdapter{},
		CodexCLIAdapter{},
	}
}

// AdapterFor looks up the registered adapter for a given Agent. Returns
// (nil, false) when the agent is recognized by [ValidateAgent] but its
// adapter is not yet wired into this build (e.g. --agent codex-cli on a
// PR 2 binary).
func AdapterFor(a Agent) (AdapterImpl, bool) {
	for _, impl := range Adapters() {
		if impl.Agent() == a {
			return impl, true
		}
	}
	return nil, false
}
