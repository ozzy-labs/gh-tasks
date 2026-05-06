// Package skills parses skill SSOT files (frontmatter + body) and resolves
// SKILL.md / SKILL.en.md pairs. SKILL.md is the canonical (ja) source; the
// adjacent SKILL.en.md is its English mirror and is validated for presence
// and basic frontmatter integrity at load time so silent translation drift
// fails the build instead of leaking through to adapters.
package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill is the canonical (ja SSOT) shape passed to adapters.
type Skill struct {
	Name          string
	Description   string
	DescriptionEN string
	Locale        string
	Frontmatter   map[string]string
	Body          string
	Raw           string
}

// OutputFile is what adapters return to the build orchestrator.
type OutputFile struct {
	RelativePath string
	Content      string
}

// frontmatterRe matches a leading YAML frontmatter block. Accepts both LF
// and CRLF line endings; an optional UTF-8 BOM is stripped before matching
// in [ParseDocument].
var frontmatterRe = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n`)

// utf8BOM is the UTF-8 byte-order-mark sequence (U+FEFF). Built from raw
// bytes so the non-ASCII repo lint (check-i18n) doesn't flag it as a
// forgotten i18n key \u2014 BOM handling has no localized text.
var utf8BOM = string([]byte{0xEF, 0xBB, 0xBF})

// ParseDocument splits frontmatter (YAML key/value lines) from body.
// fileLabel is included in error messages.
//
// The frontmatter block is parsed with gopkg.in/yaml.v3, so values may
// contain `:` (e.g. `allowed-tools: Bash(gh:*)`) without truncation. A
// previous hand-rolled `strings.Index(line, ":")` parser silently dropped
// everything after the first `:` in such values. CRLF line endings and
// a leading UTF-8 BOM (some editors save SKILL.md that way) are accepted.
func ParseDocument(text, fileLabel string) (map[string]string, string, error) {
	text = strings.TrimPrefix(text, utf8BOM)
	m := frontmatterRe.FindStringSubmatchIndex(text)
	if m == nil {
		return nil, "", fmt.Errorf("%s: missing frontmatter (--- ... ---)", fileLabel)
	}
	header := text[m[2]:m[3]]
	body := text[m[1]:]

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(header), &raw); err != nil {
		return nil, "", fmt.Errorf("%s: frontmatter YAML parse failed: %w", fileLabel, err)
	}
	frontmatter := make(map[string]string, len(raw))
	for k, v := range raw {
		if k == "" {
			continue
		}
		frontmatter[k] = stringifyYAMLValue(v)
	}
	return frontmatter, body, nil
}

// stringifyYAMLValue collapses a yaml.v3 decoded value into a string.
// Scalars round-trip via fmt.Sprint; nil becomes "" so an explicitly null
// frontmatter key looks the same as an absent key to AssertRequiredFields.
// Sequences / mappings are not expected in the current SKILL.md schema, but
// if one ever appears it is rendered with %v rather than silently dropped.
func stringifyYAMLValue(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any, map[string]any:
		return fmt.Sprintf("%v", t)
	}
	return fmt.Sprint(v)
}

// AssertRequiredFields returns an error when any required key is missing or
// empty.
func AssertRequiredFields(fm map[string]string, required []string, fileLabel string) error {
	for _, key := range required {
		if fm[key] == "" {
			return fmt.Errorf("%s: frontmatter missing required field %q", fileLabel, key)
		}
	}
	return nil
}

// LoadOptions configures Load.
type LoadOptions struct {
	Required []string
}

// defaultRequiredFields mirrors ADR-0004 + the TS build-skills.mjs gate.
// Kept unexported so external callers cannot mutate the slice; injection is
// available via LoadOptions.Required.
var defaultRequiredFields = []string{
	"name",
	"description",
	"description_en",
	"allowed-tools",
	"locale",
}

// Load reads each skills/<name>/SKILL.md from the OS filesystem. It is a
// thin wrapper over [LoadFS] that resolves paths relative to srcDir via
// [os.DirFS]. New callers should prefer LoadFS so they can pass an
// embedded fs.FS directly and avoid touching the work tree.
func Load(srcDir string, opts LoadOptions) ([]Skill, error) {
	return LoadFS(os.DirFS(srcDir), ".", opts)
}

// LoadFS reads each <root>/<name>/SKILL.md from fsys, validates the
// frontmatter, and returns the parsed Skill list sorted by name.
//
// SKILL.en.md is also loaded and validated when present. Its absence is a
// hard error (every skill must ship a mirror); when present, its
// frontmatter `name` must match SKILL.md's name, and its `locale` must be
// "en". This catches the most common drift scenarios — copy/paste errors,
// missed renames, and locale typos — that the adapters previously ignored.
//
// fsys may be any [fs.FS]: an os.DirFS for work-tree reads, an embed.FS
// for binary-bundled SSOT, or a testing/fstest.MapFS for unit tests. Path
// joining uses [path.Join] (forward slashes), which matches the embed.FS
// contract and is also accepted by os.DirFS.
func LoadFS(fsys fs.FS, root string, opts LoadOptions) ([]Skill, error) {
	required := opts.Required
	if len(required) == 0 {
		required = defaultRequiredFields
	}
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, fmt.Errorf("read skills dir: %w", err)
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	skills := []Skill{}
	for _, name := range names {
		skillDir := path.Join(root, name)
		skillPath := path.Join(skillDir, "SKILL.md")
		raw, err := fs.ReadFile(fsys, skillPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// Tolerate non-skill subdirectories silently rather than
				// failing the entire build when the user drops a stray
				// folder under skills/.
				continue
			}
			return nil, fmt.Errorf("read %s: %w", skillPath, err)
		}
		label := fmt.Sprintf("skills/%s/SKILL.md", name)
		fm, body, err := ParseDocument(string(raw), label)
		if err != nil {
			return nil, err
		}
		if err := AssertRequiredFields(fm, required, label); err != nil {
			return nil, err
		}
		if fm["name"] != name {
			return nil, fmt.Errorf(
				"%s: frontmatter name=%q does not match directory name=%q",
				label, fm["name"], name,
			)
		}
		if fm["locale"] != "ja" {
			return nil, fmt.Errorf(
				"%s: frontmatter locale=%q must be 'ja' for the canonical SSOT",
				label, fm["locale"],
			)
		}
		if err := assertEnglishMirrorFS(fsys, skillDir, name); err != nil {
			return nil, err
		}
		skills = append(skills, Skill{
			Name:          name,
			Description:   fm["description"],
			DescriptionEN: fm["description_en"],
			Locale:        fm["locale"],
			Frontmatter:   fm,
			Body:          body,
			Raw:           string(raw),
		})
	}
	return skills, nil
}

// assertEnglishMirrorFS validates that <skillDir>/SKILL.en.md exists in
// fsys and that its frontmatter is internally consistent (name matches
// the canonical skill name, locale is "en"). Body content is not compared
// against SKILL.md — translation freshness is the author's responsibility —
// but these structural checks ensure adapters that ever consume the
// mirror are not silently emitting stale or mismatched metadata.
func assertEnglishMirrorFS(fsys fs.FS, skillDir, name string) error {
	mirrorPath := path.Join(skillDir, "SKILL.en.md")
	mirrorLabel := fmt.Sprintf("skills/%s/SKILL.en.md", name)
	mirrorRaw, err := fs.ReadFile(fsys, mirrorPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s: SKILL.en.md mirror is missing — every skill must ship a SKILL.en.md alongside SKILL.md", mirrorLabel)
		}
		return fmt.Errorf("read %s: %w", mirrorPath, err)
	}
	mirrorFm, _, err := ParseDocument(string(mirrorRaw), mirrorLabel)
	if err != nil {
		return err
	}
	if got := mirrorFm["name"]; got != name {
		return fmt.Errorf(
			"%s: frontmatter name=%q does not match canonical SKILL.md name=%q",
			mirrorLabel, got, name,
		)
	}
	if got := mirrorFm["locale"]; got != "en" {
		return fmt.Errorf(
			"%s: frontmatter locale=%q must be 'en'",
			mirrorLabel, got,
		)
	}
	return nil
}
