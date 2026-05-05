---
name: task-link-pr
description: 'Link a PR to its tracking Issue / Project item. Invokes `gh tasks link <pr> <task>` — appends `Closes #N` to the PR body for repo scope, or binds both the PR and the Issue to the same Project v2 for org/user scope.'
allowed-tools: Bash(gh:*)
locale: en
---

# task-link-pr - Link PR to a tracking item

Establish a relation between a PR and its tracking Issue or Project draft item.

## Inputs

- **pr** (required): PR number / URL
- **task** (required): Issue number / draft item ID

## Steps

1. Run `gh tasks link <pr> <task>` (repo scope appends `Closes #N` to the PR body; org/user scope binds both the PR and the Issue to the same Project v2)
2. Verify the resulting relation URL from the output
3. Report success back to the user

## Fallback

- PR / task live in different scopes: confirm whether cross-scope linking is allowed
- Already linked: surface the existing link and exit
