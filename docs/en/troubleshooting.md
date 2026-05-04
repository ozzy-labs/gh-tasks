# Troubleshooting

## Auth error: `AuthError: GH_TOKEN / GITHUB_TOKEN not set`

**Cause**: `gh auth login` not run, or `GITHUB_TOKEN` not propagated to the GitHub Actions runner.

**Fix**:

```bash
gh auth login
# or
export GH_TOKEN=<your-token>
```

## `--repo` resolution failed: `RepoError: neither --repo flag nor git remote origin resolved`

**Cause**: Current directory is not a git repo, or `origin` remote is missing.

**Fix**:

```bash
gh tasks add 'title' --repo=<owner>/<name>
# or set the git remote
git remote add origin git@github.com:<owner>/<name>.git
```

## `--scope org|user` reports "Project の指定が必要です"

**Cause**: `org` / `user` scope needs a Projects v2 reference but neither `--project` nor the matching config key (`org_project` / `user_project`) is set.

**Fix**:

```bash
gh tasks list --scope=user --project=<owner>/<number>
# or persist the default in ~/.config/ozzylabs/gh-tasks.toml
echo 'user_project = "<owner>/<number>"' >> ~/.config/ozzylabs/gh-tasks.toml
```

See [installation.md](./installation.md) for the full config schema.

## Semantic clash with `gh agent-task`

GitHub's official `gh agent-task` (preview) is close in name to this extension's `gh tasks`. This is a known watch item; if it becomes a problem, a repo-internal ADR will decide the response.

## API rate limit

**Cause**: bursting GraphQL requests (GitHub allows 5000/hr authenticated).

**Fix**:

- Wait a few minutes and retry
- For heavy enumerations, use `--limit` to cap result size (`gh tasks list` / `gh tasks triage`)
- In CI, ensure `GITHUB_TOKEN` is exported so requests run on the authenticated quota

## `gh extension install` fails

**Cause**: no platform binary in the GitHub Release.

**Fix**: confirm your platform is in the 5 published targets (darwin x86_64 / arm64, linux x86_64 / arm64, windows x86_64; see [repo-internal ADR-0001](../adr/0001-use-bun-compile-for-binary.md)). On unsupported platforms, run from source: `git clone` + `bun run`.

## Related

- [installation.md](./installation.md)
- [scope-detection.md](./scope-detection.md)
