---
name: task-triage
description: Triage untriaged Issues / Project draft items. Invokes `gh tasks triage` to assist with labeling, scope routing, and close decisions.
allowed-tools: Bash(gh:*)
locale: en
---

# task-triage - Inbox triage

Sweep through untriaged Issues / draft items in batch.

## Inputs

- **--scope** (optional): `repo` / `org` / `user`. Defaults to git remote detection
- **--limit** (optional): batch size (default 20)

## Steps

1. Fetch the untriaged list via `gh tasks triage --scope <scope> --limit <N>` (read-only)
2. For each item, propose:
   - **label**: Conventional-style (`feat` / `fix` / `docs` / etc.)
   - **scope** routing: personal / repo / org
   - **status**: keep / close / merge into another Issue
3. After user approval, apply the decisions via `gh issue edit` / `gh issue close` etc.
4. Report remaining count and the next recommended triage cadence

## Fallback

- Backlog too large: shrink `--limit` and run in batches
- Ambiguous item: ask the user directly
