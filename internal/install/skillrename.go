package install

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

// ApplyNamespace returns the renamed skill name. An empty namespace is a
// no-op. For names beginning with "task-" the leading "task" segment is
// replaced with the namespace ("task-add" + "gh-tasks" → "gh-tasks-add"),
// matching the design example in #327. Other names are prefixed with
// "<namespace>-".
//
// The two-rule design exists so the slash command exposed to a user
// (`/gh-tasks-add`) makes the install provenance obvious without nesting
// directories — and so non-`task-` skills (none in the current SSOT but
// possible in future) still get a sensible rename.
func ApplyNamespace(namespace, name string) string {
	if namespace == "" {
		return name
	}
	if rest, ok := strings.CutPrefix(name, "task-"); ok {
		return namespace + "-" + rest
	}
	return namespace + "-" + name
}

// frontmatterBlockRe matches the leading YAML frontmatter block of a
// SKILL.md file. CRLF line endings are tolerated to match
// skills.ParseDocument's contract.
var frontmatterBlockRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n`)

// frontmatterNameLineRe matches the `name:` key + value inside the
// frontmatter block. Deliberately omits a `$` anchor: Go's `(?m)` only
// recognizes `\n` as a line terminator, so a CRLF source ("name:
// task-add\r\n") would never satisfy `[^\r\n]+$`. Stopping at the first
// `\r` or `\n` in the value and leaving the trailing newline outside
// the match preserves the file's original line endings byte-for-byte
// in the rewrite step.
var frontmatterNameLineRe = regexp.MustCompile(`(?m)^name:[ \t]+[^\r\n]+`)

// RenameSkillContent rewrites the frontmatter `name:` value in raw to
// newName so the on-disk SKILL.md matches its renamed directory. Returns
// an error when raw lacks a frontmatter block or has no `name:` key —
// both cases would mean a skill that skills.LoadFS shouldn't have
// accepted in the first place, but re-validating keeps cmd-layer
// mistakes loud.
func RenameSkillContent(raw, newName string) (string, error) {
	m := frontmatterBlockRe.FindStringSubmatchIndex(raw)
	if m == nil {
		return "", fmt.Errorf("install/rename: missing frontmatter delimiters")
	}
	headerStart, headerEnd := m[2], m[3]
	header := raw[headerStart:headerEnd]
	if !frontmatterNameLineRe.MatchString(header) {
		return "", fmt.Errorf("install/rename: frontmatter has no `name:` key")
	}
	newHeader := frontmatterNameLineRe.ReplaceAllString(header, "name: "+newName)
	return raw[:headerStart] + newHeader + raw[headerEnd:], nil
}

// ApplyNamespaceToSkills returns a copy of in with every skill renamed
// per [ApplyNamespace]. The Frontmatter map and Raw text are updated
// alongside Name so adapters that consume Skill.Raw (claude-code,
// codex-cli) emit a SKILL.md whose on-disk frontmatter matches its
// renamed directory.
//
// An empty namespace returns in unchanged (no copy).
func ApplyNamespaceToSkills(namespace string, in []skills.Skill) ([]skills.Skill, error) {
	if namespace == "" {
		return in, nil
	}
	out := make([]skills.Skill, len(in))
	for i, s := range in {
		newName := ApplyNamespace(namespace, s.Name)
		renamed, err := RenameSkillContent(s.Raw, newName)
		if err != nil {
			return nil, fmt.Errorf("install/rename: %s: %w", s.Name, err)
		}
		ns := s
		ns.Name = newName
		ns.Raw = renamed
		if s.Frontmatter != nil {
			fm := make(map[string]string, len(s.Frontmatter))
			for k, v := range s.Frontmatter {
				fm[k] = v
			}
			fm["name"] = newName
			ns.Frontmatter = fm
		}
		out[i] = ns
	}
	return out, nil
}
