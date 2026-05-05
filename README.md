English | [日本語](README.ja.md)

# gh-tasks

GitHub CLI extension and skill bundle for managing tasks across GitHub Projects v2, Issues, and Milestones — for personal todos, single-project work, and cross-project coordination.

`gh tasks` provides a unified abstraction over three scopes (`repo` / `org` / `user`) so the same commands work whether you are tracking a personal todo, a single repository's backlog, or cross-project coordination via a shared Project v2.

The CLI consolidates Projects v2 access into a single binary + skill bundle, distributed for Claude Code, Codex CLI, GitHub Copilot, and Gemini CLI via an agent adapter mechanism.

## Status

v0.1.0 — feature-complete. The CLI commands and skills described below are implemented and tested across all three scopes (`repo` / `org` / `user`); the release tag is published via release-please. See [docs/adr/](docs/adr/) for repo-internal decisions and [docs/manual/en/](docs/manual/en/) for the user manual.

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
| `gh tasks list` / `gh tasks today` | List filtered tasks (scope-aware). `--limit <N>` (default 30) |
| `gh tasks plan [--period daily\|weekly\|sprint] [--dry-run]` | Plan a week / iteration (Milestone for repo, Iteration for org/user) |
| `gh tasks triage [--limit <N>]` | Triage untriaged Issues / draft items (default 20) |
| `gh tasks done <id>` | Close an Issue (repo) or set Status → Done (org/user) |
| `gh tasks review [--period daily\|weekly\|sprint]` | Retrospective summary |
| `gh tasks standup [--mine] [--since <iso8601>]` | Activity summary (default last 24h) |
| `gh tasks link <pr> <task>` | Link a PR to an Issue / Project item |
| `gh tasks projects init` | Bootstrap a Project v2 from a yaml template (`--template`, `--owner`, `--title`) |

Default `--scope` resolves in this order: explicit `--scope` flag → current working directory's git remote (`origin` present → `repo`) → `~/.config/ozzylabs/gh-tasks.toml` `default_scope` → `user`. Full flag reference: [docs/manual/en/reference/cli.md](docs/manual/en/reference/cli.md).

## Skills

| Skill | Purpose |
| --- | --- |
| `task-add` | Capture a task from conversation context |
| `task-plan` | Daily / weekly / sprint planning |
| `task-triage` | Inbox triage |
| `task-review` | Daily / weekly retrospective |
| `task-standup` | Activity summary for team sharing |
| `task-link-pr` | Auto-link a PR to its tracking item |

Skills are distributed for Claude Code, Codex CLI, GitHub Copilot, and Gemini CLI via Renovate auto-sync. Pick the adapter sub-presets you need:

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
