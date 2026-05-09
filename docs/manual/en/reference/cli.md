# CLI reference

All `gh tasks` commands and flags.

## Common flags

- `--scope repo|org|user`: target scope. Auto-detected when omitted ([scope-detection.md](./scope-detection.md))
- `--repo <owner>/<name>`: repo-scope target. Defaults to git remote `origin`
- `--project <owner>/<number>`: org / user scope target Projects v2. Defaults to `org_project` / `user_project` from config
- `--lang ja|en`: output language. Resolves in priority order `--lang` flag → config `lang` → `LC_ALL` → `LANG` → `en` ([locale-detection.md](./locale-detection.md))
- `--help`, `-h`: show help
- `--version`, `-v`: show version

### Structured output

Every command (read-only **and** mutation: `add` / `done` / `link` / `plan --write` / `projects init` / `init-templates`) accepts `--json [fields]` and `--jq <query>` to emit machine-readable output for scripts and agents. `--json` is tab-completable, and `--paginate` walks the full result set on the read commands (`list`, `today`, `triage`, `standup`, `review`). See [json-output.md](./json-output.md) for the field catalog, contract, and examples.

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
gh tasks plan [--period daily|weekly|sprint] [--scope ...] [--repo ...] [--project ...] [--write]
```

- `repo` scope: finds-or-creates a Milestone for the period and binds open Issues whose `updatedAt` falls in the period
- `org` / `user` scope: finds the matching Projects v2 Iteration and updates the Iteration field on items in the period. Iteration selection priority:
  1. exact title match for the period
  2. iteration containing today
  3. iteration starting in the nearest future
  4. otherwise the last available iteration
- **Default is preview**: without `--write` the command prints the proposed milestone / iteration and the candidate list, then exits without mutating GitHub. Pass `--write` to apply the changes
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

### `gh tasks projects init` ✅

Create a Projects v2 board from a YAML template and add the custom fields in one shot.

```bash
gh tasks projects init [<yaml-path> | --template user|org] --title '<project-title>' [--owner <login>|@me] [--dry-run]
```

- Positional arg: YAML path (matches `templates/projects-v2/{user,org}.yaml`)
- `--template user|org`: use the bundled YAML (mutually exclusive with the positional arg)
- `--owner <login>`: project owner (user or org login; default `@me`)
- `--title <string>`: required
- `--dry-run`: list the fields that would be created without issuing mutations
- Field types: `text` / `number` / `date` / `single_select` / `iteration` / `repository` (`repository` is built-in and is skipped automatically)
- `single_select` options are created with `color: GRAY`; recolor in the UI as needed

Returns: prints the created Project URL to stdout, exits 0.

### `gh tasks projects init-templates` ✅

Print the bundled `user` / `org` Projects v2 field templates to stdout. Useful for copying a baseline before customizing locally.

```bash
gh tasks projects init-templates
```

- No arguments / flags
- Output is a single stdout stream containing both templates, each prefixed by a `# --template user` / `# --template org` header line. Pipe into a YAML splitter or redirect to a file as needed
- The templates are bundled into the binary and match `templates/projects-v2/{user,org}.yaml`

### `gh tasks link <pr> <task>` ✅

Link a PR to its tracking Issue / Project item.

```bash
gh tasks link <pr> <task> [--scope ...] [--repo ...] [--project ...]
```

- `repo` scope: appends `Closes #<task>` to the PR body (idempotent — already-linked PRs are reported)
- `org` / `user` scope: adds both the PR and the Issue to the same Projects v2 board so they surface together (the underlying Issue ↔ PR relation comes from the `Closes` keyword on the PR body)

### `gh tasks install-skills` ✅

Install the canonical gh-tasks skill bundle into a consumer repository. Auto-detects which agents the repo uses (`.claude/`, `AGENTS.md`, `.gemini/`, `.github/copilot-instructions.md`) and writes the right files for each, recording provenance in a per-adapter manifest so subsequent runs are idempotent.

```bash
gh tasks install-skills [--agent <name>[,<name>...]] [--target <path>] \
                        [--namespace <prefix>] [--force] \
                        [--dry-run] [--check] [--uninstall]
```

Flags:

- `--agent <name>[,<name>...]`: explicit agent selection (`claude-code` / `codex-cli` / `gemini-cli` / `copilot`). Pass multiple agents as a comma-separated value (`--agent claude-code,codex-cli`) or by repeating the flag (`--agent claude-code --agent codex-cli`). Omit for auto-detect
- `--target <path>`: target directory (consumer repo root). Defaults to the current working directory
- `--namespace <prefix>`: rename install with a prefix (e.g. `--namespace gh-tasks` turns `task-add` into `gh-tasks-add`). Both the on-disk skill directory and the SKILL.md frontmatter `name:` are rewritten
- `--force`: overwrite an untracked existing skill file. The original is preserved at `<path>.bak` so users can recover
- `--dry-run`: preview the planned actions without writing
- `--check`: exit non-zero if the on-disk tree is out of sync with the embedded SSOT (CI dogfooding)
- `--uninstall`: remove every file the per-adapter manifest tracks. Shared aggregator files (`AGENTS.md`, `.gemini/settings.json`, `.github/copilot-instructions.md`) are reference-counted across adapters, so partial uninstalls preserve files that another installed adapter still needs

Per-adapter behaviour:

| Agent | Owned files | Shared files | Manifest |
| --- | --- | --- | --- |
| `claude-code` | `.claude/skills/<name>/SKILL.md` | (none) | `.claude/skills/.gh-tasks-manifest.json` |
| `codex-cli` | `.agents/skills/<name>/SKILL.md` | `AGENTS.md` (marker block) | `.agents/skills/.gh-tasks-manifest.json` |
| `gemini-cli` | (none) | `.gemini/settings.json` (union merge), `AGENTS.md` (marker block) | `.gemini/.gh-tasks-manifest.json` |
| `copilot` | (none) | `.github/copilot-instructions.md` (marker block) | `.github/.gh-tasks-copilot-manifest.json` |

Conflict handling: an existing untracked file at a target path produces an `ActionConflict`. The default refuses to overwrite and prints both `--namespace` and `--force` as resolution paths. Shared aggregator files never produce conflicts — gh-tasks owns only the marker block (`<!-- begin: @ozzylabs/gh-tasks --> ... <!-- end: ... -->`) or specific JSON keys, never the file as a whole.

Returns: prints planned actions and a per-adapter summary (`{created} created, {updated} updated, {skipped} unchanged`) to stdout. Exits 0 on success, non-zero on conflict / error / `--check` drift.

The Renovate auto-sync path (`configs/skills-sync/<adapter>` presets) targets the same on-disk layout and marker tag, so the two paths are interoperable. Pick `install-skills` for one-shot setup, Renovate for auto-update PRs.

## Exit codes

`gh tasks` distinguishes two non-zero exit codes:

- `0` — success
- `1` — runtime failure: GitHub API error, missing token / auth, repo / project / issue not found in the API response, or other operational failure
- `2` — argument validation failure: invalid `--scope` / `--project` / `--period` value, malformed config file, missing required positional arg (e.g. `gh tasks add` without `<title>`), or template / yaml input rejected before any API call

Shell scripts can rely on `$?` to branch:

```bash
gh tasks list --scope=invalid
case $? in
  0) echo OK ;;
  2) echo "fix your flags" ;;
  *) echo "retry / network issue" ;;
esac
```

## Skill integration

Each command has a corresponding skill SSOT under `skills/{name}/SKILL.md` (ja) + `SKILL.en.md` (en). The adapter pipeline emits per-agent outputs to `dist/{adapter}/` for claude-code / codex-cli / gemini-cli / copilot. See repo-internal [ADR-0004](../../../adr/0004-skill-frontmatter-schema.md).

## Related

- [scope-detection.md](./scope-detection.md): `--scope` resolution order
- [locale-detection.md](./locale-detection.md): `--lang` resolution order
- [projects-v2-setup.md](../guides/projects-v2-setup.md): required Projects v2 fields for `org` / `user` scope
- [skills/](../../../../skills/): skill SSOT for each command
