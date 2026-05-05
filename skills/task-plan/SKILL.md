---
name: task-plan
description: 日次 / 週次 / イテレーション計画を実行する。`gh tasks plan` を呼び出して該当 scope の Milestone (repo) または Iteration (org/user) で計画項目を整理する。
description_en: Run a daily / weekly / iteration plan. Invokes `gh tasks plan` to organize plan items in the relevant scope's Milestone (repo) or Iteration (org/user).
allowed-tools: Bash(gh:*)
locale: ja
---

# task-plan - 計画立て

日次 / 週次 / イテレーション単位の計画を立て、該当する scope のバックログから今期の項目を確定する。

## 入力

- **--period** (任意): `daily` / `weekly` / `sprint` のいずれか。省略時は `weekly`
- **--scope** (任意): `repo` / `org` / `user`。省略時は git remote から推定
- **--dry-run** (任意): Milestone 作成・bind を行わず候補一覧のみ表示

## 手順

1. 直近の未完了 / open Issue / draft を取得して要約する
2. ユーザーと優先順位を確認する(必要なら `--dry-run` で候補を確認)
3. `gh tasks plan [--period ...] [--scope ...]` で確定する(repo scope は Milestone を作成 / 再利用して期間内の Issue を bind、org/user scope は Project v2 Iteration を更新)
4. 確定した計画一覧をユーザーに提示する

## 失敗時のフォールバック

- 計画候補が多すぎる: 対象を絞る条件(label / assignee 等)をユーザーに確認
- Iteration field が未定義: `docs/manual/ja/guides/projects-v2-setup.md` の手順を提示
- 既に別 Milestone に紐付いた Issue: CLI 側で skip され通知されるので、必要に応じて `--dry-run` で事前確認
