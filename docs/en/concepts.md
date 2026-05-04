# Concepts

Glossary for understanding what `gh-tasks` operates on.

## scope

Every `gh-tasks` command runs in one of three **scopes**:

| Scope | Use case | Backing storage |
| --- | --- | --- |
| `repo` | Single-repo implementation work | GitHub Issues + Milestones |
| `org` | Cross-project coordination | Organization Project v2 |
| `user` | Personal todos / daily plans | Personal Project v2 |

Scope is either passed via `--scope` or auto-detected. See [scope-detection.md](./scope-detection.md).

## item

The unit a task — backing storage differs per scope:

- `repo` scope → GitHub Issue
- `org` / `user` scope → Projects v2 draft item, or a Project item linked to an Issue

The CLI / skills abstract over `item` and dispatch to the scope-specific query.

## iteration

The "planning period" unit per scope:

- `repo` scope → GitHub Milestone
- `org` / `user` scope → Project v2 Iteration field

`gh tasks plan` / `gh tasks review` accept `--period daily|weekly|sprint` to address iterations uniformly.

## personal vs team

`user` scope is the personal project (a todo only you see); `org` scope coordinates an organization-wide Project v2. The same commands (`gh tasks add` etc.) work for both — this unified abstraction is the core design of the CLI.

## Related ADRs

- [docs/adr/0001](../adr/0001-use-bun-compile-for-binary.md): adopting Bun --compile
- [docs/adr/0003](../adr/0003-graphql-via-octokit.md): GraphQL via Octokit
