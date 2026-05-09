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
```

`--json=` (empty value) lists the available fields on stderr and exits 1. Pass one or more field names, comma-separated, to receive a JSON array on stdout. `--jq <query>` applies a [gojq](https://github.com/itchyny/gojq)-compatible filter to the array; values are emitted one per line, with two-space indent for objects.

## Supported commands (Phase 1)

| Command | Catalog |
| --- | --- |
| [`list`](./cli.md#gh-tasks-list) / [`today`](./cli.md#gh-tasks-today--period-dailyweeklysprint) | item |
| [`triage`](./cli.md#gh-tasks-triage) | item |
| [`plan`](./cli.md#gh-tasks-plan--period-dailyweeklysprint) (preview only) | item |
| [`standup`](./cli.md#gh-tasks-standup---mine---since-iso8601) / [`review`](./cli.md#gh-tasks-review--period-dailyweeklysprint) | activity (= item + `category`) |
| [`add`](./cli.md#gh-tasks-add-title) | item |

The mutation-side path (`done`, `link`, `plan --write`, `projects init` mutations) does not yet support `--json`. Combining `--json` with `plan --write` returns a localized error pointing this out; the support is planned for Phase 2.

## Field catalog

### `item` (list / today / triage / plan-preview / add)

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | GraphQL global ID of the Issue / PR / Project item |
| `number` | int | Issue / PR / Project item number. `0` for draft items |
| `title` | string | Title |
| `type` | string | `"ISSUE"` \| `"PULL_REQUEST"` \| `"DRAFT_ISSUE"` |
| `updatedAt` | string \| null | Last-update timestamp (RFC 3339). Null for items where the source response does not carry it (e.g. `add`'s mutation reply) |
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

## Behaviour and contract

### Stream separation

- **stdout** = data only. Either the JSON array or `--jq`-filtered values
- **stderr** = warnings, localized errors, the field catalog (when `--json=` is passed)
- Errors leave **stdout empty** so `... | jq` does not see partial JSON
- Exit code is non-zero on validation / runtime failure

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
# Find all in-progress items, then print just their titles
gh tasks today --json title,type --jq '.[] | select(.type=="ISSUE") | .title'
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

### Combine with `yq` for YAML output

```bash
# `yq -P` reads JSON from stdin and prints YAML
gh tasks list --json id,title | yq -P
```
