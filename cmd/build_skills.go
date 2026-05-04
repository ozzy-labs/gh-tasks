package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/adapters"
	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

func newBuildSkillsCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:    "build-skills",
		Short:  "Build skill bundles for adapter agents",
		Hidden: true,
		RunE: func(c *cobra.Command, _ []string) error {
			return runBuildSkills(c, deps)
		},
	}
	c.Flags().Bool("check-diff", false, "fail if dist/ output differs from source SSOT (CI dogfooding)")
	c.Flags().String("src", "src/skills", "skill SSOT directory")
	c.Flags().String("dist", "dist", "output directory root (WARNING: contents of <dist>/<adapter>/ are wiped before regeneration)")
	return c
}

// sanitizeDist rejects --dist values that would be unsafe to RemoveAll under,
// such as "", ".", "/", or "..". Programmer-facing (the build-skills cmd is
// Hidden) so the message is plain ASCII.
func sanitizeDist(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("refusing to use unsafe --dist value: empty string")
	}
	cleaned := filepath.Clean(raw)
	switch cleaned {
	case ".", "..", "/":
		return "", fmt.Errorf("refusing to use unsafe --dist value %q (resolves to %q)", raw, cleaned)
	}
	if cleaned == string(filepath.Separator) {
		return "", fmt.Errorf("refusing to use unsafe --dist value %q (resolves to root separator)", raw)
	}
	return cleaned, nil
}

// LocalStage is a (dist subpath, repo path) pair used to mirror generated
// skill files into this repo's `.claude/skills/` and `.agents/skills/` for
// dogfooding.
type localStage struct {
	DistSubpath string
	LocalPath   string
}

func defaultLocalStages(repoRoot string) []localStage {
	return []localStage{
		{DistSubpath: ".claude/skills", LocalPath: filepath.Join(repoRoot, ".claude", "skills")},
		{DistSubpath: ".agents/skills", LocalPath: filepath.Join(repoRoot, ".agents", "skills")},
	}
}

func runBuildSkills(c *cobra.Command, deps Deps) error {
	src, _ := c.Flags().GetString("src")
	distRoot, _ := c.Flags().GetString("dist")
	checkDiff, _ := c.Flags().GetBool("check-diff")

	distRoot, err := sanitizeDist(distRoot)
	if err != nil {
		return err
	}

	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(c.OutOrStdout(), "[build-skills] no src/skills/ — nothing to build")
		return nil
	}

	loaded, err := skills.Load(src, skills.LoadOptions{})
	if err != nil {
		return fmt.Errorf("load skills: %w", err)
	}

	all := adapters.All()
	for _, adapter := range all {
		adapterRoot := filepath.Join(distRoot, adapter.ID())
		if err := os.RemoveAll(adapterRoot); err != nil {
			return fmt.Errorf("clear %s: %w", adapterRoot, err)
		}
		for _, out := range adapter.Generate(loaded) {
			dest := filepath.Join(adapterRoot, out.RelativePath)
			if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
				return fmt.Errorf("mkdir %s: %w", dest, err)
			}
			// 0o600 cap: skill outputs are text (SKILL.md / SKILL.en.md / settings.json)
			// that never need exec bit. Pinning a conservative mode keeps gosec happy
			// and matches the trust model that this dist/ tree is consumed by humans
			// (committed to the repo) rather than executed directly. If we ever stage
			// binary assets that need 0o755 (or preserve source mode), revisit this.
			if err := os.WriteFile(dest, []byte(out.Content), 0o600); err != nil {
				return fmt.Errorf("write %s: %w", dest, err)
			}
		}
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	stages := defaultLocalStages(repoRoot)
	for _, adapter := range all {
		adapterRoot := filepath.Join(distRoot, adapter.ID())
		for _, stage := range stages {
			distSkillsDir := filepath.Join(adapterRoot, stage.DistSubpath)
			if _, err := os.Stat(distSkillsDir); errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err := os.MkdirAll(stage.LocalPath, 0o750); err != nil {
				return err
			}
			for _, s := range loaded {
				srcPath := filepath.Join(distSkillsDir, s.Name)
				if _, err := os.Stat(srcPath); errors.Is(err, os.ErrNotExist) {
					continue
				}
				destPath := filepath.Join(stage.LocalPath, s.Name)
				if err := os.RemoveAll(destPath); err != nil {
					return err
				}
				if err := copyDir(srcPath, destPath); err != nil {
					return fmt.Errorf("copy skill %s: %w", s.Name, err)
				}
			}
		}
	}

	out := c.OutOrStdout()
	fmt.Fprintf(out, "✓ Built %d skill(s) for %d adapters\n", len(loaded), len(all))
	for _, adapter := range all {
		fmt.Fprintf(out, "  dist/%s/\n", adapter.ID())
	}
	fmt.Fprintln(out, "  staged into .claude/skills/, .agents/skills/")
	for _, s := range loaded {
		fmt.Fprintf(out, "  - %s\n", s.Name)
	}

	if checkDiff {
		// --check-diff is currently informational. CI dogfooding will compare
		// dist/ against the prior committed state via a follow-up integration
		// once the skill SSOT is fully maintained from the Go pipeline.
		fmt.Fprintln(out, "(check-diff mode requested; no diff implementation yet)")
	}
	return nil
}

// copyDir mirrors src into dst preserving relative paths. All files are written
// with mode 0o600 regardless of source mode: skill assets are text (SKILL.md
// etc.) and never need an exec bit, so pinning a conservative mode keeps gosec
// satisfied and matches the trust model (human-readable repo content, not
// executables). Revisit if binary assets requiring 0o755 are ever staged.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		return copyFile(p, target, 0o600)
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec // path comes from skill load loop
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec // dst is a controlled dist/ path
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}
