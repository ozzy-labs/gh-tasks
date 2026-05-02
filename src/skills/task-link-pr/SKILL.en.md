---
name: task-link-pr
description: Link a PR to its tracking Issue / Project item. Invokes `gh tasks link <pr> <task>` to establish the GitHub relation.
allowed-tools: Bash(gh:*)
locale: en
---

# task-link-pr - Link PR to a tracking item

Establish a relation between a PR and its tracking Issue or Project draft item.

## Inputs

- **pr** (required): PR number / URL
- **task** (required): Issue number / draft item ID

## Steps

1. Run `gh tasks link <pr> <task>` (repo scope appends `Closes #N` to the PR body; org/user scope updates the Project v2 relation field)
2. Verify the resulting relation URL from the output
3. Report success back to the user

## Fallback

- PR / task live in different scopes: confirm whether cross-scope linking is allowed
- Already linked: surface the existing link and exit

> v0.1.0 stub. Awaits the `gh tasks link` CLI implementation.
