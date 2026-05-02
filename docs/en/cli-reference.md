# CLI reference

All `gh tasks` commands and flags. v0.1.0 target.

Legend:

- ✅ Implemented
- 🚧 Planned for v0.1.0 (not yet implemented)

## Common flags

- `--scope repo|org|user`: target scope. Auto-detected when omitted ([scope-detection.md](./scope-detection.md))
- `--repo <owner>/<name>`: repo-scope target. Defaults to git remote `origin`
- `--lang ja|en`: output language. Resolves in order `LC_ALL` → `LANG` env → default `en` ([locale-detection.md](./locale-detection.md))
- `--help`, `-h`: show help
- `--version`, `-v`: show version

## Commands

### `gh tasks add <title>` ✅ repo scope implemented

Add an Issue / Project draft item.

```bash
gh tasks add '<title>' [--scope repo] [--repo <owner>/<name>] [--body '<detail>']
```

- `repo` scope: creates a GitHub Issue
- `org` / `user` scope: creates a Projects v2 draft item (🚧 follow-up to v0.1.0)

Returns: prints the URL of the created Issue / item to stdout, exits 0.

### `gh tasks list` 🚧

List tasks per scope.

### `gh tasks today` 🚧

Pull today's todos.

### `gh tasks plan [--period daily|weekly|sprint]` 🚧

Week / iteration plan. `repo` scope updates a Milestone; `org` / `user` scope updates a Project v2 Iteration.

### `gh tasks triage` 🚧

Triage untriaged items. Assists with labeling, scope routing, and close decisions.

### `gh tasks done <id>` 🚧

Mark done (`repo`: Issue close; `org` / `user`: Status → Done).

### `gh tasks review [--period daily|weekly|sprint]` 🚧

Generate a retrospective summary.

### `gh tasks standup [--mine]` 🚧

Activity summary.

### `gh tasks link <pr> <task>` 🚧

Link a PR to its tracking Issue / Project item.

## Skill integration

Each command has a corresponding skill SSOT under `src/skills/{name}/SKILL.md` (ja) + `SKILL.en.md` (en). The adapter pipeline emits per-agent outputs to `dist/{adapter}/` for claude-code / codex-cli / gemini-cli / copilot. See repo-internal [ADR-0004](../adr/0004-skill-frontmatter-schema.md).

## Related

- [scope-detection.md](./scope-detection.md): `--scope` resolution order
- [src/skills/](../../src/skills/): skill SSOT for each command
