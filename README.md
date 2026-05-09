English | [日本語](README.ja.md)

# gh-tasks

GitHub CLI extension and skill bundle for managing tasks across GitHub Projects v2, Issues, and Milestones — for personal todos, single-project work, and cross-project coordination.

`gh tasks` provides a unified abstraction over three scopes (`repo` / `org` / `user`) so the same commands work whether you are tracking a personal todo, a single repository's backlog, or cross-project coordination via a shared Project v2.

The CLI consolidates Projects v2 access into a single binary + skill bundle, distributed for Claude Code, Codex CLI, GitHub Copilot, and Gemini CLI via an agent adapter mechanism.

## Status

v0.1.0 — feature-complete. The CLI commands and skills described below are implemented and tested across all three scopes (`repo` / `org` / `user`). Releases are managed by release-please. See [docs/adr/](docs/adr/) for repo-internal decisions and [docs/manual/en/](docs/manual/en/) for the user manual.

## Install

```bash
gh extension install ozzy-labs/gh-tasks
```

Optional shorter alias:

```bash
gh alias set task tasks
```

The extension inherits authentication from `gh auth login` — no separate token configuration is required.

## CLI commands

| Command | Purpose |
| --- | --- |
| `gh tasks add <title>` | Add an Issue or Project draft item. `--scope repo\|org\|user`, `--repo <name>`, `--project <id>` |
| `gh tasks list` | List filtered tasks (scope-aware). `--limit <N>` (default 30) |
| `gh tasks today` | Show tasks due / scheduled today |
| `gh tasks plan [--period daily\|weekly\|sprint] [--write]` | Plan a week / iteration (Milestone for repo, Iteration for org/user). Preview by default; pass `--write` to apply |
| `gh tasks triage [--limit <N>]` | Triage untriaged Issues / draft items (default 20) |
| `gh tasks done <id>` | Close an Issue (repo) or set Status → Done (org/user) |
| `gh tasks review [--period daily\|weekly\|sprint]` | Retrospective summary |
| `gh tasks standup [--mine] [--since <iso8601>]` | Activity summary (default last 24h) |
| `gh tasks link <pr> <task>` | Link a PR to an Issue / Project item |
| `gh tasks projects init [yaml-path]` | Bootstrap a Project v2 from a yaml template (`--template`, `--owner`, `--title`, `--dry-run`) |
| `gh tasks projects init-templates` | Print the bundled `user` / `org` field templates |

Default `--scope` resolves in this order: explicit `--scope` flag → current working directory's git remote (`origin` present → `repo`) → `~/.config/ozzylabs/gh-tasks.toml` `default_scope` → `user`. Full flag reference: [docs/manual/en/reference/cli.md](docs/manual/en/reference/cli.md).

## Structured output

Every command (read-only **and** mutation: `add` / `done` / `link` / `plan --write` / `projects init` / `init-templates`) accepts `--json [fields]` and `--jq <query>` so they pipe cleanly into shell scripts, agents, and `jq` / `yq`. Tab completion is wired on `--json`; `--paginate` walks the full result set on the read commands.

```bash
# List available fields (empty value)
gh tasks list --json=

# JSON array of selected fields (state OPEN / CLOSED / MERGED is part of the catalog)
gh tasks list --json id,number,state,title

# Built-in jq filter (Pure Go gojq, no external dep)
gh tasks list --json id --jq '.[].id'

# Capture a created Issue's id for downstream commands
issue_id=$(gh tasks add "Bug: 404 on /api" --json id --jq '.[0].id')

# Walk the full result set instead of the per-command default cap
gh tasks list --paginate --json id

# Verify a closed Issue's state programmatically
gh tasks done 42 --json state --jq '.[0].state'
# "CLOSED"
```

`stdout` is JSON-only; warnings and localized errors stay on `stderr`. Output is locale-independent (field names are English, values are GitHub source-of-truth) so scripts behave the same whether run with `--lang en` or `--lang ja`. Full reference: [docs/manual/en/reference/json-output.md](docs/manual/en/reference/json-output.md).

## Skills

| Skill | Purpose |
| --- | --- |
| `task-add` | Capture a task from conversation context |
| `task-plan` | Daily / weekly / sprint planning |
| `task-triage` | Inbox triage |
| `task-review` | Daily / weekly retrospective |
| `task-standup` | Activity summary for team sharing |
| `task-link-pr` | Auto-link a PR to its tracking item |

Skills ship for Claude Code, Codex CLI, GitHub Copilot, and Gemini CLI. There are two ways to deploy them:

### One-shot install (recommended)

```bash
cd /path/to/your-repo
gh tasks install-skills
```

The command auto-detects which agents the repo uses (looks for `.claude/`, `AGENTS.md`, `.gemini/`, `.github/copilot-instructions.md`) and writes the right files for each one. Re-running is idempotent — a per-adapter manifest tracks what gh-tasks owns so subsequent runs only update what changed.

Useful flags:

- `--agent claude-code,codex-cli` — install for specific agents instead of auto-detect
- `--namespace gh-tasks` — rename install to dodge name collisions (`task-add` → `gh-tasks-add`)
- `--force` — overwrite an untracked existing file (the original is preserved at `<path>.bak`)
- `--dry-run` — preview the planned actions
- `--check` — non-zero exit when the on-disk tree is out of sync (CI dogfooding)
- `--uninstall` — remove every file recorded in the manifest. Shared aggregator files (`AGENTS.md`, `.gemini/settings.json`, `.github/copilot-instructions.md`) are reference-counted across adapters

### Renovate auto-sync (auto-update flow)

When you want skill updates to land via PRs in your existing Renovate flow, extend the adapter sub-presets you need:

```jsonc
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": [
    "github>ozzy-labs/gh-tasks//configs/skills-sync/claude-code",
    "github>ozzy-labs/gh-tasks//configs/skills-sync/codex-cli"
  ]
}
```

See [`configs/skills-sync/README.md`](configs/skills-sync/README.md) for the full list of adapter presets and how `gh_tasks_commit:` is tracked alongside `@ozzylabs/skills`.

The two paths are interoperable: both write to the same locations and use the same marker tag, so switching between them does not produce spurious diffs.

## Scope coverage

| Scope | Use case | Backing storage |
| --- | --- | --- |
| `repo` | Single project's implementation work | Issues + Milestones |
| `org` | Cross-project coordination | Organization Project v2 |
| `user` | Personal todos / daily plans | Personal Project v2 |

## Conventions

- **Commits**: [Conventional Commits](https://www.conventionalcommits.org/)
- **Branching**: GitHub Flow with squash merge only
- **Branch naming**: `<type>/<short-description>`

## License

[MIT](LICENSE)
