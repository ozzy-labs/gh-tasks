# 設計ドキュメント(日本語、SSOT)

`gh-tasks` の設計ドキュメント。本ディレクトリが SSOT、`docs/en/` は mirror(repo-internal ADR-0002)。

## 構成

| ファイル | 用途 |
| --- | --- |
| [concepts.md](./concepts.md) | 用語(scope / item / iteration / personal vs team) |
| [installation.md](./installation.md) | `gh extension install ozzy-labs/gh-tasks` + 初期セットアップ |
| [scope-detection.md](./scope-detection.md) | `--scope` 自動判定アルゴリズムと優先順 |
| [projects-v2-setup.md](./projects-v2-setup.md) | 個人 / チーム両用途のフィールド定義 |
| [cli-reference.md](./cli-reference.md) | 全コマンド / フラグ |
| [troubleshooting.md](./troubleshooting.md) | 認証エラー、`--repo` 解決失敗、`gh agent-task` collision、API rate limit |
| recipes/{agent}.md | 各エージェントでの使用例(v0.1.0 後続) |

> v0.1.0 初版。実装と並走して内容を充実化していく。recipes は v0.1.0 後続で追加。

## 設計の根拠

- [handbook ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md): リポ新設の意思決定
- [handbook ADR-0018](https://github.com/ozzy-labs/handbook/blob/main/adr/0018-agent-adapter-architecture.md): 4 エージェント adapter 機構
- [handbook ADR-0016](https://github.com/ozzy-labs/handbook/blob/main/adr/0016-create-skills-repo.md): skill SSOT 独立リポ化
- [handbook reviews/2026-04-30-gh-tasks-design.md](https://github.com/ozzy-labs/handbook/blob/main/reviews/2026-04-30-gh-tasks-design.md): v0.1.0 実装仕様レビュー
- [docs/adr/](../adr/): repo-internal ADR (Bun --compile / i18n SSOT / Octokit / skill frontmatter)
