# Design documentation (English mirror)

Design documentation for `gh-tasks`. This is the **English mirror**; the SSOT lives in `docs/ja/` (repo-internal ADR-0002).

## Structure

| File | Purpose |
| --- | --- |
| concepts.md | Terminology (scope / item / iteration / personal vs team) |
| installation.md | `gh extension install ozzy-labs/gh-tasks` + initial setup |
| scope-detection.md | `--scope` auto-detection and config precedence |
| projects-v2-setup.md | Field definitions for personal and team usage |
| cli-reference.md | All commands and flags |
| recipes/{agent}.md | Usage examples per agent |
| troubleshooting.md | `gh agent-task` collision recovery, auth errors, PATH issues |

> This directory is a v0.1.0 stub. Content is filled in alongside implementation.

## Design rationale

- [handbook ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md): the decision to create this repo
- [handbook ADR-0018](https://github.com/ozzy-labs/handbook/blob/main/adr/0018-agent-adapter-architecture.md): the 4-agent adapter mechanism
- [handbook ADR-0016](https://github.com/ozzy-labs/handbook/blob/main/adr/0016-create-skills-repo.md): skill SSOT in a dedicated repo
- [handbook reviews/2026-04-30-gh-tasks-design.md](https://github.com/ozzy-labs/handbook/blob/main/reviews/2026-04-30-gh-tasks-design.md): v0.1.0 implementation specification
- [docs/adr/](../adr/): repo-internal ADRs (Bun --compile / i18n SSOT / Octokit / skill frontmatter)
