# CLI reference

All `gh tasks` commands and flags.

## Common flags

- `--scope repo|org|user`: target scope. Auto-detected when omitted ([scope-detection.md](./scope-detection.md))
- `--repo <owner>/<name>`: repo-scope target. Defaults to git remote `origin`
- `--project <owner>/<number>`: org / user scope target Projects v2. Defaults to `org_project` / `user_project` from config
- `--lang ja|en`: output language. Resolves in priority order `--lang` flag → config `lang` → `LC_ALL` → `LANG` → `en` ([locale-detection.md](./locale-detection.md))
- `--help`, `-h`: show help
- `--version`, `-v`: show version

## Commands

### `gh tasks add <title>` ✅

Add an Issue (`repo`) / Projects v2 draft item (`org` / `user`).

```bash
gh tasks add '<title>' [--scope repo|org|user] [--repo <owner>/<name>] [--project <owner>/<number>] [--body '<detail>']
```

- `repo` scope: creates a GitHub Issue
- `org` / `user` scope: creates a Projects v2 draft item on the resolved project
- `--body '<detail>'` / `--body=<detail>`: body content for the Issue / draft item (no body when omitted)

Returns: prints the URL of the created Issue / draft item id to stdout, exits 0.

### `gh tasks list` ✅

List tasks per scope.

```bash
gh tasks list [--scope ...] [--repo ...] [--project ...] [--limit <n>]
```

- `repo` scope: lists open Issues
- `org` / `user` scope: lists Projects v2 items
- `--limit` defaults to 30

### `gh tasks today` ✅

Items updated within today (UTC midnight `[start, end)`).

```bash
gh tasks today [--scope ...] [--repo ...] [--project ...]
```

### `gh tasks plan [--period daily|weekly|sprint]` ✅

Plan a daily / weekly / sprint cycle.

```bash
gh tasks plan [--period daily|weekly|sprint] [--scope ...] [--repo ...] [--project ...] [--dry-run]
```

- `repo` scope: finds-or-creates a Milestone for the period and binds open Issues whose `updatedAt` falls in the period
- `org` / `user` scope: finds the matching Projects v2 Iteration and updates the Iteration field on items in the period. Iteration selection priority:
  1. exact title match for the period
  2. iteration containing today
  3. iteration starting in the nearest future
  4. otherwise the last available iteration
- `--dry-run`: preview without mutating
- Period boundaries are anchored at local midnight in the resolved IANA timezone (`TZ` env → system tz → UTC fallback). `daily` is 1 day, `weekly` is 7 days from Monday, `sprint` is 14 days from today
- `--period` defaults to `weekly`

### `gh tasks triage` ✅

List untriaged items (Issues with no labels in `repo` scope; items with `Status` unset or set to `Triage` in `org` / `user` scope).

```bash
gh tasks triage [--scope ...] [--repo ...] [--project ...] [--limit <n>]
```

- `--limit` defaults to 20

### `gh tasks done <id>` ✅

Close an Issue (`repo`: `<id>` is the Issue number) or set a Projects v2 item's `Status` to `Done` (`org` / `user`: `<id>` is the project item node id, e.g. `PVTI_xxx`).

```bash
gh tasks done <id> [--scope ...] [--repo ...] [--project ...]
```

### `gh tasks review [--period daily|weekly|sprint]` ✅

Generate a retrospective summary in Markdown.

```bash
gh tasks review [--period daily|weekly|sprint] [--scope ...] [--repo ...] [--project ...]
```

- `repo` scope: aggregates Issues `closedAt` and PRs `mergedAt` falling in the period window
- `org` / `user` scope: aggregates Projects v2 items whose `Status` is `Done` and whose `updatedAt` falls in the window
- `--period` defaults to `weekly`

### `gh tasks standup [--mine]` ✅

Activity summary in Markdown (Yesterday / Today / Blockers sections).

```bash
gh tasks standup [--mine] [--since <iso8601>] [--scope ...] [--repo ...] [--project ...]
```

- `--since` defaults to 24h ago
- `--mine` filters to items where the viewer is the author or an assignee. DraftIssues have no author / assignee fields and are excluded under `--mine`

### `gh tasks link <pr> <task>` ✅

Link a PR to its tracking Issue / Project item.

```bash
gh tasks link <pr> <task> [--scope ...] [--repo ...] [--project ...]
```

- `repo` scope: appends `Closes #<task>` to the PR body (idempotent — already-linked PRs are reported)
- `org` / `user` scope: adds both the PR and the Issue to the same Projects v2 board so they surface together (the underlying Issue ↔ PR relation comes from the `Closes` keyword on the PR body)

## Skill integration

Each command has a corresponding skill SSOT under `src/skills/{name}/SKILL.md` (ja) + `SKILL.en.md` (en). The adapter pipeline emits per-agent outputs to `dist/{adapter}/` for claude-code / codex-cli / gemini-cli / copilot. See repo-internal [ADR-0004](../adr/0004-skill-frontmatter-schema.md).

## Related

- [scope-detection.md](./scope-detection.md): `--scope` resolution order
- [locale-detection.md](./locale-detection.md): `--lang` resolution order
- [projects-v2-setup.md](./projects-v2-setup.md): required Projects v2 fields for `org` / `user` scope
- [src/skills/](../../src/skills/): skill SSOT for each command
