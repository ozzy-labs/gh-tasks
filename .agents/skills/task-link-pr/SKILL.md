---
name: task-link-pr
description: PR を Issue / Project 項目と紐付ける。`gh tasks link <pr> <task>` を呼び出して GitHub の relation を作成する。
description_en: Link a PR to its tracking Issue / Project item. Invokes `gh tasks link <pr> <task>` to establish the GitHub relation.
allowed-tools: Bash(gh:*)
locale: ja
---

# task-link-pr - PR と項目の紐付け

PR と該当 Issue / Project draft item を紐付ける。

## 入力

- **pr** (必須): PR 番号 / URL
- **task** (必須): Issue 番号 / draft item ID

## 手順

1. `gh tasks link <pr> <task>` を実行する(repo scope は `Closes #N` を PR body に追記、org/user scope は Project v2 の relation field を更新)
2. 戻り値で紐付け先 URL を確認する
3. 紐付け完了をユーザーに報告する

## 失敗時のフォールバック

- PR / task が異なる scope に属す: scope 跨ぎを許可するか確認
- 既に紐付け済: 既存リンクを表示して終了
