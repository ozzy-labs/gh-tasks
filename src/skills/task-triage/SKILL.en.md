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

1. Fetch the untriaged list via `gh tasks triage --scope <scope> --limit <N>`
2. For each item, propose:
   - **label**: Conventional-style (`feat` / `fix` / `docs` / etc.)
   - **scope** routing: personal / repo / org
   - **status**: keep / close / merge into another Issue
3. Apply the decisions through the `gh tasks triage` interactive flow
4. Report remaining count and the next recommended triage cadence

## Fallback

- Backlog too large: shrink `--limit` and run in batches
- Ambiguous item: ask the user directly

> v0.1.0 stub. Awaits the `gh tasks triage` CLI implementation.
