---
name: task-triage
description: 未トリアージの Issue / Project draft item を整理する。`gh tasks triage` を呼び出してラベル付け、scope 振り分け、close 判断を補助する。
description_en: Triage untriaged Issues / Project draft items. Invokes `gh tasks triage` to assist with labeling, scope routing, and close decisions.
allowed-tools: Bash(gh:*)
locale: ja
---

# task-triage - インボックス整理

未トリアージの Issue / draft item を一括で整理する。

## 入力

- **--scope** (任意): `repo` / `org` / `user`。省略時は git remote から推定
- **--limit** (任意): 一度に扱う件数(デフォルト 20)

## 手順

1. `gh tasks triage --scope <scope> --limit <N>` で未トリアージ一覧を取得する
2. 各項目について以下を判定し、ユーザーに提案する:
   - **label**: Conventional 系(`feat` / `fix` / `docs` / etc.)
   - **scope** 振り分け: 個人項目 / repo / org のどこで管理すべきか
   - **状態**: 残す / close する / 別 Issue にマージする
3. 判定結果を `gh tasks triage` の interactive モードで反映する
4. 残件数と次回 triage の推奨タイミングを報告する

## 失敗時のフォールバック

- 大量に未トリアージがある: `--limit` を絞って分割実行
- 判定が曖昧な項目: ユーザーに直接確認
