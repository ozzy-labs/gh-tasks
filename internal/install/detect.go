package install

import (
	"os"
	"path/filepath"
)

// DetectClaudeCode returns true when targetRoot looks like a Claude Code
// consumer repo. Heuristics:
//
//   - `.claude/` directory (project-local skills, settings, hooks)
//   - `CLAUDE.md` (project-level instructions)
//
// The two are independent — the user might have CLAUDE.md without
// `.claude/` (instructions only) or `.claude/` without CLAUDE.md (skills
// only). Either is enough.
func DetectClaudeCode(targetRoot string) bool {
	if isDir(filepath.Join(targetRoot, ".claude")) {
		return true
	}
	return isFile(filepath.Join(targetRoot, "CLAUDE.md"))
}

// DetectCodexCLI returns true when targetRoot looks like a Codex CLI
// consumer repo. Heuristics (any one is enough):
//
//   - `.codex/` directory (Codex CLI's project-local config — the parallel
//     of `.claude/` for Claude Code and `.gemini/` for Gemini CLI)
//   - `.agents/` directory (the staging dir gh-tasks owns under codex-cli;
//     either pre-existing convention or left from a prior install)
//   - `AGENTS.md` (project-level instructions Codex CLI reads)
//
// The previous heuristic required `.agents/skills/` specifically, which
// only exists after `gh tasks install-skills` has run — chicken-and-egg
// for first install. The `.agents/` dir alone is now enough, matching the
// `.claude/` / `.gemini/` pattern used by the sibling detectors.
func DetectCodexCLI(targetRoot string) bool {
	if isDir(filepath.Join(targetRoot, ".codex")) {
		return true
	}
	if isDir(filepath.Join(targetRoot, ".agents")) {
		return true
	}
	return isFile(filepath.Join(targetRoot, "AGENTS.md"))
}

// DetectGeminiCLI is wired in PR 4.
func DetectGeminiCLI(targetRoot string) bool {
	return isDir(filepath.Join(targetRoot, ".gemini"))
}

// DetectCopilot is wired in PR 5.
func DetectCopilot(targetRoot string) bool {
	return isFile(filepath.Join(targetRoot, ".github", "copilot-instructions.md"))
}

// AutoDetect runs every registered adapter's Detect and returns the agents
// that match. The result preserves [Agents] order so output is stable
// regardless of map / iteration ordering.
func AutoDetect(targetRoot string) []Agent {
	out := []Agent{}
	registered := map[Agent]AdapterImpl{}
	for _, impl := range Adapters() {
		registered[impl.Agent()] = impl
	}
	for _, a := range Agents {
		impl, ok := registered[a]
		if !ok {
			continue
		}
		if impl.Detect(targetRoot) {
			out = append(out, a)
		}
	}
	return out
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
