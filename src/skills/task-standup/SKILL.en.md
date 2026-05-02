---
name: task-standup
description: Generate a standup summary of recent activity. Invokes `gh tasks standup [--mine]` to format team or personal activity for sharing.
allowed-tools: Bash(gh:*)
locale: en
---

# task-standup - Activity summary

Format recent team or personal activity for a standup post.

## Inputs

- **--mine** (optional): limit to the current user's activity
- **--since** (optional): ISO 8601 start time (default: last 24h)
- **--scope** (optional): `repo` / `org` / `user`

## Steps

1. Run `gh tasks standup [--mine] [--since ...] [--scope ...]`
2. Format the output into Yesterday / Today / Blockers
3. Present as Markdown ready to paste into Slack or meeting notes

## Fallback

- `--since` too far back, output overwhelming: suggest a shorter window
- `--mine` returns nothing: suggest extending the window or switching scope

> v0.1.0 stub. Awaits the `gh tasks standup` CLI implementation.
