package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Counts breaks down a finished plan execution by ActionType. cmd uses it
// to render the per-adapter summary line (created / updated / skipped /
// removed) and to drive the --check exit code.
type Counts struct {
	Created   int
	Updated   int
	Skipped   int
	Conflicts int
	Removed   int
}

// HasDrift reports whether the plan contains any non-skip actions. The
// `--check` flag fails when this is true: the on-disk tree no longer
// matches the embedded SSOT.
func (c Counts) HasDrift() bool {
	return c.Created+c.Updated+c.Conflicts+c.Removed > 0
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
		case ActionRemove:
			c.Removed++
		}
	}
	return c
}

// ExecuteResult bundles the relative paths Execute actually wrote, split
// by ownership. Files = adapter-owned (per-skill SKILL.md and similar);
// Shared = consumer-owned aggregator files (AGENTS.md,
// .github/copilot-instructions.md). The split flows directly into the
// matching Manifest.Files / Manifest.Shared fields and is what
// `--uninstall` keys off when reference-counting marker blocks across
// multiple adapters.
//
// Removed enumerates relative paths that ActionRemove deleted from
// disk. Uninstall flows use it for the trailing summary; Files /
// Shared are not populated by ActionRemove (the manifest is being torn
// down anyway, so there is no payload to re-record).
type ExecuteResult struct {
	Files   []string
	Shared  []string
	Removed []string
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
		switch a.Type {
		case ActionCreate, ActionUpdate:
			if a.BackupTo != "" {
				if err := backupExisting(a.Path, a.BackupTo); err != nil {
					return ExecuteResult{}, err
				}
			}
			if err := writeFile(a.Path, a.Content); err != nil {
				return ExecuteResult{}, err
			}
			if a.Shared {
				res.Shared = append(res.Shared, a.RelPath)
			} else {
				res.Files = append(res.Files, a.RelPath)
			}
		case ActionRemove:
			if err := removeFile(a.Path); err != nil {
				return ExecuteResult{}, err
			}
			res.Removed = append(res.Removed, a.RelPath)
		}
	}
	return res, nil
}

// removeFile deletes path. A missing target is treated as success
// (idempotent — repeated `--uninstall` runs do not fail). After a
// successful delete, the immediate parent directory is removed if it
// is empty so the workspace does not retain skeleton folders like
// `.claude/skills/task-add/` that have nothing in them.
func removeFile(path string) error {
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("remove %s: %w", path, err)
	}
	parent := filepath.Dir(path)
	// os.Remove on a directory only succeeds when it is empty, which is
	// exactly the semantic we want — any other state (including
	// permissions errors) is harmless to silently swallow because the
	// file removal already succeeded.
	_ = os.Remove(parent)
	return nil
}

// backupExisting renames src to dst (typically <path>.bak), creating the
// dst's parent directory if it does not already exist. A pre-existing
// dst is overwritten — repeated `--force` runs keep only the most recent
// backup, which matches users' typical mental model of `cp -i`-style
// fallback semantics.
//
// A missing src is not an error: the only caller (Execute) guarantees a
// BackupTo only when the on-disk Plan saw the file, but races (a
// concurrent rm) shouldn't make the install crash.
func backupExisting(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("backup stat %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("mkdir backup parent: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("backup rename %s -> %s: %w", src, dst, err)
	}
	return nil
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
