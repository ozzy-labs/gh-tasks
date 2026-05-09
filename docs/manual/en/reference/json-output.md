# JSON output

`gh tasks` mirrors `gh`'s `--json [fields]` / `--jq <query>` interface so the same shell idioms work.

## Synopsis

```bash
# List available fields for a command (empty value)
gh tasks list --json=

# Emit a JSON array of selected fields
gh tasks list --json id,number,title,type

# Apply a built-in jq filter (Pure Go gojq, no external dep)
gh tasks list --json id --jq '.[].id'

# Walk the full result set instead of the per-command default cap
gh tasks list --paginate --json id
```

`--json=` (empty value) lists the available fields on stderr and exits 1. Pass one or more field names, comma-separated, to receive a JSON array on stdout. `--jq <query>` applies a [gojq](https://github.com/itchyny/gojq)-compatible filter to the array; values are emitted one per line, with two-space indent for objects.

Tab completion works on the field list: `gh tasks list --json id,<TAB>` offers the remaining catalog names with the existing prefix preserved.

## Supported commands

| Command | Catalog |
| --- | --- |
| [`list`](./cli.md#gh-tasks-list) / [`today`](./cli.md#gh-tasks-today--period-dailyweeklysprint) | item |
| [`triage`](./cli.md#gh-tasks-triage) | item |
| [`plan`](./cli.md#gh-tasks-plan--period-dailyweeklysprint) (preview and `--write`) | item |
| [`standup`](./cli.md#gh-tasks-standup---mine---since-iso8601) / [`review`](./cli.md#gh-tasks-review--period-dailyweeklysprint) | activity (= item + `category`) |
| [`add`](./cli.md#gh-tasks-add-title) / [`done`](./cli.md#gh-tasks-done-id) | item |
| [`link`](./cli.md#gh-tasks-link-pr-task) | link (= item + `linkType` + `linkedTo`) |
| [`projects init`](./cli.md#gh-tasks-projects-init-yaml-path) / [`projects init-templates`](./cli.md#gh-tasks-projects-init-templates) | projectInit |

`--paginate` is available on the read-only commands (`list` / `today` / `triage` / `standup` / `review`). Other commands resolve a single record or a small fixed set so paginate has no meaningful semantic.

## Field catalog

### `item` (list / today / triage / plan / add / done)

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | GraphQL global ID of the Issue / PR / Project item |
| `number` | int | Issue / PR / Project item number. `0` for draft items |
| `state` | string | Issue / PR state: `"OPEN"` \| `"CLOSED"` \| `"MERGED"`. Empty string `""` for draft items where it does not apply |
| `title` | string | Title |
| `type` | string | `"ISSUE"` \| `"PULL_REQUEST"` \| `"DRAFT_ISSUE"` |
| `updatedAt` | string | Last-update timestamp (RFC 3339) |
| `url` | string | Absolute URL on github.com. Empty string for draft items |

### `activity` (standup / review)

`activity` is `item` + one extra field:

| Field | Type | Notes |
| --- | --- | --- |
| `category` | string | Activity bucket. Values per command below |

#### `category` values

| Command | Scope | Values |
| --- | --- | --- |
| standup | repo | `closed`, `merged`, `in-progress` |
| standup | org / user | `done`, `in-progress` |
| review | repo | `closedIssue`, `mergedPR` |
| review | org / user | `completedProjectItem` |

### `link` (link)

`link` is `item` + two extra fields describing the binding:

| Field | Type | Notes |
| --- | --- | --- |
| `linkType` | string | How the link was established: `"closesAdded"` (PR body got `Closes #N`) or `"projectBind"` (PR + task bound to the same Project v2) |
| `linkedTo` | object \| null | Target task that the PR was linked to. Object `{id, number, title, type, url}` when newly linked, `null` when the link was already in place |

### `projectInit` (projects init / init-templates)

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | GraphQL global ID of the created Project. Empty for `--dry-run` and `init-templates` |
| `number` | int | Project number. `0` for `--dry-run` and `init-templates` |
| `title` | string | Project title |
| `url` | string | Project URL on github.com. Empty for `--dry-run` and `init-templates` |
| `owner` | string | Owner login (`@me` resolved to the actual viewer login at runtime). Empty for `init-templates` |
| `template` | string | Template name (`"user"` / `"org"`) or empty when a custom yaml path was used |
| `fields` | array | `[{name, dataType, options?}]` for the configured field set. `options` is `null` for fields without single-select options |

`projects init --json` emits a single-element array; `projects init-templates --json` emits a 2-element array (user → org) so consumers can iterate.

## Behaviour and contract

### Stream separation

- **stdout** = data only. Either the JSON array or `--jq`-filtered values
- **stderr** = warnings, localized errors, the field catalog (when `--json=` is passed)
- Errors leave **stdout empty** so `... | jq` does not see partial JSON
- Exit code is non-zero on validation / runtime failure

`plan --write --json` is a documented exception: the human-readable text mutation progress flows to stdout *before* the JSON array of bound items, so terminal users still see milestone-creation lines while scripts can locate the trailing `[`.

### Locale independence

`--json` output is independent of `--lang`:

- Field names are English, camelCase
- Values are GitHub source-of-truth (e.g. a Project Status named `"進行中"` is preserved verbatim in `--json` output)
- `--lang en|ja` only affects text-mode output and stderr error messages

### Null and empty arrays

- Selected fields always appear. Missing values become JSON `null` rather than being omitted
- Array fields are `[]` rather than `null` when empty

This makes `jq` expressions like `.[].milestone.title // "none"` and `.labels[]` safe regardless of state.

### Stability

- During the `0.x` series, breaking changes (field rename / removal / value-type change) are allowed and accompanied by `feat!:` commits and CHANGELOG entries
- Field **addition** is non-breaking and may happen in any minor release
- Once `1.0.0` ships, breaking changes require a major bump

See [`docs/design/json-output.md`](../../../design/json-output.md) for the full design rationale.

### Maintaining this reference

The catalogs above are owned by `cmd/jsonpath.go` (`itemJSONFields`, `activityJSONFields`, `linkJSONFields`, `projectInitJSONFields`). The hidden `gh tasks check-json-schema` command renders every catalog as a markdown table in canonical order so the field tables on this page can be diff-checked against the in-source catalog without copy-pasting:

```bash
go run . check-json-schema
```

Add new catalogs to `jsonSchemaCatalogs()` in `cmd/check_json_schema.go` so they show up here.

## Examples

### Pipe to jq

```bash
# Find all open-state items, then print just their titles
gh tasks today --json title,state --jq '.[] | select(.state=="OPEN") | .title'
```

### Filter standup output by category

```bash
# Just the merged PRs from yesterday
gh tasks standup --json number,title,category \
  --jq '.[] | select(.category=="merged") | "\(.number): \(.title)"'
```

### Capture the created Issue ID for downstream commands

```bash
issue_id=$(gh tasks add "Bug: 404 on /api/foo" --json id --jq '.[0].id')
echo "$issue_id"
# I_kwDOSQTNsM8AAAAB...
```

### Verify a closed Issue's state programmatically

```bash
gh tasks done 42 --json state,url --jq '.[0]'
# {
#   "state": "CLOSED",
#   "url": "https://github.com/owner/repo/issues/42"
# }
```

### Pull every untriaged Issue, regardless of `--limit`

```bash
gh tasks triage --paginate --json id,title --jq 'length'
```

### Read the link target after binding a PR

```bash
gh tasks link 12 42 --json linkType,linkedTo \
  --jq '.[0] | "\(.linkType) → #\(.linkedTo.number)"'
# closesAdded → #42
```

### Combine with `yq` for YAML output

```bash
# `yq -P` reads JSON from stdin and prints YAML
gh tasks list --json id,title | yq -P
```
