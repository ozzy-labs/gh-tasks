# Installation

## Prerequisites

- `gh` CLI installed and `gh auth login` completed
- macOS / Linux / Windows (any platform `gh extension` supports)

## Install

```bash
gh extension install ozzy-labs/gh-tasks
```

Optional shorter alias:

```bash
gh alias set task tasks
```

## Authentication

`gh-tasks` inherits the token obtained by `gh auth login` via the `GH_TOKEN` environment variable (repo-internal ADR-0003). No separate auth flow.

In automated environments (e.g. GitHub Actions), `GITHUB_TOKEN` works as well.

## Config file

`~/.config/ozzylabs/gh-tasks.toml` (honors `$XDG_CONFIG_HOME`):

```toml
lang = "ja"
default_scope = "repo"
org_project = "ozzy-labs/5"
user_project = "ozzy-3/2"
```

Optional keys:

- `lang`: `ja` / `en`. Default output language (see [locale-detection.md](../reference/locale-detection.md))
- `default_scope`: `repo` / `org` / `user`. Default scope (see [scope-detection.md](../reference/scope-detection.md))
- `org_project`: `<owner>/<number>`. Default Project v2 for `--scope=org`
- `user_project`: `<owner>/<number>`. Default Project v2 for `--scope=user`

A missing config file is silently ignored — flags, env vars, and auto-detection still apply. A malformed TOML or invalid value exits with a clear error message.

## Projects v2 setup

To use `org` / `user` scope, define the required Project v2 fields. See [projects-v2-setup.md](./projects-v2-setup.md).

For `repo` scope only, no extra setup — GitHub Issues / Milestones are used directly.

## Skills (one-shot deploy)

`gh tasks install-skills` writes the gh-tasks skill bundle (`task-add` / `task-plan` / `task-triage` / `task-review` / `task-standup` / `task-link-pr`) into the consumer repository in a single step. Run it from the repository root after `gh extension install ozzy-labs/gh-tasks` finishes.

```bash
cd /path/to/your-repo
gh tasks install-skills            # auto-detects which agents the repo uses
```

Auto-detection looks for filesystem traces (`.claude/`, `AGENTS.md`, `.gemini/`, `.github/copilot-instructions.md`) and writes the right files for each. Re-runs are idempotent — a per-adapter manifest tracks gh-tasks-owned paths so subsequent runs only touch what changed.

Common variations:

- `gh tasks install-skills --agent claude-code,codex-cli` — explicit agent selection
- `gh tasks install-skills --namespace gh-tasks` — rename install (`/task-add` → `/gh-tasks-add`) to dodge name collisions with another tool's skills
- `gh tasks install-skills --force` — overwrite an untracked existing skill (the original is preserved at `<path>.bak`)
- `gh tasks install-skills --dry-run` — preview the planned actions
- `gh tasks install-skills --uninstall` — remove every file the manifest tracks. Shared aggregator files are reference-counted across adapters

The Renovate auto-sync path (`configs/skills-sync/<adapter>` presets) is also available — see [`configs/skills-sync/README.md`](../../../../configs/skills-sync/README.md). Both paths target the same on-disk layout and marker tag, so switching between them is no-op.

Per-agent recipes: [recipes/claude-code.md](../recipes/claude-code.md), [recipes/codex-cli.md](../recipes/codex-cli.md), [recipes/gemini-cli.md](../recipes/gemini-cli.md), [recipes/copilot.md](../recipes/copilot.md).

## Smoke test

```bash
gh tasks --version
gh tasks add 'first task' --scope=repo --repo=<owner>/<name>
```

## Related

- [scope-detection.md](../reference/scope-detection.md)
- [troubleshooting.md](./troubleshooting.md)
- [reference/cli.md#gh-tasks-install-skills](../reference/cli.md) — full `install-skills` flag reference
