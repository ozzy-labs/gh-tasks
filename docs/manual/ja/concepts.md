# 用語と概念

`gh-tasks` が扱う対象を理解するための用語集。

## scope

`gh-tasks` のすべてのコマンドは 3 つの **scope** のいずれかで動作する:

| Scope | 用途 | データソース |
| --- | --- | --- |
| `repo` | 単一リポジトリの実装作業 | GitHub Issues + Milestones |
| `org` | プロジェクト横断の調整 | Organization Project v2 |
| `user` | 個人 todo / 日次計画 | 個人 Project v2 |

scope は `--scope` フラグで明示するか、自動判定される。詳細は [scope-detection.md](./reference/scope-detection.md)。

## item

`gh-tasks` が扱うタスクの単位。scope ごとに backing storage が異なる:

- `repo` scope → GitHub Issue
- `org` / `user` scope → Projects v2 の draft item、または Issue にリンクされた Project item

CLI / skill では item を抽象化し、scope に応じた具体クエリへ変換する。

## iteration

scope 別の "計画期間" 単位:

- `repo` scope → GitHub Milestone
- `org` / `user` scope → Project v2 の Iteration field

`gh tasks plan` / `gh tasks review` の `--period daily|weekly|sprint` はこの iteration を抽象化したもの。

## personal vs team

`user` scope は個人プロジェクト(自分しか見ない todo)、`org` scope は組織全体の調整に使う Project v2。CLI / skill 両方で同じコマンド (`gh tasks add` 等) が両用途に使える点がこの CLI の中核設計。

## 関連 ADR

- [docs/adr/0001](../../adr/0001-use-bun-compile-for-binary.md): Bun --compile 採用
- [docs/adr/0003](../../adr/0003-graphql-via-octokit.md): GraphQL は Octokit 経由
