# CLAUDE.md

共通方針は @AGENTS.md を参照。以下は Claude Code 固有の設定。

## 基本ルール

- ユーザーへの確認には `AskUserQuestion` を使用する（テキスト出力で選択肢を列挙しない）

## Available Skills

`@ozzylabs/skills` から sync される共通 skill:

- `/implement` — Issue または指示をもとに、ブランチ作成・実装
- `/lint` — 全リンターを自動修正付きで実行
- `/test` — ビルド・テスト・型チェックを実行
- `/commit` — 変更をステージし、Conventional Commits でコミット
- `/pr` — 変更を push し、PR を作成・更新
- `/review` — コード変更や PR をレビュー
- `/ship` — lint・コミット・PR 作成を一括実行
- `/drive` — implement + ship + review loop（Issue から merge-ready な PR まで自律駆動）

リポ固有の skill(本リポの `src/skills/` SSOT、未実装):

- `/task-add` — 会話文脈からタスク化
- `/task-plan` — 日次 / 週次 / スプリント計画
- `/task-triage` — inbox triage
- `/task-review` — daily / weekly retrospective
- `/task-standup` — 活動サマリ
- `/task-link-pr` — PR と項目の紐付け

## Skills の共通ルール

- スキル完了時のネクストアクション提案には `AskUserQuestion` を使用する（テキスト出力で選択肢を列挙しない）
- ネクストアクションはユーザーの確認なく実行しない
