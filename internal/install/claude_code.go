package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ClaudeCodeAdapter installs skills under <target>/.claude/skills/<name>/
// SKILL.md, mirroring the layout produced by the build-side
// adapters.ClaudeCode renderer. The on-disk Content is the canonical
// skills.Skill.Raw — i.e. SKILL.md verbatim, no transformation.
type ClaudeCodeAdapter struct{}

// Agent returns the canonical agent name for the manifest and CLI flags.
func (ClaudeCodeAdapter) Agent() Agent { return AgentClaudeCode }

// Detect returns true when the target tree shows traces of Claude Code:
// either a `.claude/` directory or a top-level `CLAUDE.md`.
func (ClaudeCodeAdapter) Detect(targetRoot string) bool {
	return DetectClaudeCode(targetRoot)
}

// claudeCodeSkillsSubdir is the relative subdirectory under TargetRoot
// where claude-code skills live. The manifest sits as a hidden sibling of
// the per-skill directories so a stray `ls` does not surface it.
const claudeCodeSkillsSubdir = ".claude/skills"

// claudeCodeManifestName is the manifest filename (relative to the skills
// dir). Dot-prefixed so it never collides with a skill directory and so
// `LoadFS`-style scans that skip dot-prefix entries (see internal/skills)
// won't accidentally treat it as a skill.
const claudeCodeManifestName = ".gh-tasks-manifest.json"

// ManifestPath returns the absolute path of the claude-code manifest
// inside targetRoot.
func (ClaudeCodeAdapter) ManifestPath(targetRoot string) string {
	return filepath.Join(targetRoot, filepath.FromSlash(claudeCodeSkillsSubdir), claudeCodeManifestName)
}

// Plan computes the install actions for the claude-code adapter. It walks
// the canonical skill list and, for each skill, classifies the target
// SKILL.md as Create / Update / Skip / Conflict against the on-disk file
// + previous Manifest.
func (a ClaudeCodeAdapter) Plan(ctx PlanContext) ([]Action, error) {
	if ctx.TargetRoot == "" {
		return nil, fmt.Errorf("install/claude-code: PlanContext.TargetRoot is empty")
	}
	out := make([]Action, 0, len(ctx.Skills))
	for _, s := range ctx.Skills {
		relSlash := claudeCodeSkillsSubdir + "/" + s.Name + "/SKILL.md"
		absPath := filepath.Join(ctx.TargetRoot, filepath.FromSlash(relSlash))
		desired := s.Raw

		existing, exists, err := readIfExists(absPath)
		if err != nil {
			return nil, err
		}
		switch {
		case !exists:
			out = append(out, Action{
				Type:    ActionCreate,
				Path:    absPath,
				RelPath: relSlash,
				Content: desired,
			})
		case ctx.Existing.Has(relSlash):
			// Tracked by a previous gh-tasks run: safe to overwrite or
			// skip depending on whether the SSOT actually changed.
			if existing == desired {
				out = append(out, Action{
					Type: ActionSkip, Path: absPath, RelPath: relSlash,
				})
			} else {
				out = append(out, Action{
					Type:    ActionUpdate,
					Path:    absPath,
					RelPath: relSlash,
					Content: desired,
				})
			}
		case ctx.Force:
			// PR 6 `--force`: downgrade conflict to update + .bak
			// backup. Existing (untracked) content is preserved at
			// <path>.bak so the user can recover the third-party file
			// after the fact.
			out = append(out, Action{
				Type:     ActionUpdate,
				Path:     absPath,
				RelPath:  relSlash,
				Content:  desired,
				BackupTo: absPath + ".bak",
			})
		default:
			// Existing but untracked — refuse to clobber. Resolve via
			// `--namespace` (rename install) or `--force` (overwrite +
			// .bak backup).
			out = append(out, Action{
				Type: ActionConflict, Path: absPath, RelPath: relSlash,
			})
		}
	}
	return out, nil
}

// readIfExists returns (content, exists, error). A missing file yields
// ("", false, nil); any other error is reported.
func readIfExists(path string) (string, bool, error) {
	body, err := os.ReadFile(path) //nolint:gosec // path is derived from adapter Plan under a controlled target root
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read %s: %w", path, err)
	}
	return string(body), true, nil
}
