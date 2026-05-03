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

`packages/gh-tasks/src/lib/scope.ts` `detectScope`. The `hasGitRemote` function is injectable so tests are deterministic. See `scope.test.ts` (8 tests) in the same directory.

## Related

- [concepts.md](./concepts.md): scope terminology
- [cli-reference.md](./cli-reference.md): per-command `--scope` handling
