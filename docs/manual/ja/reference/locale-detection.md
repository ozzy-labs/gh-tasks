# locale 自動判定

`gh-tasks` の出力言語(`ja` / `en`)は以下の優先順で解決される。

## 優先順

1. **`--lang` フラグ**(明示)
   - `--lang ja` / `--lang=ja` 両形式をサポート
   - 同様に `--lang en` / `--lang=en`
2. **`~/.config/ozzylabs/gh-tasks.toml` の `lang`**(`ja` / `en`)
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

`internal/i18n/i18n.go` の `ResolveLocale(argv, env, config)` 関数。`env` / `config` は引数で注入可能でテストは決定論的。`config` は `internal/config` の `Load()` 経由で `~/.config/ozzylabs/gh-tasks.toml` から読み込まれる。

## SSOT 言語と出力言語

`gh-tasks` のメッセージは **英語(en)を SSOT** とする(repo-internal [ADR-0005](../../../adr/0005-i18n-reader-based-ssot.md)、[ADR-0002](../../../adr/0002-i18n-japanese-ssot.md) を Superseded)。実行時の `t(locale, key)` はまず指定 locale を引き、見つからなければ `en` にフォールバックし、両方欠けていればキー文字列そのものを返す(デバッグ用)。フォールバックは非対称で、バックストップは常に `en` テーブルのみ。`en` キーに ja 翻訳が存在しない場合は ja 出力にも英語がそのまま出るため、新規キーは en に書いてから ja に翻訳をペアで追加すること。ハードコードされた非 ASCII 文字列リテラルは禁止で、`gh tasks check-i18n`(`internal/i18ncheck`、`go/parser` ベース。lefthook `pre-commit` hook と CI で実行)が検知して reject する。

## 関連

- [scope-detection.md](./scope-detection.md): 同様の優先順設計(`--scope` フラグ)
- [cli.md](./cli.md): `--lang` フラグ
- [docs/adr/0005-i18n-reader-based-ssot.md](../../../adr/0005-i18n-reader-based-ssot.md): 現行 i18n 方針(en SSOT、ADR-0002 を Superseded)
