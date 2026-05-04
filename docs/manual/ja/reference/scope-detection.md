# scope 自動判定

`gh-tasks` のすべてのコマンドは `--scope` を省略可能で、以下の優先順で解決される。

## 優先順

1. **`--scope` フラグ**(明示)
   - `--scope repo` / `--scope=repo` 両形式をサポート
   - 同様に `--scope org` / `--scope user`
2. **git remote `origin`** が存在 → `repo`
3. **`~/.config/ozzylabs/gh-tasks.toml` の `default_scope`**(`repo` / `org` / `user`)
4. **fallback** → `user`

## 不正値の扱い

`--scope` に `repo` / `org` / `user` 以外を指定すると `ScopeError` を throw して exit 2。

```bash
$ gh tasks add 'foo' --scope=global
ScopeError: 不正な --scope 値: 'global' (有効値: repo | org | user)
```

## 複数指定時

argv 内で `--scope` が複数指定された場合、**最初に見つかったもの**を採用する。

## 実装

`packages/gh-tasks/src/lib/scope.ts` の `detectScope` 関数。`hasGitRemote` 関数を注入可能で、テストでは決定論的に挙動を切り替える。テストは同階層の `scope.test.ts`(8 件)。

## 関連

- [concepts.md](../concepts.md): scope の用語
- [cli.md](./cli.md): 各コマンドの `--scope` 受付
