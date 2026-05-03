# Codex CLI Recipes

Recipes for using `gh-tasks` (CLI + skills) from Codex CLI.

## Prerequisites

- Codex CLI installed
- `gh extension install ozzy-labs/gh-tasks` completed
- `gh auth login` completed
- For initial repo setup, see [installation.md](../installation.md)

## Loading the skills

Codex CLI starts from `AGENTS.md` and resolves referenced skills from `.agents/skills/{name}/SKILL.md`. The `gh-tasks` adapter ships both:

- `.agents/skills/{name}/SKILL.md` — the skill body (SSOT, as-is)
- `AGENTS.md.snippet` — a marker block listing the skills, inserted into the consumer's `AGENTS.md`

> **Note**: as of v0.1.0 the consumer-side delivery pipeline is still being wired up ([Issue #16](https://github.com/ozzy-labs/gh-tasks/issues/16)). Until that lands, build locally and merge `dist/codex-cli/.agents/skills/` plus `AGENTS.md.snippet` into the consumer manually.

```bash
pnpm run build:skills    # generates dist/codex-cli/.agents/skills/{name}/SKILL.md and AGENTS.md.snippet
```

The injected `AGENTS.md` block looks like:

```markdown
<!-- begin: @ozzylabs/gh-tasks -->

## gh-tasks Skills

- `task-add` — ...
- `task-plan` — ...

<!-- end: @ozzylabs/gh-tasks -->
```

Codex CLI reads this list and, when you reference a skill name, opens `.agents/skills/{name}/SKILL.md` for the procedure.

## Use cases

### 1. Weekly plan in the morning

```text
Run task-plan weekly for user scope.
```

Codex CLI loads the `task-plan` skill and dispatches `gh tasks plan --period weekly --scope user`.

### 2. Inbox triage

```text
task-triage in org scope, limit 10.
```

Codex CLI follows the `task-triage` procedure: fetches untriaged items, then walks decisions with you.

### 3. Capturing a task mid-conversation

When a side todo surfaces during implementation:

```text
Capture "Refactor scope-detection cache to use LRU" as a repo-scope task.
```

The skill condenses context into body text and runs `gh tasks add`.

### 4. Linking on PR creation

```text
Link PR #123 to Issue #456.
```

The `task-link-pr` skill runs `gh tasks link 123 456`.

### 5. End-of-day retrospective

```text
Run task-review daily for user scope.
```

Returns Markdown ready to drop into Slack or meeting notes.

### 6. Standup share-out

```text
Summarize my last 24h in org scope as a standup.
```

Equivalent to `task-standup --mine --scope org`.

## CLI vs skill: when to use which

- **Run the CLI directly**: when scripting, when you need full argument control, or when automating
- **Go through a skill**: when conversation context matters (e.g. `task-add` body extraction), or when you want the skill to handle multi-step judgement

Codex CLI skills work as Markdown procedures — the agent walks the steps. Unlike a raw CLI, judgement steps are part of the flow.

## Troubleshooting

### Skill not recognized

- Confirm the marker block exists in `AGENTS.md` (`<!-- begin: @ozzylabs/gh-tasks -->`)
- Confirm `.agents/skills/{name}/SKILL.md` exists
- Re-copy the output of `pnpm run build:skills` (once [Issue #16](https://github.com/ozzy-labs/gh-tasks/issues/16) lands, the finalized sync command will replace this manual step)

### `--scope` auto-detection fails

- No git remote `origin`, or `gh` not authenticated
- Pass `--scope user` explicitly, or see [scope-detection.md](../scope-detection.md)

### `AGENTS.md` marker block corrupted

- The sync script idempotently rewrites between markers — re-run to recover
- When editing manually, only edit outside the marker block

### `gh tasks` not found

- `gh extension install ozzy-labs/gh-tasks` not yet run
- Check via `gh extension list`

### Projects v2 fields missing

- First-time `org` / `user` scope use needs the field definitions in [projects-v2-setup.md](../projects-v2-setup.md)

## Related

- [cli-reference.md](../cli-reference.md): all commands / flags
- [concepts.md](../concepts.md): scope / item / iteration glossary
- [src/skills/](../../../src/skills/): skill SSOT
