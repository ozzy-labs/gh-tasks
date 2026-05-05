# Scope auto-detection

Every `gh-tasks` command can omit `--scope`. It is resolved in this order:

## Order

1. **`--scope` flag** (explicit)
   - Both `--scope repo` and `--scope=repo` forms supported
   - Same for `--scope org` / `--scope user`
2. **git remote `origin`** present → `repo`
3. **`~/.config/ozzylabs/gh-tasks.toml` `default_scope`** (`repo` / `org` / `user`)
4. **Fallback** → `user`

## Invalid values

Specifying anything other than `repo` / `org` / `user` for `--scope` throws `ScopeError` and exits 2.

```bash
$ gh tasks add 'foo' --scope=global
ScopeError: invalid --scope value: 'global' (valid: repo | org | user)
```

## Multiple `--scope` flags

When `--scope` appears multiple times in argv, the **first occurrence** wins.

## Implementation

`internal/scope/scope.go` `Detect`. The `HasGitRemote` field on `DetectOptions` is injectable so tests are deterministic. See `scope_test.go` (6 subtests under `TestDetect`) in the same directory.

## Related

- [concepts.md](../concepts.md): scope terminology
- [cli.md](./cli.md): per-command `--scope` handling
