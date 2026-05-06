# Gemini CLI Recipes

Recipes for using `gh-tasks` (CLI + skills) from Gemini CLI.

## Prerequisites

- Gemini CLI installed
- `gh extension install ozzy-labs/gh-tasks` completed
- `gh auth login` completed
- For initial repo setup, see [installation.md](../guides/installation.md)

## Loading the skills

Gemini CLI reads the file pointed to by `context.fileName` in `.gemini/settings.json` (typically `AGENTS.md`). Gemini CLI itself does not have a `SKILL.md` auto-load mechanism like Claude Code, so skills are exposed as a list of names + descriptions inside `AGENTS.md`. The fastest way to wire both up is:

```bash
cd /path/to/your-repo
gh tasks install-skills            # auto-detects gemini-cli from .gemini/
```

This does two things atomically:

1. Union-merges `{"context":{"fileName":["AGENTS.md"]}}` into the existing `.gemini/settings.json` — every other key (`model`, `temperature`, ...) and every other entry in `fileName` is preserved verbatim. Empty / missing settings.json is created from scratch.
2. Merges the gh-tasks marker block into `AGENTS.md`. The same marker is shared with the codex-cli adapter, so installing both does not produce two duplicate blocks.

Common variations:

- `gh tasks install-skills --agent gemini-cli` — explicit selection
- `gh tasks install-skills --namespace gh-tasks` — rename install
- `gh tasks install-skills --uninstall` — remove the manifest entries. The AGENTS.md marker is reference-counted (kept if codex-cli still needs it); the AGENTS.md entry is removed from `settings.json`'s `context.fileName` but the file itself is preserved (your `model` / `temperature` survive)

The Renovate auto-sync path is also available — see [`configs/skills-sync/README.md`](../../../../configs/skills-sync/README.md).

Example `.gemini/settings.json` after install:

```jsonc
{
  "context": {
    "fileName": ["AGENTS.md"]
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

- Re-run `gh tasks install-skills` from the repo root to refresh both the marker block and the settings.json entry (idempotent)

### Projects v2 fields missing

- First-time `org` / `user` scope use needs the field definitions in [projects-v2-setup.md](../guides/projects-v2-setup.md)

## Related

- [cli.md](../reference/cli.md): all commands / flags
- [concepts.md](../concepts.md): scope / item / iteration glossary
- [skills/](../../../../skills/): skill SSOT
