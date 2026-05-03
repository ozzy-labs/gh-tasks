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

## Config file

`~/.config/ozzylabs/gh-tasks.toml` (honors `$XDG_CONFIG_HOME`):

```toml
lang = "ja"
default_scope = "repo"
```

Optional keys:

- `lang`: `ja` / `en`. Default output language (see [locale-detection.md](./locale-detection.md))
- `default_scope`: `repo` / `org` / `user`. Default scope (see [scope-detection.md](./scope-detection.md))

A missing config file is silently ignored — flags, env vars, and auto-detection still apply. A malformed TOML or invalid value exits with a clear error message.

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
