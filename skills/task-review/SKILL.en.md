---
name: task-review
description: Generate a retrospective summary. Invokes `gh tasks review --period daily|weekly|sprint` to summarize Issues closed, PRs merged, and Project items completed within the period.
allowed-tools: Bash(gh:*)
locale: en
---

# task-review - Retrospective

Generate a daily / weekly / sprint retrospective summary.

## Inputs

- **--period** (optional): `daily` / `weekly` / `sprint`. Defaults to `weekly`
- **--scope** (optional): `repo` / `org` / `user`

## Steps

1. Run `gh tasks review --period <period> --scope <scope>`
2. Format the result (Done / In progress / Blockers) as Markdown
3. Ask the user what was learned in the period and what carries over

## Fallback

- Empty result: re-confirm period / scope with the user
- API rate limit: shrink the period and retry
