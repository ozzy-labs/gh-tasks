// Package skills parses skill SSOT files (frontmatter + body) and resolves
// SKILL.md / SKILL.en.md pairs.
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Skill is the canonical (ja SSOT) shape passed to adapters. Mirrors the TS
// `Skill` type 1:1.
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

// frontmatterRe matches a leading YAML frontmatter block. Mirrors the TS
// implementation: `^---\n(...)\n---\n`.
var frontmatterRe = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`)

// ParseDocument splits frontmatter (key: value lines) from body. fileLabel is
// included in error messages.
func ParseDocument(text, fileLabel string) (map[string]string, string, error) {
	m := frontmatterRe.FindStringSubmatchIndex(text)
	if m == nil {
		return nil, "", fmt.Errorf("%s: missing frontmatter (--- ... ---)", fileLabel)
	}
	header := text[m[2]:m[3]]
	body := text[m[1]:]
	frontmatter := map[string]string{}
	for _, line := range strings.Split(header, "\n") {
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if key != "" {
			frontmatter[key] = value
		}
	}
	return frontmatter, body, nil
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

// Load reads each src/skills/<name>/SKILL.md, validates the frontmatter, and
// returns the parsed Skill list sorted by name.
func Load(srcDir string, opts LoadOptions) ([]Skill, error) {
	required := opts.Required
	if len(required) == 0 {
		required = defaultRequiredFields
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir: %w", err)
	}
	names := []string{}
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	skills := []Skill{}
	for _, name := range names {
		path := filepath.Join(srcDir, name, "SKILL.md")
		raw, err := os.ReadFile(path) //nolint:gosec // SSOT path is internal
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		label := fmt.Sprintf("src/skills/%s/SKILL.md", name)
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
