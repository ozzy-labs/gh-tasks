# 設計ドキュメント(日本語、SSOT)

`gh-tasks` の設計ドキュメント。本ディレクトリが SSOT、`docs/en/` は mirror(repo-internal ADR-0002)。

## 構成

| ファイル | 用途 |
| --- | --- |
| [concepts.md](./concepts.md) | 用語(scope / item / iteration / personal vs team) |
| [installation.md](./installation.md) | `gh extension install ozzy-labs/gh-tasks` + 初期セットアップ |
| [scope-detection.md](./scope-detection.md) | `--scope` 自動判定アルゴリズムと優先順 |
| [locale-detection.md](./locale-detection.md) | `--lang` / `LC_ALL` / `LANG` 出力言語の優先順 |
| [projects-v2-setup.md](./projects-v2-setup.md) | 個人 / チーム両用途のフィールド定義 |
| [cli-reference.md](./cli-reference.md) | 全コマンド / フラグ |
| [troubleshooting.md](./troubleshooting.md) | 認証エラー、`--repo` 解決失敗、`gh agent-task` collision、API rate limit |
| [recipes/claude-code.md](./recipes/claude-code.md) | Claude Code での skill 取り込みと利用シーン |
| [recipes/codex-cli.md](./recipes/codex-cli.md) | Codex CLI での skill 取り込みと利用シーン |
| [recipes/gemini-cli.md](./recipes/gemini-cli.md) | Gemini CLI での skill 取り込みと利用シーン |
| [recipes/copilot.md](./recipes/copilot.md) | GitHub Copilot での skill 取り込みと利用シーン |

## 設計の根拠

- [docs/adr/](../adr/): repo-internal ADR (Bun --compile / i18n SSOT / Octokit / skill frontmatter)
