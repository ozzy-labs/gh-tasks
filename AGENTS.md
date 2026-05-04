# AGENTS.md

このファイルは AI エージェント向けの共通 instructions です。

## 基本方針

- 日本語で応答する
- 推奨案とその理由を提示する
- `.env` ファイルは読み取り・ステージングしない
- 破壊的な Git 操作を避ける

## プロジェクト概要

`gh-tasks`: GitHub Projects v2 / Issues / Milestone を横断するタスク管理 CLI(`gh tasks` extension)+ skill bundle。3 スコープ(`repo` / `org` / `user`)を統一抽象でカバーし、4 エージェント(Claude Code / Codex CLI / Gemini CLI / GitHub Copilot)向け skill を adapter 機構経由で配布する。

## Tech Stack

- Runtime: Bun(CLI)+ Node.js(scripts、tooling)
- Language: TypeScript(strict、ESM)
- Binary build: `bun build --compile`(repo-internal ADR-0001)
- Package manager: pnpm 10
- GraphQL client: Octokit(`gh api graphql` shell-out は不採用、repo-internal ADR-0003)
- Linting: Biome(TS/JS/JSON)、yamllint + yamlfmt(YAML)、markdownlint-cli2(Markdown)、shellcheck + shfmt(Shell)
- Git hooks: lefthook(commit-msg: commitlint、pre-commit: linters、pre-push: typecheck)
- Testing: Vitest

## ディレクトリ構成

```text
packages/gh-tasks/      → CLI 本体(TS、Bun --compile 対象)
packages/templates/     → Projects v2 フィールド定義 / Issue templates
src/skills/             → SSOT(SKILL.md = ja、SKILL.en.md = en)
dist/{adapter}/         → 4 エージェント向け adapter 出力
docs/{ja,en}/           → 設計ドキュメント(ja SSOT、en mirror)
docs/adr/               → repo-internal ADR
scripts/                → build / sync スクリプト
.agents/ ・ .claude/    → commons + skills sync 配置先
```

## 主要コマンド

```bash
pnpm install              # 依存関係インストール
pnpm run build            # CLI バイナリ + skills dist 生成
pnpm run lint             # Biome
pnpm run lint:all         # Biome + markdownlint + yamllint(gitleaks/trivy/shellcheck 等は lefthook hook 側)
pnpm run typecheck        # tsc --noEmit
pnpm test                 # Vitest
```

## i18n SSOT

- README: `README.md`(en SSOT)+ `README.ja.md`(ja mirror)
- 設計 docs: `docs/ja/`(SSOT)+ `docs/en/`(mirror)
- SKILL.md: `SKILL.md`(ja SSOT)+ `SKILL.en.md`(en mirror)
- ADR(`docs/adr/`): ja のみ(社内意思決定文書、翻訳しない)

repo-internal ADR-0002 で根拠記録。

## 規約

- コミット: Conventional Commits(commitlint で強制)
- ブランチ: GitHub Flow + squash merge のみ、`<type>/<short-description>`
- type: feat / fix / docs / style / refactor / perf / test / build / ci / chore / revert

<!-- begin: @ozzylabs/skills -->

## Available Skills

- `commit` — 変更をステージし、Conventional Commits でコミットする。プッシュや PR 作成は行わない。
- `commit-conventions` — Conventional Commits のメッセージ生成ルール（Type/Scope 判定表、フォーマット）。他スキルから参照される。
- `drive` — Issue から実装・PR 作成・セルフレビュー・修正を自動で回し、merge-ready な PR を出す。Issue 番号またはテキスト指示を受け取る。オプションでマージまで実行可能。
- `implement` — Issue または指示をもとに、ブランチ作成・実装計画・コード変更を行う。Issue 番号またはテキスト指示を受け取る。
- `lint` — 全リンターを自動修正付きで実行し、結果を報告する。コード品質チェック、フォーマット、型チェック、セキュリティスキャンを含む。
- `lint-rules` — 拡張子別リンター・フォーマッターのコマンド対応表と型チェックルール。他スキルから参照される。
- `pr` — コミット済みの変更をリモートにプッシュし、PR を作成・更新する。
- `review` — コード変更や PR をレビューし、問題点・改善案を報告する。PR 番号または空（ワーキングツリー）を受け取る。
- `ship` — lint・コミット・PR 作成を一括実行する。変更に対して lint → コミット → PR 作成を順に実行する統合パイプライン。
- `test` — ビルド・テスト・型チェックを実行し、結果を報告する。

<!-- end: @ozzylabs/skills -->

<!-- begin: @ozzylabs/gh-tasks -->

## gh-tasks Skills

- `task-add` — 会話文脈からタスクを追加する。GitHub Issue / Project draft item / repo Milestone を自動判定し、`gh tasks add` を呼び出す。
- `task-link-pr` — PR を Issue / Project 項目と紐付ける。`gh tasks link <pr> <task>` を呼び出して GitHub の relation を作成する。
- `task-plan` — 日次 / 週次 / イテレーション計画を実行する。`gh tasks plan` を呼び出して該当 scope の Milestone (repo) または Iteration (org/user) で計画項目を整理する。
- `task-review` — 振り返りサマリを生成する。`gh tasks review --period daily|weekly|sprint` を呼び出して期間内の Issue close / PR merge / Project アイテムの完了を要約する。
- `task-standup` — 直近活動のスタンドアップ用サマリを生成する。`gh tasks standup [--mine]` を呼び出してチーム / 個人の動きを共有可能な形に整形する。
- `task-triage` — 未トリアージの Issue / Project draft item を整理する。`gh tasks triage` を呼び出してラベル付け、scope 振り分け、close 判断を補助する。

<!-- end: @ozzylabs/gh-tasks -->

## Adapter Files

| Agent | Configuration |
| ----- | ------------- |
| Claude Code | `CLAUDE.md`, `.claude/` |
| Gemini CLI | `.gemini/settings.json` → `AGENTS.md` |
| Codex CLI | `AGENTS.md` + `.agents/skills/` |
| GitHub Copilot | `AGENTS.md` + `.agents/skills/` |
