---
name: task-add
description: Capture a task from conversation context. Creates a GitHub Issue (repo scope) or Project draft item (org/user scope) via `gh tasks add`.
allowed-tools: Bash(gh:*)
locale: en
---

# task-add - Capture a task from conversation

Persist an item that emerged in the conversation to the right scope.

## Inputs

- **title** (required): one-line summary of the task
- **--scope** (optional): one of `repo` / `org` / `user`. Defaults to git remote detection
- **--repo** (optional): the target repository (`<owner>/<name>`) when scope is `repo`

## Steps

1. Extract body / acceptance criteria / related PR links from the conversation context
2. Invoke `gh tasks add <title> [--scope ...] [--repo ...]`
3. Surface the resulting Issue / draft item URL back to the user

## Fallback

- `gh auth login` not run: surface the auth flow and stop
- scope auto-detected to `repo` but no git remote present: suggest `--scope user`
