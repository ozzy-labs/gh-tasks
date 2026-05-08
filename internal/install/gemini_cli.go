package install

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// GeminiCLIAdapter installs the Gemini CLI integration: a union-merged
// `.gemini/settings.json` plus the same AGENTS.md marker block the
// codex-cli adapter writes.
//
// settings.json semantics: gh-tasks contributes only the "AGENTS.md"
// entry inside `context.fileName`. Every other top-level key (model,
// temperature, ...) and every other entry under `context` is preserved
// verbatim. Re-running install-skills against a settings.json that
// already contains our entry is a no-op (ActionSkip).
//
// AGENTS.md uses the same MarkerTag as codex-cli so installing both
// agents yields a single shared marker block, not two duplicates. PR 7's
// reference-counted --uninstall is what disambiguates ownership.
type GeminiCLIAdapter struct{}

// Agent returns the canonical agent name.
func (GeminiCLIAdapter) Agent() Agent { return AgentGeminiCLI }

// Detect returns true when `.gemini/` exists under the target.
func (GeminiCLIAdapter) Detect(targetRoot string) bool {
	return DetectGeminiCLI(targetRoot)
}

const (
	geminiSettingsRel    = ".gemini/settings.json"
	geminiManifestSubdir = ".gemini"
	geminiManifestName   = ".gh-tasks-manifest.json"
	geminiAgentsMdRel    = "AGENTS.md"
	geminiAgentsMdLocale = "ja"
	geminiContextEntry   = "AGENTS.md"
)

// ManifestPath returns the absolute path of the gemini-cli manifest.
func (GeminiCLIAdapter) ManifestPath(targetRoot string) string {
	return filepath.Join(targetRoot, geminiManifestSubdir, geminiManifestName)
}

// Plan computes the install actions for gemini-cli: a settings.json
// union merge (Shared) and a marker-block merge into AGENTS.md (Shared).
//
// Both target files are consumer-owned, so neither produces an
// ActionConflict. The settings.json merge is logical (preserve every
// other key); the AGENTS.md merge is textual (preserve every byte
// outside the marker block).
func (GeminiCLIAdapter) Plan(ctx PlanContext) ([]Action, error) {
	if ctx.TargetRoot == "" {
		return nil, fmt.Errorf("install/gemini-cli: PlanContext.TargetRoot is empty")
	}
	out := make([]Action, 0, 2)

	// settings.json
	settingsAbs := filepath.Join(ctx.TargetRoot, filepath.FromSlash(geminiSettingsRel))
	existingSettings, settingsExists, err := readIfExists(settingsAbs)
	if err != nil {
		return nil, err
	}
	desiredSettings, err := MergeGeminiSettings([]byte(existingSettings))
	if err != nil {
		return nil, err
	}
	switch {
	case !settingsExists:
		out = append(out, Action{
			Type:    ActionCreate,
			Path:    settingsAbs,
			RelPath: geminiSettingsRel,
			Content: string(desiredSettings),
			Shared:  true,
		})
	case existingSettings == string(desiredSettings):
		out = append(out, Action{
			Type:    ActionSkip,
			Path:    settingsAbs,
			RelPath: geminiSettingsRel,
			Shared:  true,
		})
	default:
		out = append(out, Action{
			Type:    ActionUpdate,
			Path:    settingsAbs,
			RelPath: geminiSettingsRel,
			Content: string(desiredSettings),
			Shared:  true,
		})
	}

	// AGENTS.md marker block
	body := RenderAgentsSnippet(ctx.Skills, geminiAgentsMdLocale)
	agentsAbs := filepath.Join(ctx.TargetRoot, geminiAgentsMdRel)
	existingAgents, agentsExists, err := readIfExists(agentsAbs)
	if err != nil {
		return nil, err
	}
	// On first create, seed the file with a minimal consumer-owned
	// scaffold so it does not open with our `##` heading. Mirrors the
	// codex-cli adapter — both share the same AGENTS.md target.
	basis := existingAgents
	if !agentsExists {
		basis = AgentsMdScaffold
	}
	desiredAgents := MergeMarkerBlock(basis, body)
	switch {
	case !agentsExists:
		out = append(out, Action{
			Type:    ActionCreate,
			Path:    agentsAbs,
			RelPath: geminiAgentsMdRel,
			Content: desiredAgents,
			Shared:  true,
		})
	case existingAgents == desiredAgents:
		out = append(out, Action{
			Type:    ActionSkip,
			Path:    agentsAbs,
			RelPath: geminiAgentsMdRel,
			Shared:  true,
		})
	default:
		out = append(out, Action{
			Type:    ActionUpdate,
			Path:    agentsAbs,
			RelPath: geminiAgentsMdRel,
			Content: desiredAgents,
			Shared:  true,
		})
	}

	return out, nil
}

// PlanUninstall removes the gh-tasks contribution from
// `.gemini/settings.json` (deleting only the "AGENTS.md" entry, never
// the file as a whole), excises the AGENTS.md marker block iff
// codex-cli no longer references it, and removes the manifest. The
// settings.json file is always preserved on disk because users may
// store unrelated keys (model, temperature, ...) we have no business
// deleting.
func (a GeminiCLIAdapter) PlanUninstall(ctx UninstallContext) ([]Action, error) {
	if ctx.TargetRoot == "" {
		return nil, fmt.Errorf("install/gemini-cli: PlanUninstall TargetRoot empty")
	}
	out := make([]Action, 0, 3)

	// settings.json — always patch (no ref-count needed; only gemini-cli
	// writes the AGENTS.md entry into this file).
	if hasSharedEntry(ctx.Existing, geminiSettingsRel) {
		settingsAbs := filepath.Join(ctx.TargetRoot, filepath.FromSlash(geminiSettingsRel))
		existing, exists, err := readIfExists(settingsAbs)
		if err != nil {
			return nil, err
		}
		if exists {
			stripped, err := RemoveGeminiSettingsEntry([]byte(existing))
			if err != nil {
				return nil, err
			}
			if string(stripped) != existing {
				out = append(out, Action{
					Type:    ActionUpdate,
					Path:    settingsAbs,
					RelPath: geminiSettingsRel,
					Content: string(stripped),
					Shared:  true,
				})
			}
		}
	}

	if hasSharedEntry(ctx.Existing, geminiAgentsMdRel) &&
		!isSharedRelReferencedByOthers(ctx.Others, geminiAgentsMdRel) {
		agentsAbs := filepath.Join(ctx.TargetRoot, geminiAgentsMdRel)
		existing, exists, err := readIfExists(agentsAbs)
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
					Path:    agentsAbs,
					RelPath: geminiAgentsMdRel,
					Shared:  true,
				})
			default:
				out = append(out, Action{
					Type:    ActionUpdate,
					Path:    agentsAbs,
					RelPath: geminiAgentsMdRel,
					Content: stripped,
					Shared:  true,
				})
			}
		}
	}

	mfRel := geminiManifestSubdir + "/" + geminiManifestName
	out = append(out, Action{
		Type:    ActionRemove,
		Path:    filepath.Join(ctx.TargetRoot, filepath.FromSlash(mfRel)),
		RelPath: mfRel,
	})
	return out, nil
}

// MergeGeminiSettings returns the new bytes for `.gemini/settings.json`
// after ensuring `context.fileName` contains "AGENTS.md". Every other
// key — both top-level (model, temperature, ...) and under `context`
// (other custom keys the user may have added) — is preserved verbatim.
//
// existing may be nil or empty for a fresh install. JSON is rendered
// with 2-space indent and a trailing newline so re-emitting an unchanged
// settings.json is idempotent.
//
// Returns an error if existing is non-empty but malformed JSON, or if
// `context` exists but is not an object, or if `context.fileName` exists
// but is neither a string nor an array. The cmd layer surfaces these
// via error.install.* keys to keep the error path localizable.
func MergeGeminiSettings(existing []byte) ([]byte, error) {
	settings := map[string]any{}
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &settings); err != nil {
			return nil, fmt.Errorf("parse settings.json: %w", err)
		}
	}

	contextRaw, hasContext := settings["context"]
	var contextMap map[string]any
	if hasContext {
		m, ok := contextRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("settings.json: 'context' must be an object")
		}
		contextMap = m
	} else {
		contextMap = map[string]any{}
		settings["context"] = contextMap
	}

	filesRaw, hasFiles := contextMap["fileName"]
	var fileList []any
	if hasFiles {
		switch v := filesRaw.(type) {
		case []any:
			fileList = v
		case string:
			// Promote a legacy single-string form to a one-element array
			// so we can append in the standard way without rewriting the
			// existing semantics.
			fileList = []any{v}
		default:
			return nil, fmt.Errorf("settings.json: 'context.fileName' must be a string or array")
		}
	}

	for _, f := range fileList {
		if s, ok := f.(string); ok && s == geminiContextEntry {
			// Already present — write the file back through the same
			// marshal path so unrelated formatting changes (e.g. user
			// re-ordered keys) don't surprise us.
			return marshalSettings(settings, contextMap, fileList)
		}
	}
	fileList = append(fileList, geminiContextEntry)
	return marshalSettings(settings, contextMap, fileList)
}

// marshalSettings writes the merged settings tree, threading fileList
// back into context.fileName so the caller cannot accidentally drop the
// append by reassigning the local slice.
func marshalSettings(settings, contextMap map[string]any, fileList []any) ([]byte, error) {
	contextMap["fileName"] = fileList
	body, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal settings.json: %w", err)
	}
	return append(body, '\n'), nil
}

// RemoveGeminiSettingsEntry returns the settings.json bytes after
// removing "AGENTS.md" from `context.fileName`. The file is preserved:
// every other key under `context` and every other top-level key (model,
// temperature, ...) survives untouched. When removing our entry leaves
// fileName empty, the key is dropped; if `context` itself has no
// remaining keys, it is dropped too — but the document is never deleted
// outright, since users may store unrelated settings in the same file.
//
// An empty / whitespace-only input is returned as-is. A malformed JSON
// document yields an error so the cmd layer can surface a localized
// message rather than silently writing garbage.
func RemoveGeminiSettingsEntry(existing []byte) ([]byte, error) {
	if len(strings.TrimSpace(string(existing))) == 0 {
		return existing, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(existing, &doc); err != nil {
		return nil, fmt.Errorf("parse settings.json: %w", err)
	}
	contextRaw, ok := doc["context"]
	if !ok {
		return existing, nil
	}
	contextMap, ok := contextRaw.(map[string]any)
	if !ok {
		// Non-object context: leave the user's data alone.
		return existing, nil
	}

	switch v := contextMap["fileName"].(type) {
	case []any:
		filtered := make([]any, 0, len(v))
		for _, f := range v {
			if s, ok := f.(string); ok && s == geminiContextEntry {
				continue
			}
			filtered = append(filtered, f)
		}
		if len(filtered) == 0 {
			delete(contextMap, "fileName")
		} else {
			contextMap["fileName"] = filtered
		}
	case string:
		if v == geminiContextEntry {
			delete(contextMap, "fileName")
		}
	default:
		// nil or unexpected type — nothing to remove.
	}
	if len(contextMap) == 0 {
		delete(doc, "context")
	}

	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal settings.json: %w", err)
	}
	return append(body, '\n'), nil
}
