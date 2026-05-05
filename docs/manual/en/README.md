# User Manual (English, SSOT)

User manual for `gh-tasks`. This directory is the **SSOT**; `docs/manual/ja/` is the Japanese mirror (repo-internal [ADR-0005](../../adr/0005-i18n-reader-based-ssot.md)). New content lands here first; the Japanese mirror is updated to match.

## Structure (Diátaxis simplified)

- [concepts.md](./concepts.md) — Explanation: terminology (scope / item / iteration / personal vs team)
- [guides/](./guides/) — How-to guides
  - [installation.md](./guides/installation.md) — `gh extension install ozzy-labs/gh-tasks` and initial setup
  - [projects-v2-setup.md](./guides/projects-v2-setup.md) — Field definitions for personal and team use
  - [troubleshooting.md](./guides/troubleshooting.md) — Auth errors, `--repo` resolution, `gh agent-task` clash, API rate limit
- [reference/](./reference/) — Reference
  - [cli.md](./reference/cli.md) — All commands and flags
  - [scope-detection.md](./reference/scope-detection.md) — `--scope` auto-detection and precedence
  - [locale-detection.md](./reference/locale-detection.md) — `--lang` / `LC_ALL` / `LANG` output language precedence
- [recipes/](./recipes/) — Tutorials per agent
  - [claude-code.md](./recipes/claude-code.md) — Loading skills and use cases on Claude Code
  - [codex-cli.md](./recipes/codex-cli.md) — Loading skills and use cases on Codex CLI
  - [gemini-cli.md](./recipes/gemini-cli.md) — Loading skills and use cases on Gemini CLI
  - [copilot.md](./recipes/copilot.md) — Loading skills and use cases on GitHub Copilot

## Design rationale

- [docs/adr/](../../adr/) — repo-internal ADRs (Go migration / i18n / GraphQL via go-gh + genqlient / skill frontmatter)
- [docs/design/](../../design/) — repo-internal living design documents
