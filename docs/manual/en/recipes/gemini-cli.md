# Gemini CLI Recipes

Recipes for using `gh-tasks` (CLI + skills) from Gemini CLI.

## Prerequisites

- Gemini CLI installed
- `gh extension install ozzy-labs/gh-tasks` completed
- `gh auth login` completed
- For initial repo setup, see [installation.md](../guides/installation.md)

## Loading the skills

Gemini CLI reads the file pointed to by `context.fileName` in `.gemini/settings.json` (typically `AGENTS.md`). The `gh-tasks` adapter ships only `AGENTS.md.snippet`, which is merged into the marker block in `AGENTS.md`. Gemini CLI itself does not have a `SKILL.md` auto-load mechanism like Claude Code, so skills are exposed as a list of names + descriptions inside `AGENTS.md`.

```bash
# 1. Build the adapter outputs in gh-tasks
gh tasks build-skills    # emits dist/gemini-cli/AGENTS.md.snippet

# 2. From the consumer repo root, run commons' sync-skills.sh with MARKER_TAG override
MARKER_TAG=@ozzylabs/gh-tasks bash /path/to/commons/sync-skills.sh -y \
  /path/to/gh-tasks/dist \
  .
```

The snippet is merged into the marker block in the consumer's `AGENTS.md`. See [`skills-sync/README.md`](../../../../skills-sync/README.md) for details.

Example `.gemini/settings.json`:

```jsonc
{
  "context": {
    "fileName": "AGENTS.md"
  }
}
```

The `AGENTS.md` marker block:

```markdown
<!-- begin: @ozzylabs/gh-tasks -->

## gh-tasks Skills

- `task-add` — Capture a task from conversation context...
- `task-plan` — Run a daily / weekly / iteration plan...

<!-- end: @ozzylabs/gh-tasks -->
```

When you reference a skill by name, Gemini CLI looks up the entry and infers the corresponding `gh tasks` invocation.

## Use cases

### 1. Weekly plan in the morning

```text
Run task-plan weekly for user scope.
```

Gemini CLI consults the skill list in AGENTS.md and dispatches `gh tasks plan --period weekly --scope user`.

### 2. Inbox triage

```text
task-triage in org scope, limit 10.
```

Gemini CLI runs `gh tasks triage --scope org --limit 10` and surfaces decisions for you to review.

### 3. Capturing a task mid-conversation

```text
Capture "Refactor scope-detection cache to use LRU" as a repo-scope task.
```

Equivalent to `gh tasks add 'Refactor scope-detection cache to use LRU' --scope repo`.

### 4. Linking on PR creation

```text
Link PR #123 to Issue #456.
```

Runs `gh tasks link 123 456`.

### 5. End-of-day retrospective

```text
Run task-review daily for user scope.
```

Returns Markdown ready to drop into Slack or meeting notes.

### 6. Standup share-out

```text
Summarize my last 24h in org scope as a standup.
```

Equivalent to `gh tasks standup --mine --scope org`.

## CLI vs skill: when to use which

Gemini CLI has no `SKILL.md` auto-load, so the skill mechanism is thin in practice:

- **Run the CLI directly**: when you need precise behavior control
- **Refer to skills via AGENTS.md**: useful when you only remember the skill name (`task-add` etc.) and want the agent to interpret context

The `SKILL.md` bodies still live under `.agents/skills/` (Codex CLI) and `.claude/skills/` (Claude Code), so Gemini CLI can read them on request — but loading is not automatic.

## Troubleshooting

### Skill name doesn't trigger anything

- Confirm the marker block exists in `AGENTS.md`
- Confirm `.gemini/settings.json` `context.fileName` points at `AGENTS.md`
- Restart Gemini CLI to reload context

### `--scope` auto-detection fails

- No git remote `origin`, or `gh` not authenticated
- Pass `--scope user` explicitly, or see [scope-detection.md](../reference/scope-detection.md)

### `gh tasks` not found

- `gh extension install ozzy-labs/gh-tasks` not yet run
- Check via `gh extension list`

### `AGENTS.md` snippet stale

- Re-run `MARKER_TAG=@ozzylabs/gh-tasks bash /path/to/commons/sync-skills.sh -y /path/to/gh-tasks/dist .` to refresh the marker block (idempotent)
- The snippet update is idempotent — safe to run repeatedly

### Projects v2 fields missing

- First-time `org` / `user` scope use needs the field definitions in [projects-v2-setup.md](../guides/projects-v2-setup.md)

## Related

- [cli.md](../reference/cli.md): all commands / flags
- [concepts.md](../concepts.md): scope / item / iteration glossary
- [src/skills/](../../../../src/skills/): skill SSOT
