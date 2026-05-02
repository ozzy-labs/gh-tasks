---
name: task-plan
description: Run a daily / weekly / iteration plan. Invokes `gh tasks plan` to organize plan items in the relevant scope's Milestone (repo) or Iteration (org/user).
allowed-tools: Bash(gh:*)
locale: en
---

# task-plan - Plan a period

Build a daily / weekly / iteration plan and commit the chosen items into the right scope's backlog.

## Inputs

- **--period** (optional): one of `daily` / `weekly` / `sprint`. Defaults to `daily`
- **--scope** (optional): `repo` / `org` / `user`. Defaults to git remote detection

## Steps

1. Pull and summarize recent open / unfinished Issues and drafts
2. Confirm priorities with the user
3. Commit the plan via `gh tasks plan [--period ...] [--scope ...]` (repo scope updates a Milestone; org/user scope updates a Project v2 Iteration)
4. Present the finalized plan back to the user

## Fallback

- Too many plan candidates: ask the user for a narrowing condition (label / assignee)
- Iteration field undefined: surface the setup steps from `docs/en/projects-v2-setup.md`

> v0.1.0 stub. Awaits the `gh tasks plan` CLI implementation.
