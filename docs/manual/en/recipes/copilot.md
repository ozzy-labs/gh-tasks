# GitHub Copilot Recipes

Recipes for using `gh-tasks` (CLI + skills) from GitHub Copilot.

## Prerequisites

- GitHub Copilot Chat / Coding Agent available
- `gh extension install ozzy-labs/gh-tasks` completed
- `gh auth login` completed
- For initial repo setup, see [installation.md](../guides/installation.md)

## Loading the skills

GitHub Copilot reads `.github/copilot-instructions.md` as its top-level instruction file. Copilot does not load `SKILL.md` directly, so skills are surfaced as a list of names + descriptions inside the marker block. The fastest way to set this up:

```bash
cd /path/to/your-repo
gh tasks install-skills            # auto-detects copilot from .github/copilot-instructions.md
```

This merges the gh-tasks marker block into `.github/copilot-instructions.md` (creating the file if missing) and tracks the contribution in `.github/.gh-tasks-copilot-manifest.json`. The marker block is exclusive to gh-tasks; content outside it is preserved verbatim.

Common variations:

- `gh tasks install-skills --agent copilot` — explicit selection
- `gh tasks install-skills --namespace gh-tasks` — rename install
- `gh tasks install-skills --uninstall` — strip the marker block; the surrounding consumer content is left intact

The Renovate auto-sync path is also available — see [`configs/skills-sync/README.md`](../../../../configs/skills-sync/README.md).

Either include AGENTS.md by reference from `copilot-instructions.md`, or place the skill marker block alongside AGENTS.md. Snippet shape:

```markdown
<!-- begin: @ozzylabs/gh-tasks -->

## gh-tasks Skills

- `task-add` — ...
- `task-plan` — ...

<!-- end: @ozzylabs/gh-tasks -->
```

When you reference a skill by name, Copilot infers the corresponding `gh tasks` invocation and proposes it.

## Use cases

### 1. Weekly plan in the morning

In Copilot Chat:

```text
@workspace Run task-plan weekly for user scope.
```

Copilot proposes `gh tasks plan --period weekly --scope user`.

### 2. Inbox triage

```text
@workspace task-triage in org scope, limit 10.
```

Copilot proposes `gh tasks triage --scope org --limit 10` and helps you triage based on context.

### 3. Capturing a task mid-conversation

When a side todo comes up in chat:

```text
@workspace Capture "Refactor scope-detection cache to use LRU" as a repo-scope task.
```

Copilot proposes `gh tasks add`.

### 4. Linking on PR creation

From a PR template or PR comment:

```text
@workspace Link this PR (#123) to Issue #456.
```

Copilot proposes `gh tasks link 123 456`. In `repo` scope, `Closes #456` is appended idempotently to the PR body.

### 5. End-of-day retrospective

```text
@workspace Run task-review daily for user scope.
```

Markdown summary suitable for an Issue comment or wiki page.

### 6. Standup share-out

```text
@workspace standup --mine for org scope, last 24h.
```

Copilot proposes `gh tasks standup --mine --scope org`.

## CLI vs skill: when to use which

Copilot does not load `SKILL.md` bodies, so the skill mechanism is reduced to a name + description list.

- **Run the CLI directly**: from a terminal, in scripts, or in CI
- **Use the Copilot proposal**: when you want commands assembled from natural language, or when invoking from a PR / Issue comment

Copilot Coding Agent can run `gh` CLI in its environment, which combines well with PR comment triggers — useful for automating `task-link-pr`.

## Troubleshooting

### Skill name doesn't trigger anything

- Confirm the marker block exists in `.github/copilot-instructions.md`
- Reopen the repo so Copilot reloads instructions
- Re-run `gh tasks install-skills` from the repo root (idempotent)

### `--scope` auto-detection fails

- No git remote `origin`, or `gh` not authenticated
- Pass `--scope user` explicitly, or see [scope-detection.md](../reference/scope-detection.md)

### `gh tasks` not found

- `gh extension install ozzy-labs/gh-tasks` not yet run
- In Copilot Coding Agent environments, ensure a setup step installs the extension first
- Check via `gh extension list`

### `copilot-instructions.md` snippet corrupted

- Re-run `gh tasks install-skills` to rewrite the marker block (idempotent — safe to run repeatedly)

### Projects v2 fields missing

- First-time `org` / `user` scope use needs the field definitions in [projects-v2-setup.md](../guides/projects-v2-setup.md)

## Related

- [cli.md](../reference/cli.md): all commands / flags
- [concepts.md](../concepts.md): scope / item / iteration glossary
- [skills/](../../../../skills/): skill SSOT
