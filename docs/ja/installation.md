# インストール

## 前提

- `gh` CLI がインストール済で `gh auth login` 完了済
- macOS / Linux / Windows のいずれか(`gh extension` 対応プラットフォーム)

## インストール手順

```bash
gh extension install ozzy-labs/gh-tasks
```

短縮 alias を設定する場合(任意):

```bash
gh alias set task tasks
```

## 認証

`gh-tasks` は `gh auth login` で取得済のトークンを `GH_TOKEN` 環境変数経由で継承する(repo-internal ADR-0003)。追加の認証手順は不要。

GitHub Actions 等の自動環境では `GITHUB_TOKEN` env でも動作する。

## 設定ファイル

`~/.config/ozzylabs/gh-tasks.toml`(`$XDG_CONFIG_HOME` を尊重):

```toml
lang = "ja"
default_scope = "repo"
org_project = "ozzy-labs/5"
user_project = "ozzy-3/2"
```

省略可能なキー:

- `lang`: `ja` / `en`。出力言語の既定値([locale-detection.md](./locale-detection.md))
- `default_scope`: `repo` / `org` / `user`。スコープの既定値([scope-detection.md](./scope-detection.md))
- `org_project`: `<owner>/<number>` 形式。`--scope=org` の既定 Project v2
- `user_project`: `<owner>/<number>` 形式。`--scope=user` の既定 Project v2

設定ファイルが存在しない場合は無視され、フラグ / 環境変数 / 自動推定にフォールバックする。TOML が壊れている / キーが不正値のときは分かりやすいエラーで終了する。

## Projects v2 セットアップ

`org` / `user` scope を使う場合、Projects v2 のフィールド定義が必要。手順は [projects-v2-setup.md](./projects-v2-setup.md)。

`repo` scope のみ使う場合、追加セットアップは不要(GitHub Issues / Milestones がそのまま使われる)。

## 動作確認

```bash
gh tasks --version
gh tasks add 'first task' --scope=repo --repo=<owner>/<name>
```

## 関連

- [scope-detection.md](./scope-detection.md)
- [troubleshooting.md](./troubleshooting.md)
