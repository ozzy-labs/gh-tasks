English | [日本語](README.ja.md)

# gh-tasks

GitHub CLI extension and skill bundle for managing tasks across GitHub Projects v2, Issues, and Milestones — for personal todos, single-project work, and cross-project coordination.

`gh tasks` provides a unified abstraction over three scopes (`repo` / `org` / `user`) so the same commands work whether you are tracking a personal todo, a single repository's backlog, or coordination across the OzzyLabs Platform.

This package backs the [OzzyLabs handbook ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md) decision to consolidate Projects v2 access into a single CLI + skill bundle, distributed for Claude Code, Codex CLI, GitHub Copilot, and Gemini CLI via the [ADR-0018](https://github.com/ozzy-labs/handbook/blob/main/adr/0018-agent-adapter-architecture.md) agent adapter mechanism. Skills SSOT lives in [`@ozzylabs/skills`](https://github.com/ozzy-labs/skills) per [handbook ADR-0016](https://github.com/ozzy-labs/handbook/blob/main/adr/0016-create-skills-repo.md); the v0.1.0 implementation specification was finalized in [handbook reviews/2026-04-30-gh-tasks-design.md](https://github.com/ozzy-labs/handbook/blob/main/reviews/2026-04-30-gh-tasks-design.md).

## Status

Early scaffold. The CLI surface and skill set described below are the v0.1.0 target. See [docs/adr/](docs/adr/) for repo-internal decisions and [docs/en/](docs/en/) for design documentation.

## Install

```bash
gh extension install ozzy-labs/gh-tasks
```

Optional shorter alias:

```bash
gh alias set task tasks
```

The extension inherits authentication from `gh auth login` — no separate token configuration is required.

## CLI commands (v0.1.0 target)

| Command | Purpose |
| --- | --- |
| `gh tasks add <title>` | Add an Issue or Project draft item. `--scope repo\|org\|user`, `--repo <name>` |
| `gh tasks list` / `gh tasks today` | List filtered tasks (scope-aware) |
| `gh tasks plan` | Plan a week / iteration (Milestone for repo, Iteration for org/user) |
| `gh tasks triage` | Triage untriaged Issues / draft items |
| `gh tasks done <id>` | Close an Issue (repo) or set Status → Done (org/user) |
| `gh tasks review [--daily\|--weekly\|--sprint]` | Retrospective summary |
| `gh tasks standup [--mine]` | Activity summary |
| `gh tasks link <pr> <task>` | Link a PR to an Issue / Project item |

Default `--scope` resolves in this order: current working directory's git remote → `~/.config/ozzylabs/gh-tasks.toml` `default_scope` → `repo`.

## Skills (v0.1.0 target)

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
    "github>ozzy-labs/gh-tasks//skills-sync/claude-code",
    "github>ozzy-labs/gh-tasks//skills-sync/codex-cli"
  ]
}
```

See [`skills-sync/README.md`](skills-sync/README.md) for the full list of adapter presets and how `gh_tasks_commit:` is tracked alongside `@ozzylabs/skills`.

## Scope coverage

| Scope | Use case | Backing storage |
| --- | --- | --- |
| `repo` | Single project's implementation work | Issues + Milestones |
| `org` | Cross-project coordination | `OzzyLabs Platform` Project v2 |
| `user` | Personal todos / daily plans | Personal Project v2 |

## Conventions

- **Commits**: [Conventional Commits](https://www.conventionalcommits.org/)
- **Branching**: GitHub Flow with squash merge only
- **Branch naming**: `<type>/<short-description>`

## License

[MIT](LICENSE)
