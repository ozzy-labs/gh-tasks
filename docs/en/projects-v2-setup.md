# Projects v2 field setup

The `org` / `user` scopes use GitHub Projects v2 as backing storage. This page lists the minimum fields required.

## Personal (`user` scope)

Define these fields on a personal Project v2:

| Field | Type | Purpose |
| --- | --- | --- |
| Title | (built-in) | One-line item summary |
| Status | Single select | `Triage` / `Todo` / `In Progress` / `Done` (`Triage` flags items for `gh tasks triage`; items with `Status` unset also count as untriaged) |
| Iteration | Iteration | Weekly / sprint planning (target of `gh tasks plan`) |

## Team (`org` scope)

An organization Project v2 needs the personal set plus:

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

`gh tasks projects init` consumes the YAML directly and creates the
Project + custom fields in one shot:

```bash
gh tasks projects init --template user --title "gh-tasks personal"
gh tasks projects init --template org --owner <org> --title "team board"
gh tasks projects init packages/templates/projects-v2/user.yaml --title "from path"
```

Use `--dry-run` to preview the field set. The hand-rolled
`gh project field-create` fallback is documented in
[`packages/templates/README.md`](../../packages/templates/README.md).

## Per-scope mapping

| scope | Project | Required fields |
| --- | --- | --- |
| `repo` | (unused, Milestones instead) | — |
| `org` | Organization Project | team set (above) |
| `user` | Personal Project | personal set (above) |

## Related

- [concepts.md](./concepts.md): scope / iteration terminology
- [cli-reference.md](./cli-reference.md): how `gh tasks plan` etc. behave
