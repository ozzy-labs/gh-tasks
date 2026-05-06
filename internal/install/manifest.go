package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// ManifestSchemaVersion is the JSON-on-disk schema version for the
// per-adapter manifest. It is bumped when the manifest format changes in a
// way that older gh-tasks binaries cannot read.
const ManifestSchemaVersion = 1

// Manifest captures the provenance of files installed into a consumer repo
// by a previous run of `gh tasks install-skills`. Each adapter owns its
// own manifest file (see AdapterImpl.ManifestPath) so removing one agent
// does not orphan state for another.
//
// Files lists every standalone file the adapter writes (e.g.
// `.claude/skills/task-add/SKILL.md`). Shared lists logical contributions
// to consumer-owned aggregator files (PR 3+ only — `AGENTS.md` and
// `.github/copilot-instructions.md` use this for reference-count cleanup).
// Both fields use forward-slash relative paths so manifests round-trip
// cleanly across Windows / macOS / Linux checkouts.
type Manifest struct {
	SchemaVersion int      `json:"schema_version"`
	GHTasksVer    string   `json:"gh_tasks_version"`
	Agent         Agent    `json:"agent"`
	Namespace     string   `json:"namespace,omitempty"`
	Files         []string `json:"files"`
	Shared        []string `json:"shared,omitempty"`
}

// IsZero reports whether m has not been populated (no Agent set). Used by
// adapters to distinguish "no previous install" from "empty file list".
func (m Manifest) IsZero() bool { return m.Agent == "" }

// Has reports whether relPath was tracked by the previous install. relPath
// is matched as-is (forward-slash); callers that have an OS-native path
// should normalize via filepath.ToSlash first.
func (m Manifest) Has(relPath string) bool {
	for _, f := range m.Files {
		if f == relPath {
			return true
		}
	}
	return false
}

// ReadManifest loads the manifest at path. A non-existent file returns the
// zero Manifest and a nil error so callers can treat "first install" the
// same as "previous install with no files".
//
// A read or JSON-decode failure returns a localizable wrapper via
// fmt.Errorf so cmd-layer callers can surface the failure with the
// appropriate i18n key without the install package importing the i18n
// catalog directly.
func ReadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // path is derived from adapter.ManifestPath under a controlled target root
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Manifest{}, nil
		}
		return Manifest{}, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	return m, nil
}

// WriteManifest serializes m as pretty-printed JSON at path, creating any
// missing parent directories. Files / Shared are sorted in place so two
// runs producing the same logical set produce byte-identical manifests
// (helpful for diff review and for `--check`).
func WriteManifest(path string, m Manifest) error {
	if m.SchemaVersion == 0 {
		m.SchemaVersion = ManifestSchemaVersion
	}
	sort.Strings(m.Files)
	sort.Strings(m.Shared)
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("mkdir manifest parent: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}
