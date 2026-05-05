# Architecture

`gh-tasks` の全体俯瞰。モジュール境界、ディレクトリ構成、主要データフローを記述する。各意思決定の根拠は `docs/adr/` に分離。

## ハイレベル構成

`gh-tasks` は **3 つの柱** から成る:

```text
                    ┌─────────────────────────────┐
                    │   skills/{name}/        │ ← skill SSOT (ja)
                    │     SKILL.md / SKILL.en.md  │
                    └────────────┬────────────────┘
                                 │ gh tasks build-skills (per-adapter transform)
                                 ▼
                    ┌─────────────────────────────┐
                    │   dist/{adapter-id}/        │ ← 4 adapter 出力
                    │   .claude/skills/(staged)   │
                    │   .agents/skills/(staged)   │
                    └────────────┬────────────────┘
                                 │ skills-sync (Renovate preset + sync-skills.sh)
                                 ▼
                    consumer リポ(.claude/skills 等)

┌────────────────────────────────────────────────────────────┐
│  main.go (リポルート)                                        │ ← CLI 本体
│  cmd/                (cobra コマンド: add/list/today/plan/   │
│                       triage/done/review/standup/link/       │
│                       projects + Hidden: build-skills /      │
│                       check-i18n)                            │
│  internal/                                                   │
│    ├─ github/queries/(GraphQL operation + 型、ADR-0007)     │
│    ├─ i18n/          (en SSOT + ja translation、embed JSON) │
│    ├─ scope/ repo/ project/ projectitem/ period/ config/    │
│    ├─ skills/        (frontmatter parse + Load)             │
│    └─ adapters/      (4 agent OutputFile 生成)              │
└────────────────────────────────────────────────────────────┘
                                 │ cli/gh-extension-precompile@v2
                                 ▼
                    <goos>-<goarch>[.exe] + manifest.yml
                    (darwin/linux/windows × amd64/arm64、SLSA 付)
```

## ディレクトリ構成

```text
gh-tasks/
├── main.go                            # cobra root エントリ(gh extension 慣習)
├── cmd/                               # cobra コマンド定義
│   ├── root.go / deps.go              # root / 依存注入
│   ├── add.go list.go today.go ...    # ユーザー向けコマンド
│   ├── build_skills.go                # Hidden: adapter pipeline 実行
│   └── check_i18n.go                  # Hidden: ハードコード非 ASCII 検知
├── internal/                          # ドメインロジック(import 不可境界)
│   ├── github/                        # cli/go-gh ラッパ + クエリ
│   │   ├── github.go                  # GraphQLClient / RESTClient interfaces
│   │   └── queries/                   # genqlient SSOT + REST 用ハンドコード型
│   │       ├── operations.graphql     # 25 操作の SSOT(ADR-0007)
│   │       ├── genqlient.go           # genqlient 自動生成型
│   │       ├── pagination.go          # GraphQL pagination helper
│   │       └── rest_types.go          # REST 用ハンドコード型
│   ├── i18n/                          # embed JSON catalog + Resolve / T
│   ├── i18ncheck/                     # go/parser ベース i18n lint
│   ├── scope/ repo/ project/          # ID 解決群
│   ├── projectitem/ period/ config/   # ドメイン helpers
│   ├── skills/                        # SKILL.md frontmatter parse + Load
│   ├── adapters/                      # 4 agent OutputFile 生成
│   └── testfake/                      # GraphQL テスト用 fake (cmd/internal 共通、REST は cmd-only)
├── templates/                # Projects v2 フィールド定義 YAML
├── skills/{name}/                 # skill SSOT(ja: SKILL.md、en mirror)
├── docs/
│   ├── manual/{en,ja}/                # ユーザーマニュアル(en SSOT、ja mirror)
│   ├── adr/                           # 意思決定記録(ja 単一)
│   └── design/                        # 設計ドキュメント(本ディレクトリ、ja 単一)
├── dist/{adapter-id}/                 # adapter 出力(.gitignore、build-skills で再生成)
├── .claude/skills/ .agents/skills/    # ローカル staged コピー(dogfooding)
├── configs/skills-sync/                       # consumer 向け Renovate preset
├── go.mod / go.sum / .golangci.yaml / genqlient.yaml
└── .github/workflows/                 # ci.yaml(go ジョブ含)+ release.yaml(precompile-action)
```

## 主要モジュール境界

### `main.go` + `cmd/root.go`

エントリポイント。責務:

- `cmd.Root()` が cobra root を組み立て、各サブコマンドを `Add` する
- `SilenceUsage: true` / `SilenceErrors: true`(Cobra ベストプラクティス)
- 各コマンドは `Deps`(stdout / stderr / GraphQL client factory / config loader / time / env / git remote)を注入可能で、テストはフェイクを差し込んで決定論的に検証する

### `cmd/{name}.go`

各サブコマンドの実装。共通パターン:

1. `deps.Resolve()` で config + locale を読み込み
2. `scope.Detect(...)` で scope 判定
3. scope に応じて分岐(repo は GitHub Issues + Milestones、org/user は Projects v2)
4. GraphQL は `clients.GraphQL.Do(ctx, queries.<Op>, vars, &resp)` 経由
5. 結果を localized メッセージで `cmd.OutOrStdout()` に書く、`localizedError` でエラーを stderr へ写像
6. RunE は `errors.As` でドメインエラーを判別、`%w` ラップを徹底

### `internal/*`

純粋関数 + interface 中心のヘルパー群。各パッケージは「1 つの解決責務」を持つ:

| パッケージ | 責務 | 主な関数 / 型 |
| --- | --- | --- |
| `config` | TOML config 読込 + 検証 | `Load`、`ConfigError` |
| `repo` | `<owner>/<name>` 解決 | `Resolve`、`RepoError` |
| `scope` | `--scope` 自動判定 | `Detect`、`ScopeError` |
| `project` | `<owner>/<number>` 解決 | `Resolve`、`ProjectError` |
| `period` | `daily`/`weekly`/`sprint` の境界計算(IANA tz 対応) | `Of`、`PeriodError` |
| `github` | go-gh wrapper、interface ベース GraphQL/REST client、token 解決 | `NewClients`、`AuthError` |
| `projectitem` | Project v2 item 解決 + format helper | `ResolveProjectNodeID`、`FormatItem` |
| `github/queries` | GraphQL operation SSOT(`operations.graphql`)+ genqlient 自動生成型 + REST ハンドコード型 | `GetOrgProjectV2` 等(25 operations) |
| `i18n` | embed JSON catalog + locale 解決 + `T` | `ResolveLocale`、`T`、`Payload` |
| `i18ncheck` | go/parser ベースの非 ASCII 検知 | `Scan`、`HasNonASCII`、`Decorative` |
| `skills` | `skills/<name>/SKILL.md` parse + Load | `Load`、`ParseDocument` |
| `adapters` | 4 agent OutputFile 生成 | `ClaudeCode` / `CodexCLI` / `GeminiCLI` / `Copilot` |
| `testfake` | `cmd/` および `internal/` 共通の GraphQL フェイク(REST フェイクは cmd テスト内に閉じる) | `FakeGraphQL`、`RecordingGraphQL` |

### `internal/i18n`

CLI 出力 / エラーメッセージの key ベース translation。

- `en.json` が **SSOT**(repo-internal ADR-0005)、`ja.json` が translation。両方 `embed` で binary に同梱
- `i18n.T(locale, key, args...)` で参照、locale 解決は `i18n.ResolveLocale(argv, env, config)`
- フォールバック chain: 指定 locale → en → key 文字列(デバッグ用)
- locale 解決順: `--lang` フラグ → config `lang` → `LC_ALL` → `LANG` → fallback `en`(POSIX 標準で `LC_ALL` が `LANG` より優先)
- 実装方針: 外部 i18n ライブラリ(`nicksnyder/go-i18n/v2` 等)は不採用。Go 標準 `embed` + 自前 JSON unmarshal で軽量化と依存最小化を優先(go-migration-plan.md 冒頭「計画と最終実装で異なった主な箇所」参照)

## エラー設計

`internal/{config,repo,scope,project,period,github}` のエラーは **i18n.Payload 埋め込みパターン**:

```go
type ScopeError struct{ i18n.Payload }

func (e *ScopeError) Error() string { return e.Key }
```

- `Payload` は `Key string` + `Args map[string]any` を保持し、`I18nKey()` / `I18nArgs()` を実装する `i18n.Localized` インタフェースに準拠する
- `errors.As(err, &target)` でドメイン判別、上位 (`cmd/list.go` 等の `localizedError`) で `i18n.T(loc, key, flat...)` を呼んで stderr に書く
- ハードコード非 ASCII 文字列は `gh tasks check-i18n` (`internal/i18ncheck`、`go/parser`) が pre-commit / CI で検知して reject

## 配布モデル

### CLI バイナリ

- `cli/gh-extension-precompile@v2`(GitHub 公式 Action)が `go build` を全 OS/arch で実行し、`<goos>-<goarch>[.exe]` 命名規則 + `manifest.yml`(プラットフォーム解決メタデータ)+ SLSA attestations を発行する
- GitHub Releases に attach、`gh extension install ozzy-labs/gh-tasks` でユーザー側にダウンロード(`manifest.yml` を gh が読んで適切な binary を選択)
- ローカル開発: `gh extension install . --force` でカレントブランチの `go build` 出力をそのまま使う dogfooding(repo-internal ADR-0006)

### skill bundle

詳細は `docs/design/adapter-pipeline.md` を参照。要約:

1. `skills/{name}/SKILL.md`(ja SSOT)を `gh tasks build-skills`(`cmd/build_skills.go`)が読み込み
2. 4 adapter(claude-code / codex-cli / gemini-cli / copilot、`internal/adapters/`)が `dist/{adapter-id}/` に各エージェント形式で出力
3. consumer リポは `configs/skills-sync/{adapter}.json` Renovate preset を extend し、`sync-skills.sh` で `dist/` 内容を取り込む

## テスト構成

- `*_test.go` は同パッケージ + `_test` パッケージで配置(black-box テスト、`internal/scope/scope_test.go` 等)
- `go test -race -shuffle=on ./...` を CI 必須(repo-internal ADR-0008)
- `Deps` 構造体に GraphQL client factory / config loader / time / env / git remote を注入して決定論的に検証(`cmd/<cmd>_flow_test.go` で `cmd/testhelpers_test.go` の `captureGraphQL` 経由、共通フェイクは `internal/testfake/`)
- 詳細は [docs/design/test-structure.md](./test-structure.md) を参照
- diff は `google/go-cmp` を使用、`testify/require` は致命エラーの fail-fast 限定

## 関連 ADR

- [ADR-0001](../adr/0001-use-bun-compile-for-binary.md): Bun --compile 採用(Superseded by 0006)
- [ADR-0002](../adr/0002-i18n-japanese-ssot.md): i18n は Japanese SSOT(Superseded by 0005)
- [ADR-0003](../adr/0003-graphql-via-octokit.md): GraphQL は Octokit 経由(Superseded by 0007)
- [ADR-0004](../adr/0004-skill-frontmatter-schema.md): SKILL.md frontmatter スキーマ
- [ADR-0005](../adr/0005-i18n-reader-based-ssot.md): i18n SSOT を読み手ベースに再設計、docs/ 構造再編
- [ADR-0006](../adr/0006-go-and-cobra-migration.md): Go 1.25 + cobra + `cli/gh-extension-precompile@v2` への完全移行
- [ADR-0007](../adr/0007-go-gh-graphql-client.md): GitHub API は `cli/go-gh/v2` 経由
- [ADR-0008](../adr/0008-go-test-and-quality-chain.md): Go テスト・品質チェーン
