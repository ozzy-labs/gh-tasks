# Claude Code Recipes

Recipes for using `gh-tasks` (CLI + skills) from Claude Code.

## Prerequisites

- Claude Code installed (`claude` on `PATH`)
- `gh extension install ozzy-labs/gh-tasks` completed
- `gh auth login` completed
- For initial repo setup, see [installation.md](../installation.md)

## Loading the skills

Claude Code reads skill definitions from `.claude/skills/{name}/SKILL.md`. The `gh-tasks` adapter places `task-add` / `task-plan` / `task-triage` / `task-review` / `task-standup` / `task-link-pr` at the same paths.

> **Note**: as of v0.1.0 the consumer-side delivery pipeline is still being wired up ([Issue #16](https://github.com/ozzy-labs/gh-tasks/issues/16)). Until that lands, build locally and copy `dist/claude-code/.claude/skills/` into the consumer's `.claude/skills/` manually.

```bash
pnpm run build:skills        # generates dist/claude-code/.claude/skills/{name}/SKILL.md
```

The skill SSOT lives in `src/skills/{name}/SKILL.md`. Claude Code reads the frontmatter (`name`, `description`, `allowed-tools`) and may auto-trigger the skill when relevant. To invoke explicitly, use slash-command form like `/task-add`.

## Use cases

### 1. Weekly plan in the morning

At the start of the week:

```text
/task-plan --period weekly --scope user
```

The skill first runs `gh tasks plan` with `--dry-run`, presents candidates, then commits when you confirm. Omitting `--scope` falls back to git remote inference.

### 2. Inbox triage

Sweep untriaged Issues / draft items:

```text
/task-triage --scope org --limit 10
```

The skill walks each item with you, proposing label / scope / close decisions interactively.

### 3. Capturing a task mid-conversation

When you realize "this should be a separate task" mid-implementation:

```text
/task-add 'Refactor scope-detection cache to use LRU' --scope repo
```

The skill extracts body / acceptance criteria / related PRs from conversation context and dispatches `gh tasks add`. The returned Issue URL is your confirmation.

### 4. Linking on PR creation

Right after opening a PR, link to its tracking Issue / Project item:

```text
/task-link-pr 123 456
```

In `repo` scope, `Closes #456` is appended idempotently to the PR body. In `org` / `user` scope, both surface together on the same Project board.

### 5. End-of-day retrospective

```text
/task-review --period daily --scope user
```

Outputs Markdown grouped into Done / In progress / Blockers. The skill prompts for learnings and carryover items.

### 6. Standup share-out

Before standup, summarize the last 24h:

```text
/task-standup --mine --scope org
```

Yesterday / Today / Blockers Markdown that pastes straight into Slack or meeting notes.

## CLI vs skill: when to use which

- **Use the CLI (`gh tasks ...`) directly**: from automation scripts, cron, CI, or whenever you want full argument control
- **Use a skill (`/task-*`)**: when you need conversation context (e.g. `task-add` extracting body from chat), when you want interactive confirmation, or when multi-step judgement should be delegated to the skill

Skills only call the CLI under the hood, so side-effects are identical. When a skill fails, running the CLI directly is the fastest way to surface the underlying error.

## Troubleshooting

### Skill not recognized

- Confirm `.claude/skills/{name}/SKILL.md` exists
- Confirm the frontmatter `name` matches the directory name
- Restart Claude Code (skills are loaded at session start)

### `--scope` auto-detection fails

- No git remote `origin`, or `gh` not authenticated
- Either pass `--scope user` explicitly, or check the precedence rules in [scope-detection.md](../scope-detection.md)

### `gh tasks` not found

- `gh extension install ozzy-labs/gh-tasks` not yet run
- Check via `gh extension list`

### Projects v2 fields missing

- First-time `org` / `user` scope use needs the field definitions in [projects-v2-setup.md](../projects-v2-setup.md)
- Ensure at minimum `Status` and `Iteration` fields exist on the project

## Related

- [cli-reference.md](../cli-reference.md): all commands / flags
- [concepts.md](../concepts.md): scope / item / iteration glossary
- [src/skills/](../../../src/skills/): skill SSOT
