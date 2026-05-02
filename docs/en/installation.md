# Installation

## Prerequisites

- `gh` CLI installed and `gh auth login` completed
- macOS / Linux / Windows (any platform `gh extension` supports)

## Install

```bash
gh extension install ozzy-labs/gh-tasks
```

Optional shorter alias:

```bash
gh alias set task tasks
```

## Authentication

`gh-tasks` inherits the token obtained by `gh auth login` via the `GH_TOKEN` environment variable (repo-internal ADR-0003). No separate auth flow.

In automated environments (e.g. GitHub Actions), `GITHUB_TOKEN` works as well.

## Config file (planned for v0.2.0)

`~/.config/ozzylabs/gh-tasks.toml`:

```toml
default_scope = "repo"
```

Not yet wired in v0.1.0. Use `--scope` explicitly or rely on git-remote auto-detection (see [scope-detection.md](./scope-detection.md)).

## Projects v2 setup

To use `org` / `user` scope, define the required Project v2 fields. See [projects-v2-setup.md](./projects-v2-setup.md).

For `repo` scope only, no extra setup — GitHub Issues / Milestones are used directly.

## Smoke test

```bash
gh tasks --version
gh tasks add 'first task' --scope=repo --repo=<owner>/<name>
```

## Related

- [scope-detection.md](./scope-detection.md)
- [troubleshooting.md](./troubleshooting.md)
