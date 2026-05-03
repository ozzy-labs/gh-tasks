# Projects v2 field setup

The `org` / `user` scopes use GitHub Projects v2 as backing storage. This page lists the minimum fields required.

## Personal (`user` scope)

Define these fields on a personal Project v2:

| Field | Type | Purpose |
| --- | --- | --- |
| Title | (built-in) | One-line item summary |
| Status | Single select | `Todo` / `In Progress` / `Done` |
| Iteration | Iteration | Weekly / sprint planning (target of `gh tasks plan`) |

## Team (`org` scope)

OzzyLabs Platform Project v2 needs the personal set plus:

| Field | Type | Purpose |
| --- | --- | --- |
| Repository | Repository | Owning repo when coordinating across projects |
| Project | Single select | Cross-project identifier above individual repos |

## Setup steps

Create a Project in the GitHub UI and add the fields above under Settings → Custom fields. The `gh project` CLI works too, but the Iteration field is easier in the UI.

### YAML templates

The field-set source of truth ships with the repository under
`packages/templates/projects-v2/`:

| File | Scope |
| --- | --- |
| [`packages/templates/projects-v2/user.yaml`](../../packages/templates/projects-v2/user.yaml) | Personal / `user` scope (Status, Iteration) |
| [`packages/templates/projects-v2/org.yaml`](../../packages/templates/projects-v2/org.yaml) | Team / `org` scope (user set + Repository, Project) |

The GitHub CLI does not currently implement `gh project create --from-yaml`,
so the templates are applied through a sequence of
`gh project field-create` calls. See
[`packages/templates/README.md`](../../packages/templates/README.md) for
ready-to-run command snippets.

## Per-scope mapping

| scope | Project | Required fields |
| --- | --- | --- |
| `repo` | (unused, Milestones instead) | — |
| `org` | OzzyLabs Platform Project | team set (above) |
| `user` | Personal Project | personal set (above) |

## Related

- [handbook ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md): adopting Projects v2
- [concepts.md](./concepts.md): scope / iteration terminology
- [cli-reference.md](./cli-reference.md): how `gh tasks plan` etc. behave
