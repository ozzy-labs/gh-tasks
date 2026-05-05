---
name: task-add
description: 会話文脈からタスクを追加する。GitHub Issue / Project draft item / repo Milestone を自動判定し、`gh tasks add` を呼び出す。
description_en: Capture a task from conversation context. Auto-detects whether the target is a GitHub Issue, Project draft item, or repo Milestone, and dispatches via `gh tasks add`.
allowed-tools: Bash(gh:*)
locale: ja
---

# task-add - 会話文脈からタスクを追加

会話の中で「これはタスクとして残しておこう」と判断した内容を、適切なスコープに反映する。

## 入力

- **title** (必須): タスクの 1 行サマリ
- **--scope** (任意): `repo` / `org` / `user` のいずれか。省略時は git remote から推定
- **--repo** (任意): repo scope 時のターゲットリポジトリ (`<owner>/<name>`)

## 手順

1. 会話文脈から本文 / 受け入れ条件 / 関連 PR / リンクを抽出して整形する
2. `gh tasks add <title> [--scope ...] [--repo ...]` を実行する
3. 戻り値の Issue / draft item URL をユーザーに提示する

## 失敗時のフォールバック

- `gh auth login` 未実行: 認証手順を提示して中断
- scope 自動判定が `repo` だが git remote 不在: `--scope user` でフォールバック提案
