# ユーザーマニュアル(日本語、mirror)

`gh-tasks` のユーザーマニュアル。本ディレクトリは **mirror**、SSOT は `docs/manual/en/`(repo-internal [ADR-0005](../../adr/0005-i18n-reader-based-ssot.md))。新規コンテンツは英語版に先行追加し、本ディレクトリで日本語訳を追従させる。

## 構成(Diátaxis 簡易版)

- [concepts.md](./concepts.md) — Explanation: 用語(scope / item / iteration / personal vs team)
- [guides/](./guides/) — How-to ガイド
  - [installation.md](./guides/installation.md) — `gh extension install ozzy-labs/gh-tasks` + 初期セットアップ
  - [projects-v2-setup.md](./guides/projects-v2-setup.md) — 個人 / チーム両用途のフィールド定義
  - [troubleshooting.md](./guides/troubleshooting.md) — 認証エラー、`--repo` 解決失敗、`gh agent-task` collision、API rate limit
- [reference/](./reference/) — Reference
  - [cli.md](./reference/cli.md) — 全コマンド / フラグ
  - [scope-detection.md](./reference/scope-detection.md) — `--scope` 自動判定アルゴリズムと優先順
  - [locale-detection.md](./reference/locale-detection.md) — `--lang` / `LC_ALL` / `LANG` 出力言語の優先順
- [recipes/](./recipes/) — エージェント別 Tutorial
  - [claude-code.md](./recipes/claude-code.md) — Claude Code での skill 取り込みと利用シーン
  - [codex-cli.md](./recipes/codex-cli.md) — Codex CLI での skill 取り込みと利用シーン
  - [gemini-cli.md](./recipes/gemini-cli.md) — Gemini CLI での skill 取り込みと利用シーン
  - [copilot.md](./recipes/copilot.md) — GitHub Copilot での skill 取り込みと利用シーン

## 設計の根拠

- [docs/adr/](../../adr/) — repo-internal ADR(Go 移行 / i18n / GraphQL は go-gh + genqlient 経由 / skill frontmatter)
- [docs/design/](../../design/) — repo-internal な living 設計ドキュメント
