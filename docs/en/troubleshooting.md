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

## `--scope user` / `--scope org` returns "not implemented"

**Cause**: v0.1.0 only implements `repo` scope. `org` / `user` are gated on the projectId resolver landing.

**Fix**: Use `--scope repo` for now or wait for the follow-up release. Track progress via handbook ADR-0022 and related PRs.

## Semantic clash with `gh agent-task`

GitHub's official `gh agent-task` (preview) is close in name to this extension's `gh tasks`. handbook ADR-0022 marked this as something to watch. If it becomes a problem, a repo-internal ADR will decide the response.

## API rate limit

**Cause**: bursting GraphQL requests (GitHub allows 5000/hr authenticated).

**Fix**:

- Wait a few minutes and retry
- For heavy enumerations, use `--limit` (planned) to cap result size
- In CI, ensure `GITHUB_TOKEN` is exported so requests run on the authenticated quota

## `gh extension install` fails

**Cause**: no platform binary in the GitHub Release.

**Fix**: confirm your platform is in the 5 published targets (darwin x86_64 / arm64, linux x86_64 / arm64, windows x86_64; see [repo-internal ADR-0001](../adr/0001-use-bun-compile-for-binary.md)). On unsupported platforms, run from source: `git clone` + `bun run`.

## Related

- [installation.md](./installation.md)
- [scope-detection.md](./scope-detection.md)
