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

- Language / runtime: **Go 1.25** + toolchain go1.26.0(repo-internal ADR-0006)
- CLI framework: `spf13/cobra`
- GitHub API: `cli/go-gh/v2` (auth + GraphQL + REST、repo-internal ADR-0007)
- GraphQL types: `internal/github/queries/operations.graphql` を SSOT として `Khan/genqlient` で型自動生成(`go.mod` の `tool` ディレクティブで管理、`internal/github/queries/genqlient.go` が生成物、repo-internal ADR-0007)。REST 用ハンドコード型は `rest_types.go` のみ
- Binary build / release: `cli/gh-extension-precompile@v2`(SLSA attestations、manifest.yml 自動生成)
- Linting / formatting: `golangci-lint` v2(`gofumpt` + `goimports` + `gci`)、yamllint + yamlfmt、markdownlint-cli2、shellcheck + shfmt
- Vulnerability scan: `govulncheck`(text + SARIF を Code Scanning に出力)
- Git hooks: lefthook(commit-msg: commitlint、pre-commit: linters + `gh tasks check-i18n` (本体 + `--refs`)、pre-push: `go test` + branch 名チェック)
- Testing: 標準 `testing` + `google/go-cmp`、`go test -race -shuffle=on` を CI 必須(repo-internal ADR-0008)

> 移行履歴: 2026-04 までは Bun --compile + TypeScript で実装(ADR-0001 / 0003)。2026-05 から Go へ完全移行(計画書: `docs/design/go-migration-plan.md`、後継 ADR-0006 / 0007 / 0008)。

## ディレクトリ構成

```text
main.go                 → cobra root エントリ(リポルート、gh extension 慣習)
cmd/                    → cobra コマンド定義(add/list/today/done/standup/
                         review/plan/triage/link/projects、build-skills /
                         check-i18n は Hidden)
internal/               → ドメインロジック
  ├── github/           → cli/go-gh ラッパ(GraphQL + REST)
  │   └── queries/      → `operations.graphql`(SSOT)+ genqlient 自動生成型 + REST 用ハンドコード型(`rest_types.go`)
  ├── i18n/             → embed 済 en/ja JSON catalog + ResolveLocale + T
  ├── i18ncheck/        → ハードコード非 ASCII 検知 (gh tasks check-i18n)
  ├── scope/ repo/      → 3-scope (repo/org/user) 抽象 + リポ解決
  ├── project/ projectitem/ period/ config/ → ドメイン helpers
  ├── skills/           → SKILL.md frontmatter parse + Load
  ├── adapters/         → 4 エージェント向け OutputFile 生成
  └── testfake/         → GraphQL/REST テスト用 fake (cmd/internal 共通)
skills/             → skill SSOT(SKILL.md = ja、SKILL.en.md = en)
dist/{adapter}/         → adapter 出力(`gh tasks build-skills` で生成)
docs/manual/{en,ja}/    → ユーザーマニュアル(en SSOT、ja mirror)
docs/adr/               → repo-internal ADR(ja 単一)
docs/design/            → repo-internal な living 設計ドキュメント(ja 単一)
.agents/ ・ .claude/    → commons + skills sync 配置先
```

## 主要コマンド

```bash
mise install                          # toolchain (go / golangci-lint / govulncheck 等)
go build ./...                        # CLI バイナリ
go run . build-skills                 # adapter pipeline 実行(dist/{adapter}/ 生成)
go run . check-i18n                   # ハードコード非 ASCII 検知
go test -race -shuffle=on ./...       # テスト
golangci-lint run --timeout 5m ./...  # lint v2
govulncheck ./...                     # 脆弱性スキャン
yamllint . && yamlfmt . && markdownlint-cli2 '**/*.md'   # 共有 lint
```

## i18n SSOT

読み手ベース SSOT(repo-internal ADR-0005、ADR-0002 を Superseded):

- README: `README.md`(en SSOT)+ `README.ja.md`(ja mirror)
- ユーザーマニュアル: `docs/manual/en/`(**en SSOT**)+ `docs/manual/ja/`(ja mirror、新規コンテンツは en 先行で ja 追従)
- SKILL.md: `SKILL.md`(ja SSOT)+ `SKILL.en.md`(en mirror)
- ADR(`docs/adr/`): ja のみ(repo-internal な意思決定記録、翻訳しない)
- 設計ドキュメント(`docs/design/`): ja のみ(living な設計メモ、翻訳しない)
- CLI 出力 / エラー(`internal/i18n/`): **en SSOT** + ja translation(新規キーは en に書き、ja を追従させる)
- AGENTS.md / CLAUDE.md: ja のみ

**ハードコード文字列禁止**: 非 ASCII を含むリテラルは `i18n.T(locale, "key", args...)` 経由必須(`internal/i18n/{en,ja}.json` に定義)。`gh tasks check-i18n`(`internal/i18ncheck`、`go/parser` ベース)が pre-commit / CI で強制。エラー型は `i18n.Payload` を埋め込んで `i18n.Localized` インタフェースを満たし、上位 (`cmd/list.go` 等の `localizedError`) で resolve した locale を使い localize する(例: `scope.ScopeError`、`repo.RepoError`、`project.ProjectError`、`period.PeriodError`、`config.ConfigError`、`github.AuthError`)。

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

- `task-add` — 会話文脈からタスクを追加する。scope に応じて GitHub Issue (repo) または Project draft item (org/user) を作成し、`gh tasks add` を呼び出す。
- `task-link-pr` — PR を Issue / Project 項目と紐付ける。`gh tasks link <pr> <task>` を呼び出し、repo scope は PR body に `Closes #N` を追記、org/user scope は PR と Issue を同じ Project v2 に bind する。
- `task-plan` — 日次 / 週次 / イテレーション計画を実行する。`gh tasks plan` を呼び出して該当 scope の Milestone (repo) または Iteration (org/user) で計画項目を整理する。
- `task-review` — 振り返りサマリを生成する。`gh tasks review --period daily|weekly|sprint` を呼び出して期間内の Issue close / PR merge / Project アイテムの完了を要約する。
- `task-standup` — 直近活動のスタンドアップ用サマリを生成する。`gh tasks standup [--mine]` を呼び出してチーム / 個人の動きを共有可能な形に整形する。
- `task-triage` — 未トリアージの Issue / Project draft item を整理する。`gh tasks triage` を呼び出してラベル付け、scope 振り分け、close 判断を補助する。

<!-- end: @ozzylabs/gh-tasks -->

## Adapter Files

`gh tasks build-skills` が `dist/{adapter}/` 配下に出力するファイル(consumer リポへ sync される):

| Agent | Adapter Output |
| ----- | -------------- |
| Claude Code | `.claude/skills/<name>/SKILL.md` |
| Codex CLI | `.agents/skills/<name>/SKILL.md` + `AGENTS.md.snippet` |
| Gemini CLI | `.gemini/settings.json` + `AGENTS.md.snippet` |
| GitHub Copilot | `.github/copilot-instructions.md.snippet` |

`AGENTS.md.snippet` / `copilot-instructions.md.snippet` は consumer 側の `AGENTS.md` / `.github/copilot-instructions.md` の marker block (`<!-- begin: @ozzylabs/gh-tasks -->` ～ `<!-- end -->`) に挿入される(idempotent)。
