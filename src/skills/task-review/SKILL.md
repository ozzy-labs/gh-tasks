---
name: task-review
description: 振り返りサマリを生成する。`gh tasks review --daily|--weekly|--sprint` を呼び出して期間内の Issue close / PR merge / 差分を要約する。
description_en: Generate a retrospective summary. Invokes `gh tasks review --daily|--weekly|--sprint` to summarize Issues closed / PRs merged / diffs within the period.
allowed-tools: Bash(gh:*)
locale: ja
---

# task-review - 振り返り

daily / weekly / sprint 単位の振り返りサマリを生成する。

## 入力

- **--period** (任意): `daily` / `weekly` / `sprint`。省略時は `weekly`
- **--scope** (任意): `repo` / `org` / `user`

## 手順

1. `gh tasks review --period <period> --scope <scope>` を実行する
2. 戻り値(完了 / 進行中 / ブロッカーの 3 セクション)を Markdown で整形する
3. 期間内に学んだこと / 次期に持ち越す課題をユーザーに確認する

## 失敗時のフォールバック

- 戻り値が空: 期間 / scope の指定をユーザーに再確認
- API rate limit: 期間を絞って再試行

> v0.1.0 スタブ。`gh tasks review` の CLI 実装が完成するまで実用不可。
