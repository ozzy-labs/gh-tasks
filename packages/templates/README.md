# @ozzylabs/gh-tasks templates

Reusable assets that `gh-tasks` ships alongside the CLI.

## Layout

```text
packages/templates/
└── projects-v2/
    ├── user.yaml   # Personal (user) scope field set
    └── org.yaml    # Team (org) scope field set (extends user)
```

## Projects v2 field templates

YAML descriptors for the custom fields that `gh-tasks` expects on a
Projects v2 board. They are the source of truth for the field schemas
referenced by [`docs/{ja,en}/projects-v2-setup.md`](../../docs/en/projects-v2-setup.md).

### Schema

```yaml
name: <project title>           # informational, not applied by gh CLI
description: <project summary>  # informational
fields:
  - name: <field name>
    type: text|single_select|date|number|iteration|repository
    options: [<opt1>, <opt2>, ...]   # required for single_select
```

`type` mirrors the GraphQL [`ProjectV2FieldType`](https://docs.github.com/en/graphql/reference/enums#projectv2fieldtype)
enum (lower-cased). `iteration` and `repository` are GitHub-managed
field types and have no configurable options here; their cadence
(iteration duration / start date) is set in the Projects UI under
*Settings -> Iteration*.

### Available templates

| File | Scope | Fields |
| --- | --- | --- |
| `projects-v2/user.yaml` | Personal Project (user scope) | Status, Iteration |
| `projects-v2/org.yaml` | Team Project (org scope) | Status, Iteration, Repository, Project |

### Applying a template

> **Heads up:** the GitHub CLI (`gh` 2.x) does **not** implement
> `gh project create --from-yaml`. These YAML files are the
> authoritative spec for the field set; the snippets below apply them
> through the `gh project field-create` subcommand. A future
> `gh tasks projects init` helper (tracked in
> [#65](https://github.com/ozzy-labs/gh-tasks/issues/65)) will consume
> the same YAML directly.

#### Personal (user) scope

```bash
# 1. Create the empty project (Iteration field is added automatically by GitHub).
PROJECT_NUMBER=$(gh project create \
  --owner "@me" \
  --title "gh-tasks personal" \
  --format json --jq .number)

# 2. Add Status with the canonical option set.
gh project field-create "$PROJECT_NUMBER" \
  --owner "@me" \
  --name "Status" \
  --data-type SINGLE_SELECT \
  --single-select-options "Todo,In Progress,Done"

# 3. Configure Iteration cadence in the GitHub UI
#    (Settings -> Iteration). The CLI cannot set duration/start.
```

#### Team (org) scope

```bash
ORG="ozzy-labs"

PROJECT_NUMBER=$(gh project create \
  --owner "$ORG" \
  --title "OzzyLabs Platform" \
  --format json --jq .number)

gh project field-create "$PROJECT_NUMBER" --owner "$ORG" \
  --name "Status" --data-type SINGLE_SELECT \
  --single-select-options "Todo,In Progress,Done"

gh project field-create "$PROJECT_NUMBER" --owner "$ORG" \
  --name "Project" --data-type SINGLE_SELECT \
  --single-select-options "Platform,Docs,Infra"

# Iteration and Repository fields are GitHub-managed types: add them via
# the Projects UI (Settings -> Custom fields -> + New field).
```

### Validation

YAML files in this directory are linted with
[`yamllint`](https://yamllint.readthedocs.io/) via
`pnpm run lint:yaml`. They are intentionally lightweight and are not
consumed by any tool yet, so there is no schema validator beyond
yamllint's structural checks.

## See also

- [`docs/en/projects-v2-setup.md`](../../docs/en/projects-v2-setup.md) /
  [`docs/ja/projects-v2-setup.md`](../../docs/ja/projects-v2-setup.md)
- [GraphQL `ProjectV2FieldType` reference](https://docs.github.com/en/graphql/reference/enums#projectv2fieldtype)
