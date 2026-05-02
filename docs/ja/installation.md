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

## 設定ファイル(v0.2.0 予定)

`~/.config/ozzylabs/gh-tasks.toml`:

```toml
default_scope = "repo"
```

v0.1.0 では設定ファイル読み込みは未実装。`--scope` フラグで明示するか、git remote から自動推定される(詳細は [scope-detection.md](./scope-detection.md))。

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
