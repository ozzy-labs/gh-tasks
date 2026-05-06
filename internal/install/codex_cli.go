package install

import (
	"fmt"
	"path/filepath"
)

// CodexCLIAdapter installs skills under <target>/.agents/skills/<name>/
// SKILL.md (mirroring the build-side adapters.CodexCLI dist/ output) and
// merges a marker block into the consumer-owned AGENTS.md.
//
// The AGENTS.md contribution uses [MergeMarkerBlock] with the same
// MarkerTag the gemini-cli adapter (PR 4) writes, so installing both
// agents yields a single shared marker block — not two — and PR 7's
// reference-counted uninstall can identify when it is safe to drop.
type CodexCLIAdapter struct{}

// Agent returns the canonical agent name.
func (CodexCLIAdapter) Agent() Agent { return AgentCodexCLI }

// Detect returns true when the target tree shows traces of Codex CLI
// (or a generic AGENTS.md-style multi-agent project): an existing
// `AGENTS.md` file or a `.agents/skills/` directory.
func (CodexCLIAdapter) Detect(targetRoot string) bool {
	return DetectCodexCLI(targetRoot)
}

const (
	codexSkillsSubdir   = ".agents/skills"
	codexManifestName   = ".gh-tasks-manifest.json"
	codexAgentsMdRel    = "AGENTS.md"
	codexAgentsMdLocale = "ja"
)

// ManifestPath returns the absolute path of the codex-cli manifest.
func (CodexCLIAdapter) ManifestPath(targetRoot string) string {
	return filepath.Join(targetRoot, filepath.FromSlash(codexSkillsSubdir), codexManifestName)
}

// Plan computes install actions for codex-cli: per-skill SKILL.md files
// (owned, conflict-detected against the previous manifest) and a single
// AGENTS.md marker-block merge action (shared, never produces conflict).
func (a CodexCLIAdapter) Plan(ctx PlanContext) ([]Action, error) {
	if ctx.TargetRoot == "" {
		return nil, fmt.Errorf("install/codex-cli: PlanContext.TargetRoot is empty")
	}
	out := make([]Action, 0, len(ctx.Skills)+1)

	for _, s := range ctx.Skills {
		relSlash := codexSkillsSubdir + "/" + s.Name + "/SKILL.md"
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
			if existing == desired {
				out = append(out, Action{Type: ActionSkip, Path: absPath, RelPath: relSlash})
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
			// backup so the third-party file survives recoverably.
			out = append(out, Action{
				Type:     ActionUpdate,
				Path:     absPath,
				RelPath:  relSlash,
				Content:  desired,
				BackupTo: absPath + ".bak",
			})
		default:
			out = append(out, Action{
				Type: ActionConflict, Path: absPath, RelPath: relSlash,
			})
		}
	}

	// AGENTS.md marker-block merge. Shared = consumer-owned aggregator,
	// so we never raise Conflict — the marker block is our exclusive zone
	// and content outside it is preserved verbatim by MergeMarkerBlock.
	body := RenderAgentsSnippet(ctx.Skills, codexAgentsMdLocale)
	agentsAbs := filepath.Join(ctx.TargetRoot, codexAgentsMdRel)
	existingAgents, exists, err := readIfExists(agentsAbs)
	if err != nil {
		return nil, err
	}
	desiredAgents := MergeMarkerBlock(existingAgents, body)

	switch {
	case !exists:
		out = append(out, Action{
			Type:    ActionCreate,
			Path:    agentsAbs,
			RelPath: codexAgentsMdRel,
			Content: desiredAgents,
			Shared:  true,
		})
	case existingAgents == desiredAgents:
		out = append(out, Action{
			Type:    ActionSkip,
			Path:    agentsAbs,
			RelPath: codexAgentsMdRel,
			Shared:  true,
		})
	default:
		out = append(out, Action{
			Type:    ActionUpdate,
			Path:    agentsAbs,
			RelPath: codexAgentsMdRel,
			Content: desiredAgents,
			Shared:  true,
		})
	}

	return out, nil
}

// PlanUninstall removes every owned per-skill SKILL.md plus the manifest,
// and excises the AGENTS.md marker block iff no still-installed adapter
// (gemini-cli today; future adapters that share the marker block) still
// needs it. AGENTS.md is preserved when the marker block is the only
// gh-tasks contribution and other consumer content remains.
func (a CodexCLIAdapter) PlanUninstall(ctx UninstallContext) ([]Action, error) {
	if ctx.TargetRoot == "" {
		return nil, fmt.Errorf("install/codex-cli: PlanUninstall TargetRoot empty")
	}
	out := make([]Action, 0, len(ctx.Existing.Files)+2)
	for _, rel := range ctx.Existing.Files {
		out = append(out, Action{
			Type:    ActionRemove,
			Path:    filepath.Join(ctx.TargetRoot, filepath.FromSlash(rel)),
			RelPath: rel,
		})
	}

	if hasSharedEntry(ctx.Existing, codexAgentsMdRel) &&
		!isSharedRelReferencedByOthers(ctx.Others, codexAgentsMdRel) {
		agentsAbs := filepath.Join(ctx.TargetRoot, codexAgentsMdRel)
		existing, exists, err := readIfExists(agentsAbs)
		if err != nil {
			return nil, err
		}
		if exists {
			stripped := RemoveMarkerBlock(existing)
			switch stripped {
			case existing:
				// Marker already absent (manual edit): nothing to do.
			case "":
				out = append(out, Action{
					Type:    ActionRemove,
					Path:    agentsAbs,
					RelPath: codexAgentsMdRel,
					Shared:  true,
				})
			default:
				out = append(out, Action{
					Type:    ActionUpdate,
					Path:    agentsAbs,
					RelPath: codexAgentsMdRel,
					Content: stripped,
					Shared:  true,
				})
			}
		}
	}

	mfRel := codexSkillsSubdir + "/" + codexManifestName
	out = append(out, Action{
		Type:    ActionRemove,
		Path:    filepath.Join(ctx.TargetRoot, filepath.FromSlash(mfRel)),
		RelPath: mfRel,
	})
	return out, nil
}
