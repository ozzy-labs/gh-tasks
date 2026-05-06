package install

import (
	"fmt"
	"path/filepath"
)

// CopilotAdapter installs the GitHub Copilot integration: a marker block
// merged into `.github/copilot-instructions.md`. Unlike claude-code /
// codex-cli, copilot has no per-skill SKILL.md layout — the entire skill
// catalogue surfaces only as bullet items inside the marker block, so
// the adapter writes one Shared file and nothing else.
//
// The MarkerTag is the same one codex-cli + gemini-cli use, but the host
// file is different (`.github/copilot-instructions.md`, not AGENTS.md),
// so the marker block is independent — no cross-adapter reference count
// is needed for this file. The own-manifest still uses Shared so PR 7's
// uninstall logic stays uniform across adapters.
type CopilotAdapter struct{}

// Agent returns the canonical agent name.
func (CopilotAdapter) Agent() Agent { return AgentCopilot }

// Detect returns true when `.github/copilot-instructions.md` exists.
func (CopilotAdapter) Detect(targetRoot string) bool {
	return DetectCopilot(targetRoot)
}

const (
	copilotInstructionsRel = ".github/copilot-instructions.md"
	copilotManifestSubdir  = ".github"
	copilotManifestName    = ".gh-tasks-copilot-manifest.json"
	copilotSnippetLocale   = "ja"
)

// ManifestPath returns the absolute path of the copilot manifest. The
// filename has a `-copilot` suffix because `.github/` already hosts
// repo-wide config (workflows, dependabot, ...) and a generic
// `.gh-tasks-manifest.json` would be ambiguous.
func (CopilotAdapter) ManifestPath(targetRoot string) string {
	return filepath.Join(targetRoot, copilotManifestSubdir, copilotManifestName)
}

// Plan computes a single Shared marker-block-merge action against
// .github/copilot-instructions.md. No conflict detection: the marker
// block is gh-tasks's exclusive zone, content outside it is preserved.
func (CopilotAdapter) Plan(ctx PlanContext) ([]Action, error) {
	if ctx.TargetRoot == "" {
		return nil, fmt.Errorf("install/copilot: PlanContext.TargetRoot is empty")
	}
	body := RenderAgentsSnippet(ctx.Skills, copilotSnippetLocale)
	abs := filepath.Join(ctx.TargetRoot, filepath.FromSlash(copilotInstructionsRel))
	existing, exists, err := readIfExists(abs)
	if err != nil {
		return nil, err
	}
	desired := MergeMarkerBlock(existing, body)
	switch {
	case !exists:
		return []Action{{
			Type:    ActionCreate,
			Path:    abs,
			RelPath: copilotInstructionsRel,
			Content: desired,
			Shared:  true,
		}}, nil
	case existing == desired:
		return []Action{{
			Type:    ActionSkip,
			Path:    abs,
			RelPath: copilotInstructionsRel,
			Shared:  true,
		}}, nil
	default:
		return []Action{{
			Type:    ActionUpdate,
			Path:    abs,
			RelPath: copilotInstructionsRel,
			Content: desired,
			Shared:  true,
		}}, nil
	}
}

// PlanUninstall strips the gh-tasks marker block from
// `.github/copilot-instructions.md` and removes the per-adapter
// manifest. No ref-counting is needed: the marker block in this file
// is exclusively gh-tasks territory.
func (CopilotAdapter) PlanUninstall(ctx UninstallContext) ([]Action, error) {
	if ctx.TargetRoot == "" {
		return nil, fmt.Errorf("install/copilot: PlanUninstall TargetRoot empty")
	}
	out := make([]Action, 0, 2)

	if hasSharedEntry(ctx.Existing, copilotInstructionsRel) {
		abs := filepath.Join(ctx.TargetRoot, filepath.FromSlash(copilotInstructionsRel))
		existing, exists, err := readIfExists(abs)
		if err != nil {
			return nil, err
		}
		if exists {
			stripped := RemoveMarkerBlock(existing)
			switch stripped {
			case existing:
				// Marker already absent: skip.
			case "":
				out = append(out, Action{
					Type:    ActionRemove,
					Path:    abs,
					RelPath: copilotInstructionsRel,
					Shared:  true,
				})
			default:
				out = append(out, Action{
					Type:    ActionUpdate,
					Path:    abs,
					RelPath: copilotInstructionsRel,
					Content: stripped,
					Shared:  true,
				})
			}
		}
	}

	mfRel := copilotManifestSubdir + "/" + copilotManifestName
	out = append(out, Action{
		Type:    ActionRemove,
		Path:    filepath.Join(ctx.TargetRoot, filepath.FromSlash(mfRel)),
		RelPath: mfRel,
	})
	return out, nil
}
