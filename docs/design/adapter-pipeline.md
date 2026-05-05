# Adapter Pipeline

`skills/{name}/SKILL.md`(SSOT)から 4 エージェント向けの skill 配布物(`dist/{adapter-id}/`)を生成し、ローカル staged コピー(`.claude/skills/`、`.agents/skills/`)を再生成する `gh tasks build-skills`(`cmd/build_skills.go`)の処理連鎖を記述する。

## 目的

- skill SSOT は **1 ファイル**(`skills/{name}/SKILL.md`、ja)に集約
- Claude Code / Codex CLI / Gemini CLI / GitHub Copilot の **4 エージェント**は読み込み形式が異なるため、SSOT を adapter で各形式に変換
- consumer リポへの配信は **Renovate preset + commons の `sync-skills.sh`** が担当(本リポは生成のみ)

## 全体フロー

```text
skills/{name}/
  ├─ SKILL.md       (ja SSOT、frontmatter + body)
  └─ SKILL.en.md    (en mirror、現状 build には未消費)
        │
        ▼
gh tasks build-skills (cmd/build_skills.go: runBuildSkills)
  ├─ skills.Load(src, ...)         ← skills/ 直下のディレクトリ列挙 + SKILL.md parse + frontmatter 検証(ADR-0004)
  ├─ adapters.All()                ← 4 adapter (ClaudeCode / CodexCLI / GeminiCLI / Copilot) を取得
  ├─ adapter.Generate(loaded)      ← 各 adapter で OutputFile を生成 → dist/{id}/ に書き出し
  └─ defaultLocalStages + copyDir  ← dist/ 内の skill body を .claude/.agents/ にコピー
        │
        ▼
dist/
  ├─ claude-code/     ← .claude/skills/{name}/SKILL.md
  ├─ codex-cli/       ← .agents/skills/{name}/SKILL.md + AGENTS.md.snippet
  ├─ gemini-cli/      ← .gemini/settings.json + AGENTS.md.snippet
  └─ copilot/         ← .github/copilot-instructions.md.snippet
        │
        ▼
.claude/skills/{name}/  ← Claude Code dogfood(本リポを開いた Claude Code が参照)
.agents/skills/{name}/  ← Codex CLI dogfood
        │
        ▼ (consumer 側)
configs/skills-sync/{adapter}.json (Renovate preset) で gh_tasks_commit を bump
        │
        ▼
sync-skills.sh(commons、MARKER_TAG=@ozzylabs/gh-tasks 上書き)
        │
        ▼
consumer リポの .claude/skills/、AGENTS.md(marker block)、
.github/copilot-instructions.md(marker block)等
```

## SKILL.md スキーマ(ADR-0004)

各 SKILL.md は `--- ... ---` の frontmatter + Markdown body 構造:

```markdown
---
name: task-add
description: 会話文脈からタスクを追加する。GitHub Issue / Project draft item / repo Milestone を自動判定し、`gh tasks add` を呼び出す。
description_en: Capture a task from conversation context. Auto-detects whether the target is a GitHub Issue, Project draft item, or repo Milestone, and dispatches via `gh tasks add`.
allowed-tools: Bash(gh:*)
locale: ja
---

# task-add - Capture a task from conversation
...(本文)...
```

必須フィールド(`internal/skills/skills.go` の frontmatter 検証):

| Field | 役割 |
| --- | --- |
| `name` | skill 識別子。`skills/{name}/` ディレクトリ名と一致必須 |
| `description` | ja の 1 行説明(`AGENTS.md` snippet で使用) |
| `description_en` | en の 1 行説明(将来の en locale adapter 用に保持) |
| `allowed-tools` | Claude Code 等が認識するツール許可宣言(例: `Bash(gh:*)`) |
| `locale` | SSOT locale。本リポでは `ja` 固定(検証エラーで reject) |

`name` と `locale` は `skills.Load` 内で厳格に検証され、不一致は build 失敗。

## adapter 契約(`internal/adapters/adapters.go`)

各 adapter は **純粋関数**(handbook ADR-0018):

```go
type Adapter interface {
    ID() string                                       // dist/{id}/ サブディレクトリ名
    Generate(s []skills.Skill) []skills.OutputFile    // 副作用なし、決定論的
}
```

- ファイルシステム書き込みは **orchestrator のみ**(`cmd/build_skills.go` の `runBuildSkills`)
- adapter は入力 `[]skills.Skill` から `[]skills.OutputFile`(`{ RelativePath, Content }`)を返すだけ
- 同じ入力で常に同じ出力(順序・内容含めて)を返す必要がある
- skill 名のソートは Unicode-aware collator(`golang.org/x/text/collate`)を使い、TS 版の `String.prototype.localeCompare` と互換

## 4 adapter の実装

`internal/adapters/adapters.go` に 4 adapter とも実装される(TS 版で `scripts/adapters/*.mjs` に分離していたものを Go では 1 ファイル集約)。

### claude-code (`ClaudeCode` 型)

- 出力: `.claude/skills/{name}/SKILL.md`(`skill.Raw` をそのまま — frontmatter 含む全文)
- Claude Code は `.claude/skills/{name}/SKILL.md` を skill 定義として直接ロード
- frontmatter の `name` / `description` / `allowed-tools` で auto-trigger 判定

### codex-cli (`CodexCLI` 型)

- 出力 1: `.agents/skills/{name}/SKILL.md`(`skill.Raw` をそのまま)
- 出力 2: `AGENTS.md.snippet`(`renderAgentsMdSnippet(skills, "ja")` で生成)
- Codex CLI は `AGENTS.md` 起点で skill 名を参照、本体は `.agents/skills/{name}/SKILL.md` をロード

### gemini-cli (`GeminiCLI` 型)

- 出力 1: `.gemini/settings.json`(`{ "context": { "fileName": ["AGENTS.md"] } }`)
- 出力 2: `AGENTS.md.snippet`(codex-cli と同じ snippet 共有)
- Gemini CLI は `SKILL.md` 自動ロード機構を持たないため、AGENTS.md の skill 一覧を参照する形

### copilot (`Copilot` 型)

- 出力: `.github/copilot-instructions.md.snippet`(skill 名 + ja description のみのリスト)
- GitHub Copilot は `SKILL.md` 本体を読まないので、skill 機構は名前 + 説明文に縮約

## snippet marker block(`internal/adapters/adapters.go`)

`AGENTS.md.snippet` / `copilot-instructions.md.snippet` は **marker block 形式**:

```markdown
<!-- begin: @ozzylabs/gh-tasks -->

## gh-tasks Skills

- `task-add` — 会話文脈からタスクを追加する...
- `task-plan` — 日次 / 週次 / イテレーション計画を実行する...
...

<!-- end: @ozzylabs/gh-tasks -->
```

設計判断:

- marker tag は **`@ozzylabs/gh-tasks` 固定**(`snippetTag` 定数)、上流の `@ozzylabs/skills` の marker と独立
- consumer の `AGENTS.md` には両方の marker block が共存できる
- 上下に空行 1 つを挟む(`wrapSnippet` 関数)のは Prettier の Markdown フォーマッタ idempotency 対策
- consumer 側の sync スクリプトは marker 内のみ書き換え、外側の手書き内容は不変

## ローカル staged コピー(`defaultLocalStages` + `copyDir`)

本リポ自身を Claude Code / Codex CLI で開いたときに `/task-*` skill を即利用できるようにするため、`dist/` 配下の skill body を repo ルートの staged ディレクトリにコピーする:

| stage 先 | 内容 |
| --- | --- |
| `.claude/skills/{name}/` | `dist/claude-code/.claude/skills/{name}/` をコピー |
| `.agents/skills/{name}/` | `dist/codex-cli/.agents/skills/{name}/` をコピー |

スコープは `task-*` の本リポ skill のみ(commons の commit / lint / pr 等は `sync-commons` で別経路、衝突しない)。

## CI ガード(`--check-diff`)

`gh tasks build-skills --check-diff` は `dist/` を書き換えず、SSOT から再生成した内容と既存の `dist/{adapter}/` を比較する。差分があれば stderr に `path: <reason>` + 1 行 diff を出力し非ゼロ終了。CI で実行することで「SSOT を変えたのに `gh tasks build-skills` を走らせ忘れて `dist/` が古い」状態を検知する。

## consumer 側配信(`configs/skills-sync/`)

build 成果物を consumer リポに配信するためのフロー:

1. **Renovate preset**: `configs/skills-sync/{adapter}.json` を consumer の `renovate.json` で extend

   ```jsonc
   {
     "extends": [
       "github>ozzy-labs/gh-tasks//configs/skills-sync/claude-code",
       "github>ozzy-labs/gh-tasks//configs/skills-sync/codex-cli"
     ]
   }
   ```

2. **`.commons/sync.yaml`**: consumer リポに `gh_tasks_commit: <40-char-sha>` を持つ。Renovate がこの SHA を bump する PR を発行
3. **`sync-skills.sh`**: commons リポの汎用 sync スクリプトを `MARKER_TAG=@ozzylabs/gh-tasks` で上書きして実行、`dist/` 内容を marker 単位で展開

これにより、`@ozzylabs/skills`(汎用 commit / lint / pr 等)と `@ozzylabs/gh-tasks`(task-* 群)が同じ consumer リポに並走する。

## 検証

build 通過後の確認ポイント:

- `dist/{adapter-id}/` 4 つすべてが存在
- 各 SKILL.md の frontmatter が ADR-0004 を満たす(必須 5 フィールド、`name` 一致、`locale: ja`)
- staged copy が 6 skill 全て(`task-add` / `task-link-pr` / `task-plan` / `task-review` / `task-standup` / `task-triage`)に展開される
- `gh tasks build-skills` 出力末尾に skill 一覧が表示される
- CI で `gh tasks build-skills --check-diff` が成功する

## 関連 ADR

- [ADR-0004](../adr/0004-skill-frontmatter-schema.md): SKILL.md frontmatter 最小スキーマ
- [ADR-0005](../adr/0005-i18n-reader-based-ssot.md): i18n SSOT(SKILL.md は ja 単一を維持)
- [ADR-0006](../adr/0006-go-and-cobra-migration.md): Go + cobra 移行(本 pipeline の Go 化はこの一環)

## 関連ファイル

- orchestrator: `cmd/build_skills.go`(`runBuildSkills` / `runCheckDiff` / `defaultLocalStages` / `copyDir`)
- adapter 契約 + 4 adapter 実装 + snippet ヘルパー: `internal/adapters/adapters.go`
- skill loader + frontmatter parser: `internal/skills/skills.go`
- consumer 配信: `configs/skills-sync/README.md`、`configs/skills-sync/{adapter}.json`
