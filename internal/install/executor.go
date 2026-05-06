package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// Counts breaks down a finished plan execution by ActionType. cmd uses it
// to render the per-adapter summary line (created / updated / skipped) and
// to drive the --check exit code.
type Counts struct {
	Created   int
	Updated   int
	Skipped   int
	Conflicts int
}

// HasDrift reports whether the plan contains any non-skip actions. The
// `--check` flag fails when this is true: the on-disk tree no longer
// matches the embedded SSOT.
func (c Counts) HasDrift() bool {
	return c.Created+c.Updated+c.Conflicts > 0
}

// Tally groups a slice of Actions into Counts.
func Tally(actions []Action) Counts {
	var c Counts
	for _, a := range actions {
		switch a.Type {
		case ActionCreate:
			c.Created++
		case ActionUpdate:
			c.Updated++
		case ActionSkip:
			c.Skipped++
		case ActionConflict:
			c.Conflicts++
		}
	}
	return c
}

// ExecuteResult bundles the relative paths Execute actually wrote, split
// by ownership. Files = adapter-owned (per-skill SKILL.md and similar);
// Shared = consumer-owned aggregator files (AGENTS.md,
// .github/copilot-instructions.md). The split flows directly into the
// matching Manifest.Files / Manifest.Shared fields and is what PR 7's
// --uninstall keys off when reference-counting marker blocks across
// multiple adapters.
type ExecuteResult struct {
	Files  []string
	Shared []string
}

// Execute applies a planned []Action to disk. It refuses to proceed when
// any Conflict action is present — the cmd layer checks for conflicts
// first and surfaces a localized error before calling Execute, but the
// guard is repeated here so direct callers (tests, future scripting) can
// not silently overwrite untracked files.
//
// On success Execute returns the relative paths it actually wrote
// (Create + Update), split into Files (owned) and Shared (consumer-owned
// aggregator). Skip and Conflict actions never appear in the result.
func Execute(actions []Action) (ExecuteResult, error) {
	for _, a := range actions {
		if a.Type == ActionConflict {
			return ExecuteResult{}, fmt.Errorf("refusing to execute plan with %d conflict(s); resolve before retrying", Tally(actions).Conflicts)
		}
	}
	res := ExecuteResult{}
	for _, a := range actions {
		if a.Type != ActionCreate && a.Type != ActionUpdate {
			continue
		}
		if err := writeFile(a.Path, a.Content); err != nil {
			return ExecuteResult{}, err
		}
		if a.Shared {
			res.Shared = append(res.Shared, a.RelPath)
		} else {
			res.Files = append(res.Files, a.RelPath)
		}
	}
	return res, nil
}

// writeFile creates parent directories as needed and writes content at
// path with mode 0o600. Skill assets are text (SKILL.md, settings.json)
// that never need an exec bit, mirroring the build-skills convention.
func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
