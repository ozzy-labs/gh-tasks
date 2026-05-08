# CLAUDE.md

共通方針は @AGENTS.md を参照。以下は Claude Code 固有の設定。

## 基本ルール

- ユーザーへの確認には `AskUserQuestion` を使用する（テキスト出力で選択肢を列挙しない）

## エージェント委譲

- 本リポは外部委譲を許容（機密ファイルなし、`.no-external-delegation` 不在）
- 公開 docs / Web リサーチ / 巨大ファイル要約は `gemini-delegate` を第一選択
- 詳細な判断基準は `~/.claude/CLAUDE.md` の「エージェント委譲」セクションを参照

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

リポ固有の skill(本リポの `skills/` SSOT。CLI は `gh tasks` で実装済、配信は 2 経路: ワンショット `gh tasks install-skills`(`internal/install/`、推奨)と Renovate sync 用 `gh tasks build-skills` → `dist/{adapter}/`(後者は `cmd/build_skills.go`、Hidden)。詳細は AGENTS.md "Adapter Files"):

- `/task-add` — 会話文脈からタスクを追加する。scope に応じて GitHub Issue (repo) または Project draft item (org/user) を作成し、`gh tasks add` を呼び出す。
- `/task-plan` — 日次 / 週次 / イテレーション計画を実行する。`gh tasks plan` を呼び出して該当 scope の Milestone (repo) または Iteration (org/user) で計画項目を整理する。
- `/task-triage` — 未トリアージの Issue / Project draft item を整理する。`gh tasks triage` を呼び出してラベル付け、scope 振り分け、close 判断を補助する。
- `/task-review` — 振り返りサマリを生成する。`gh tasks review --period daily|weekly|sprint` を呼び出して期間内の Issue close / PR merge / Project アイテムの完了を要約する。
- `/task-standup` — 直近活動のスタンドアップ用サマリを生成する。`gh tasks standup [--mine]` を呼び出してチーム / 個人の動きを共有可能な形に整形する。
- `/task-link-pr` — PR を Issue / Project 項目と紐付ける。`gh tasks link <pr> <task>` を呼び出し、repo scope は PR body に `Closes #N` を追記、org/user scope は PR と Issue を同じ Project v2 に bind する。

リポ専用 skill (本リポでのみ動く、配信対象外):

- `/e2e` — gh-tasks の E2E テストを実 GitHub API に対して実行する。`mise run e2e` 系を呼び出す orchestrator。リリース前 / 大改変後の網羅検証用。詳細は `docs/design/e2e-test-plan.md`。

## Skills の共通ルール

- スキル完了時のネクストアクション提案には `AskUserQuestion` を使用する（テキスト出力で選択肢を列挙しない）
- ネクストアクションはユーザーの確認なく実行しない
