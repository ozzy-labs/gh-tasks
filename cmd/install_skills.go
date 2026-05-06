package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/install"
	"github.com/ozzy-labs/gh-tasks/internal/skills"
)

// embeddedSkillsRoot is the directory inside Deps.EmbeddedSkills that holds
// the canonical skill SSOT. It mirrors the on-disk `skills/` layout used by
// the development tree and matches the `//go:embed all:skills` directive in
// main.go.
const embeddedSkillsRoot = "skills"

func newInstallSkillsCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "install-skills",
		Short: "Install gh-tasks skills into a consumer repo (1-action UX)",
		Long: `Install the canonical gh-tasks skill bundle into a consumer repository.

The default flow is "auto-detect + write": gh tasks looks at the target
directory for tell-tale signs of Claude Code, Codex CLI, Gemini CLI, or
GitHub Copilot and writes the skill files each agent expects. Use --agent
to override detection.

A per-adapter manifest (.gh-tasks-manifest.json) records the files this
command writes so subsequent runs can tell "previously installed by
gh-tasks" from "third-party file" without prompting.`,
		RunE: func(c *cobra.Command, _ []string) error {
			return runInstallSkills(c, deps)
		},
	}
	c.Flags().StringSlice("agent", nil,
		"agent(s) to install for (claude-code|codex-cli|gemini-cli|copilot); omit for auto-detect")
	c.Flags().String("target", ".", "target directory (consumer repo root)")
	c.Flags().Bool("dry-run", false, "show planned actions without writing")
	c.Flags().Bool("check", false, "exit non-zero if any file would be created or updated (CI dogfooding)")
	return c
}

func runInstallSkills(c *cobra.Command, deps Deps) error {
	r, err := deps.Resolve(c)
	if err != nil {
		return localizedError(c, r, err)
	}

	target, _ := c.Flags().GetString("target")
	dryRun, _ := c.Flags().GetBool("dry-run")
	check, _ := c.Flags().GetBool("check")
	agentFlag, _ := c.Flags().GetStringSlice("agent")

	absTarget, err := resolveTarget(target)
	if err != nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.targetNotADirectory", "path", target))
		return ErrSilentArgs
	}

	if deps.EmbeddedSkills == nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.embedNotAvailable"))
		return ErrSilentRuntime
	}
	loaded, err := skills.LoadFS(deps.EmbeddedSkills, embeddedSkillsRoot, skills.LoadOptions{})
	if err != nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.loadSkills", "reason", err.Error()))
		return ErrSilentRuntime
	}

	agents, err := resolveAgents(c, r, agentFlag, absTarget)
	if err != nil {
		return err
	}

	out := c.OutOrStdout()
	fmt.Fprintln(out, r.T("install.heading", "target", absTarget))
	if len(agentFlag) == 0 {
		fmt.Fprintln(out, r.T("install.detect.auto", "agents", joinAgents(agents)))
	} else {
		fmt.Fprintln(out, r.T("install.detect.specified", "agents", joinAgents(agents)))
	}
	if dryRun {
		fmt.Fprintln(out, r.T("install.dryRun.header"))
	}

	driftCount := 0
	for _, a := range agents {
		impl, ok := install.AdapterFor(a)
		if !ok {
			// Agent name validated by resolveAgents but not yet wired into
			// the install registry (PR 3-5 in flight). Surface a clear
			// localized error rather than silently skipping.
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.unknownAgent",
				"value", string(a),
				"valid", joinAgents(registeredAgents()),
			))
			return ErrSilentArgs
		}
		manifestPath := impl.ManifestPath(absTarget)
		existing, err := install.ReadManifest(manifestPath)
		if err != nil {
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.manifestParseFailed",
				"path", manifestPath, "reason", err.Error(),
			))
			return ErrSilentRuntime
		}
		actions, err := impl.Plan(install.PlanContext{
			SkillsFS:   deps.EmbeddedSkills,
			SkillsRoot: embeddedSkillsRoot,
			TargetRoot: absTarget,
			Skills:     loaded,
			Existing:   existing,
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(out, "\n%s:\n", a)
		for _, act := range actions {
			fmt.Fprintln(out, renderAction(r, act))
		}

		counts := install.Tally(actions)

		if check {
			if counts.HasDrift() {
				fmt.Fprintln(c.ErrOrStderr(), r.T("install.check.diff",
					"agent", string(a),
					"count", counts.Created+counts.Updated+counts.Conflicts,
				))
				driftCount++
			} else {
				fmt.Fprintln(out, r.T("install.check.ok", "agent", string(a)))
			}
			continue
		}

		if dryRun {
			continue
		}

		if counts.Conflicts > 0 {
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.conflict",
				"agent", string(a),
				"count", counts.Conflicts,
			))
			return ErrSilentRuntime
		}

		res, err := install.Execute(actions)
		if err != nil {
			return err
		}
		// Preserve previously-tracked entries that we just rewrote (Update
		// actions) and the entries we Skipped because content matched —
		// dropping them would let a future run see them as third-party.
		manifest := install.Manifest{
			SchemaVersion: install.ManifestSchemaVersion,
			GHTasksVer:    Version,
			Agent:         a,
			Files:         mergeTrackedRelPaths(res.Files, actions, false),
			Shared:        mergeTrackedRelPaths(res.Shared, actions, true),
		}
		if err := install.WriteManifest(manifestPath, manifest); err != nil {
			return err
		}
		fmt.Fprintln(out, r.T("install.summary.installed",
			"agent", string(a),
			"created", counts.Created,
			"updated", counts.Updated,
			"skipped", counts.Skipped,
		))
		fmt.Fprintln(out, r.T("install.manifest.written", "path", manifestPath))
	}

	if dryRun {
		fmt.Fprintln(out, r.T("install.dryRun.note"))
	}
	if check && driftCount > 0 {
		return ErrSilentRuntime
	}
	return nil
}

// resolveTarget canonicalizes the --target flag to an absolute path and
// verifies it points at a directory. The mkdir-if-missing semantics are
// deliberately omitted: `gh tasks install-skills` should be run inside an
// existing consumer repo, not bootstrap a new tree.
func resolveTarget(target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}
	return abs, nil
}

// resolveAgents combines the --agent flag and auto-detection into a final
// ordered slice. When --agent is empty, AutoDetect is run; an empty
// detection result yields a localized error pointing at the user. Any
// --agent value that is not a member of [install.Agents] yields
// error.install.unknownAgent.
func resolveAgents(c *cobra.Command, r Resolved, agentFlag []string, target string) ([]install.Agent, error) {
	if len(agentFlag) == 0 {
		detected := install.AutoDetect(target)
		if len(detected) == 0 {
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.noAgentDetected",
				"valid", joinAgents(registeredAgents()),
			))
			return nil, ErrSilentArgs
		}
		return detected, nil
	}
	out := []install.Agent{}
	seen := map[install.Agent]bool{}
	for _, raw := range agentFlag {
		// StringSlice values can still arrive comma-glued when callers pass
		// `--agent a,b`. Split here so the user-facing semantics match the
		// flag's help text without making the cobra plumbing pickier.
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			a, ok := install.ValidateAgent(part)
			if !ok {
				fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.unknownAgent",
					"value", part,
					"valid", joinAgents(install.Agents),
				))
				return nil, ErrSilentArgs
			}
			if seen[a] {
				continue
			}
			seen[a] = true
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.noAgentDetected",
			"valid", joinAgents(registeredAgents()),
		))
		return nil, ErrSilentArgs
	}
	return out, nil
}

// registeredAgents returns the agents whose adapters are wired into this
// build of gh-tasks, sorted in canonical order. Distinct from
// [install.Agents] (which is the design-target list — every agent the
// install-skills feature plans to support, not just those shipping in the
// current build).
func registeredAgents() []install.Agent {
	out := []install.Agent{}
	for _, impl := range install.Adapters() {
		out = append(out, impl.Agent())
	}
	sort.Slice(out, func(i, j int) bool {
		return canonicalAgentRank(out[i]) < canonicalAgentRank(out[j])
	})
	return out
}

func canonicalAgentRank(a install.Agent) int {
	for i, x := range install.Agents {
		if x == a {
			return i
		}
	}
	return len(install.Agents)
}

func joinAgents(in []install.Agent) string {
	if len(in) == 0 {
		return "(none)"
	}
	parts := make([]string, len(in))
	for i, a := range in {
		parts[i] = string(a)
	}
	return strings.Join(parts, ", ")
}

func renderAction(r Resolved, a install.Action) string {
	switch a.Type {
	case install.ActionCreate:
		return i18n.T(r.Locale, "install.action.create", "path", a.RelPath)
	case install.ActionUpdate:
		return i18n.T(r.Locale, "install.action.update", "path", a.RelPath)
	case install.ActionSkip:
		return i18n.T(r.Locale, "install.action.skip", "path", a.RelPath)
	case install.ActionConflict:
		return i18n.T(r.Locale, "install.action.conflict", "path", a.RelPath)
	}
	return a.RelPath
}

// mergeTrackedRelPaths produces the Files (when shared=false) or Shared
// (when shared=true) list that should land in the freshly written
// manifest. We keep every path the adapter just touched (Create + Update,
// already enumerated in `written`) and every path it judged as already in
// sync (ActionSkip with the matching `a.Shared` flag). Conflict paths are
// excluded by construction — Execute would already have errored out
// before reaching this helper, but the explicit filter keeps the manifest
// pure if a future caller starts treating Execute's conflict guard as
// advisory.
func mergeTrackedRelPaths(written []string, actions []install.Action, shared bool) []string {
	out := make([]string, 0, len(actions))
	seen := map[string]bool{}
	for _, p := range written {
		if !seen[p] {
			out = append(out, p)
			seen[p] = true
		}
	}
	for _, a := range actions {
		if a.Type != install.ActionSkip {
			continue
		}
		if a.Shared != shared {
			continue
		}
		if !seen[a.RelPath] {
			out = append(out, a.RelPath)
			seen[a.RelPath] = true
		}
	}
	return out
}
