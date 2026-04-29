# 設計ドキュメント(日本語、SSOT)

`gh-tasks` の設計ドキュメント。本ディレクトリが SSOT、`docs/en/` は mirror(repo-internal ADR-0002)。

## 構成

| ファイル | 用途 |
| --- | --- |
| concepts.md | 用語(scope / item / iteration / personal vs team) |
| installation.md | `gh extension install ozzy-labs/gh-tasks` + 初期セットアップ |
| scope-detection.md | `--scope` 自動判定アルゴリズムと設定優先順 |
| projects-v2-setup.md | 個人 / チーム両用途のフィールド定義 |
| cli-reference.md | 全コマンド / フラグ |
| recipes/{agent}.md | 各エージェントでの使用例 |
| troubleshooting.md | `gh agent-task` collision 検知時の手順、認証エラー、PATH 問題 |

> 本ディレクトリは v0.1.0 スコープのスタブ。実装と並行して埋めていく。
