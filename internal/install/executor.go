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

// Execute applies a planned []Action to disk. It refuses to proceed when
// any Conflict action is present — the cmd layer checks for conflicts
// first and surfaces a localized error before calling Execute, but the
// guard is repeated here so direct callers (tests, future scripting) can
// not silently overwrite untracked files.
//
// On success Execute returns the list of relative paths it actually wrote
// (Create + Update). The caller composes this with any Shared entries
// (PR 3+ for AGENTS.md) and persists a fresh Manifest.
func Execute(actions []Action) ([]string, error) {
	for _, a := range actions {
		if a.Type == ActionConflict {
			return nil, fmt.Errorf("refusing to execute plan with %d conflict(s); resolve before retrying", Tally(actions).Conflicts)
		}
	}
	written := []string{}
	for _, a := range actions {
		if a.Type != ActionCreate && a.Type != ActionUpdate {
			continue
		}
		if err := writeFile(a.Path, a.Content); err != nil {
			return nil, err
		}
		written = append(written, a.RelPath)
	}
	return written, nil
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
