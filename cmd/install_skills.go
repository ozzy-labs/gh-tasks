package cmd

import (
	"fmt"
	"io"
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
	c.Flags().String("namespace", "", "rename skills with this prefix (e.g. --namespace gh-tasks turns task-add into gh-tasks-add)")
	c.Flags().Bool("force", false, "overwrite untracked existing files (the original is preserved at <path>.bak)")
	c.Flags().Bool("uninstall", false, "remove every file recorded in the per-adapter manifest (reference-counted for shared aggregator files)")
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
	namespace, _ := c.Flags().GetString("namespace")
	force, _ := c.Flags().GetBool("force")
	uninstall, _ := c.Flags().GetBool("uninstall")

	absTarget, err := resolveTarget(target)
	if err != nil {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.targetNotADirectory", "path", target))
		return ErrSilentArgs
	}

	if uninstall {
		return runUninstallSkills(c, r, absTarget, agentFlag, dryRun)
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
	if namespace != "" {
		loaded, err = install.ApplyNamespaceToSkills(namespace, loaded)
		if err != nil {
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.namespaceRename",
				"namespace", namespace, "reason", err.Error()))
			return ErrSilentRuntime
		}
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
			Force:      force,
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
			renderConflictReport(c.ErrOrStderr(), r, a, actions)
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
			Namespace:     namespace,
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
	case install.ActionRemove:
		return i18n.T(r.Locale, "install.action.remove", "path", a.RelPath)
	}
	return a.RelPath
}

// runUninstallSkills implements `gh tasks install-skills --uninstall`.
// It reads every adapter's manifest up front so each adapter's
// PlanUninstall sees a coherent ref-count view (agents the same
// invocation is also uninstalling are excluded from Others before the
// helper runs). When --agent is omitted the function picks every
// adapter that currently has a manifest on disk: that matches the
// install-side semantics (auto-detect from filesystem traces) but uses
// gh-tasks's own bookkeeping rather than guessing from `.claude/` etc,
// since after a partial install only the manifest reliably says "we
// installed this here".
func runUninstallSkills(c *cobra.Command, r Resolved, absTarget string, agentFlag []string, dryRun bool) error {
	out := c.OutOrStdout()
	manifests := map[install.Agent]install.Manifest{}
	for _, impl := range install.Adapters() {
		mp := impl.ManifestPath(absTarget)
		m, err := install.ReadManifest(mp)
		if err != nil {
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.manifestParseFailed",
				"path", mp, "reason", err.Error()))
			return ErrSilentRuntime
		}
		if !m.IsZero() {
			manifests[impl.Agent()] = m
		}
	}

	agents, err := resolveUninstallAgents(c, r, agentFlag, manifests)
	if err != nil {
		return err
	}

	scheduled := map[install.Agent]bool{}
	for _, a := range agents {
		scheduled[a] = true
	}

	fmt.Fprintln(out, r.T("install.uninstall.heading", "target", absTarget))
	if len(agentFlag) == 0 {
		fmt.Fprintln(out, r.T("install.detect.auto", "agents", joinAgents(agents)))
	} else {
		fmt.Fprintln(out, r.T("install.detect.specified", "agents", joinAgents(agents)))
	}
	if dryRun {
		fmt.Fprintln(out, r.T("install.dryRun.header"))
	}

	for _, a := range agents {
		impl, ok := install.AdapterFor(a)
		if !ok {
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.install.unknownAgent",
				"value", string(a),
				"valid", joinAgents(registeredAgents()),
			))
			return ErrSilentArgs
		}
		others := map[install.Agent]install.Manifest{}
		for ag, m := range manifests {
			if ag == a {
				continue
			}
			if scheduled[ag] {
				continue
			}
			others[ag] = m
		}
		actions, err := impl.PlanUninstall(install.UninstallContext{
			TargetRoot: absTarget,
			Existing:   manifests[a],
			Others:     others,
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(out, "\n%s:\n", a)
		for _, act := range actions {
			fmt.Fprintln(out, renderAction(r, act))
		}

		if dryRun {
			continue
		}
		res, err := install.Execute(actions)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, r.T("install.uninstall.summary",
			"agent", string(a),
			"removed", len(res.Removed),
			"updated", len(res.Files)+len(res.Shared),
		))
	}

	if dryRun {
		fmt.Fprintln(out, r.T("install.dryRun.note"))
	}
	return nil
}

// resolveUninstallAgents picks the agents to uninstall. The semantics
// differ from resolveAgents (the install path) on auto-detect: install
// looks at filesystem traces (`.claude/`, `AGENTS.md`, ...), but
// uninstall looks at which adapters have a manifest on disk — that is
// the only reliable record of "we installed this here." Explicit
// `--agent` is honored verbatim, validated against install.Agents.
func resolveUninstallAgents(c *cobra.Command, r Resolved, agentFlag []string, manifests map[install.Agent]install.Manifest) ([]install.Agent, error) {
	if len(agentFlag) == 0 {
		out := []install.Agent{}
		for _, impl := range install.Adapters() {
			if _, ok := manifests[impl.Agent()]; ok {
				out = append(out, impl.Agent())
			}
		}
		if len(out) == 0 {
			fmt.Fprintln(c.ErrOrStderr(), r.T("install.uninstall.nothingToDo"))
			return nil, ErrSilentRuntime
		}
		return out, nil
	}
	out := []install.Agent{}
	seen := map[install.Agent]bool{}
	for _, raw := range agentFlag {
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
		fmt.Fprintln(c.ErrOrStderr(), r.T("install.uninstall.nothingToDo"))
		return nil, ErrSilentRuntime
	}
	return out, nil
}

// renderConflictReport prints the structured conflict block: a header
// line stating the count + agent, one line per conflicting path, and a
// trailing resolutions section pointing the user at --namespace and
// --force. The per-action listing has already been printed by the main
// loop (renderAction emits "  ! {path} (conflict ...)" for each one),
// so this function does not repeat it.
func renderConflictReport(w io.Writer, r Resolved, a install.Agent, actions []install.Action) {
	count := 0
	for _, act := range actions {
		if act.Type == install.ActionConflict {
			count++
		}
	}
	fmt.Fprintln(w, r.T("error.install.conflict", "agent", string(a), "count", count))
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
