# Design documentation (English mirror)

Design documentation for `gh-tasks`. This is the **English mirror**; the SSOT lives in `docs/ja/` (repo-internal ADR-0002).

## Structure

| File | Purpose |
| --- | --- |
| [concepts.md](./concepts.md) | Terminology (scope / item / iteration / personal vs team) |
| [installation.md](./installation.md) | `gh extension install ozzy-labs/gh-tasks` + initial setup |
| [scope-detection.md](./scope-detection.md) | `--scope` auto-detection and precedence |
| [locale-detection.md](./locale-detection.md) | `--lang` / `LC_ALL` / `LANG` output language precedence |
| [projects-v2-setup.md](./projects-v2-setup.md) | Field definitions for personal and team use |
| [cli-reference.md](./cli-reference.md) | All commands and flags |
| [troubleshooting.md](./troubleshooting.md) | Auth errors, `--repo` resolution, `gh agent-task` clash, API rate limit |
| recipes/{agent}.md | Usage examples per agent (follow-up to v0.1.0) |

> v0.1.0 initial version. Content evolves alongside implementation; recipes land after v0.1.0.

## Design rationale

- [handbook ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md): the decision to create this repo
- [handbook ADR-0018](https://github.com/ozzy-labs/handbook/blob/main/adr/0018-agent-adapter-architecture.md): the 4-agent adapter mechanism
- [handbook ADR-0016](https://github.com/ozzy-labs/handbook/blob/main/adr/0016-create-skills-repo.md): skill SSOT in a dedicated repo
- [handbook reviews/2026-04-30-gh-tasks-design.md](https://github.com/ozzy-labs/handbook/blob/main/reviews/2026-04-30-gh-tasks-design.md): v0.1.0 implementation specification
- [docs/adr/](../adr/): repo-internal ADRs (Bun --compile / i18n SSOT / Octokit / skill frontmatter)
