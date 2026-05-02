# locale 自動判定

`gh-tasks` の出力言語(`ja` / `en`)は以下の優先順で解決される。

## 優先順

1. **`--lang` フラグ**(明示)
   - `--lang ja` / `--lang=ja` 両形式をサポート
   - 同様に `--lang en` / `--lang=en`
2. **`~/.config/ozzylabs/gh-tasks.toml` の `lang`**(v0.2.0 予定、v0.1.0 では skip)
3. **`LC_ALL` 環境変数** が `ja` で始まる(大小無視)→ `ja`
4. **`LANG` 環境変数** が `ja` で始まる(大小無視)→ `ja`
5. **fallback** → `en`

`LC_ALL` は `LANG` より優先される(POSIX 標準)。`LC_ALL=en_US` + `LANG=ja_JP` の環境では `en` が選択される。

## 不正値の扱い

`--lang` に `ja` / `en` 以外を指定した場合、その flag は **無視** されて env / fallback に進む。エラーで停止しない設計。

```bash
$ gh tasks add 'foo' --scope=repo --repo=owner/name --lang=fr
# `--lang=fr` は無視され、`LANG` または default の `en` で出力される
```

複数の `--lang` フラグがある場合、**最初に見つかったもの**を採用する。

## 実装

`packages/gh-tasks/src/i18n/index.ts` の `resolveLocale(argv, env?)` 関数。
`env` は引数で注入可能でテストは決定論的。

## SSOT 言語と出力言語

`gh-tasks` のメッセージは ja を SSOT(repo-internal [ADR-0002](../adr/0002-i18n-japanese-ssot.md))としており、`en` キーが欠けている場合は ja の値が出力される。逆に `ja` キーが欠けていて `en` キーが存在する場合は `t()` のフォールバックで en が出力される。両方欠けている場合はキー文字列そのものが出力される(デバッグ用)。

## 関連

- [scope-detection.md](./scope-detection.md): 同様の優先順設計(`--scope` フラグ)
- [cli-reference.md](./cli-reference.md): `--lang` フラグ
- [docs/adr/0002-i18n-japanese-ssot.md](../adr/0002-i18n-japanese-ssot.md): ja SSOT 方針
