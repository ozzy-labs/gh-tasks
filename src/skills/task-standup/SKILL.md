---
name: task-standup
description: 直近活動のスタンドアップ用サマリを生成する。`gh tasks standup [--mine]` を呼び出してチーム / 個人の動きを共有可能な形に整形する。
description_en: Generate a standup summary of recent activity. Invokes `gh tasks standup [--mine]` to format team or personal activity for sharing.
allowed-tools: Bash(gh:*)
locale: ja
---

# task-standup - 活動サマリ

直近のチーム / 個人の動きをスタンドアップ向けに整形する。

## 入力

- **--mine** (任意): 自分の活動だけに絞る
- **--since** (任意): ISO 8601 形式の開始日時(デフォルト: 直近 24h)
- **--scope** (任意): `repo` / `org` / `user`

## 手順

1. `gh tasks standup [--mine] [--since ...] [--scope ...]` を実行する
2. 戻り値を「昨日 / 今日 / ブロッカー」3 セクションに整形する
3. Slack / 議事録に貼り付け可能な Markdown で提示する

## 失敗時のフォールバック

- `--since` が古すぎて活動が多すぎる: より短い期間を提案
- `--mine` で活動ゼロ: 期間を伸ばすか scope を切り替える提案
