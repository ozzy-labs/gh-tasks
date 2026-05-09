---
name: task-plan
description: Run a daily / weekly / iteration plan. Invokes `gh tasks plan` to organize plan items in the relevant scope's Milestone (repo) or Iteration (org/user).
allowed-tools: Bash(gh:*)
locale: en
---

# task-plan - Plan a period

Build a daily / weekly / iteration plan and commit the chosen items into the right scope's backlog.

## Inputs

- **--period** (optional): one of `daily` / `weekly` / `sprint`. Defaults to `weekly`
- **--scope** (optional): `repo` / `org` / `user`. Defaults to git remote detection
- **--write** (optional): apply changes. Without it, `plan` only previews candidates and never creates milestones or updates iterations

## Steps

1. Pull and summarize recent open / unfinished Issues and drafts
2. Run `gh tasks plan [--period ...] [--scope ...]` (preview by default) and confirm priorities with the user
3. Once confirmed, re-run with `--write` to apply changes (repo scope creates or reuses a Milestone and binds in-range Issues; org/user scope updates a Project v2 Iteration)
4. Present the finalized plan back to the user

## Fallback

- Too many plan candidates: ask the user for a narrowing condition (label / assignee)
- Iteration field undefined: surface the setup steps from <https://github.com/ozzy-labs/gh-tasks/blob/main/docs/manual/en/guides/projects-v2-setup.md>
- Issue already bound to another Milestone: the CLI skips and reports it; re-run without `--write` if you want to inspect the plan again
