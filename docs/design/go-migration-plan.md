# Go 完全移行計画

`gh-tasks` のすべての TypeScript / Node.js 実装を Go に置き換える移行計画。CLI 本体だけでなく、build / sync スクリプトまで含む完全移行を対象とする。

- Status: Completed (Phase 7 カットオーバー完了、2026-05)
- Date: 2026-05-04(計画策定)/ 2026-05 完了
- Scope: `packages/gh-tasks/` + `scripts/` + toolchain
- Owner: ozzy

> **本ドキュメントは歴史的記録**: 移行は 2026-05 に完了済。Phase / DoD のチェックボックスは計画書時点の表記を保持し、後付けで `[x]` に更新しない方針(完了状況は `Status: Completed` で示す)。
>
> **計画と最終実装で異なった主な箇所**:
>
> - i18n は `nicksnyder/go-i18n/v2` 採用案だったが、最終実装は標準 `embed` + 自前 JSON unmarshal(`internal/i18n/i18n.go`)。go-i18n は不採用(軽量化のため)。
> - Repository 検出は `cli/go-gh/v2/pkg/repository` の `Current()` 採用案だったが、最終実装は `internal/repo` の独自 git remote 解析 + `--repo` flag(テスト容易性のため)。
>
> **更新履歴**
>
> - 2026-05-04 初版（goreleaser 採用案）
> - 2026-05-04 v2: gh extension 公式仕様レビューを反映 — `cli/gh-extension-precompile@v2` 採用 / entry shim 廃止 / `main.go` をリポルートに / `genqlient` を最初から導入 / build-skills をサブコマンド統合に変更
> - 2026-05-04 v3: Go 公式仕様レビューを反映 — Go 1.25 採用 / `go-cmp` を主テスト DSL に / `govulncheck` を CI 必須化 / `golangci-lint v2` 明示 / `go test -race` 必須化 / `tool` ディレクティブ (1.24+) で genqlient 管理 / 移行期間は機能追加 freeze / 内部矛盾（entry shim / ADR-0006 説明 / Phase 1 main.go パス）を解消

## 1. 動機

ADR-0001（Bun `--compile`）と ADR-0003（Octokit）は当時の判断として妥当だったが、`gh extension` という製品ジャンルにおいて以下のミスマッチが顕在化している:

1. **`cli/go-gh` ライブラリの非利用**: `gh auth` トークン解決・GraphQL/REST client・host 解決・user agent 設定を手動配線している（ADR-0003 の "企業 SSO ヘッダ継承不能" Negative はこれが原因）。go-gh はこれらすべてを `gh` 本体と同じ経路で解決する。
2. **配布物特性の劣後**: バイナリサイズ ~50MB（Go なら ~10MB）、cold start 30-100ms（Go なら 5-20ms）。`gh extension install` の UX に直結。
3. **クロスコンパイル成熟度**: ADR-0001 自身が「Bun のクロスコンパイルはやや若い」と認めている。Go の cross-compile は何年も枯れている。
4. **`gh extension` エコシステム整合性**: `gh extension create --precompiled=go` が公式パス、`gh-dash` / `gh-poi` 等の事実上の標準実装も Go。

## 2. 移行範囲

### 2.1 In scope（Go へ書き換え）

| 対象 | 現状 | 移行後 |
| --- | --- | --- |
| CLI 本体 | `packages/gh-tasks/src/` (~9,286 行 TS) | `main.go`（リポルート）+ `cmd/`（cobra コマンド）+ `internal/`（ドメイン） |
| GraphQL / REST | Octokit | `cli/go-gh/v2/pkg/api` |
| i18n | TS + JSON catalogs | `nicksnyder/go-i18n/v2` + `embed` JSON |
| Skill build | `scripts/build-skills.mjs` (155 行) | `gh tasks build-skills` サブコマンド（`Hidden: true`）、`cmd/build_skills.go` |
| Adapter pipeline | `scripts/adapters/*.mjs` (158 行) | `internal/adapters/` (Go) |
| ハードコード i18n チェック | `scripts/check-no-hardcoded-i18n.mjs` | Go ベースの linter（or shell + ripgrep） |
| Frontmatter parse | `scripts/lib/frontmatter.mjs` | `goccy/go-yaml` |
| ビルド・リリース | `bun build --compile` 5 target | **`cli/gh-extension-precompile@v2`**（公式 Action） + `go build` |
| テスト | Vitest | 標準 `testing` + `google/go-cmp` |
| Lint / Format | Biome | `gofmt` + `golangci-lint` |
| ローカル CLI dev | `pnpm run build` (host-only) | `go build` または `go run` |

### 2.2 Out of scope（撤廃 or 維持）

| 対象 | 扱い | 理由 |
| --- | --- | --- |
| `skills/{name}/SKILL.md` | **そのまま維持** | 言語非依存の markdown SSOT。ADR-0004 のフロントマター規約も維持 |
| `docs/manual/` / `docs/adr/` / `docs/design/` | **そのまま維持** | コンテンツ層、言語非依存 |
| `lefthook` | 維持（YAML 設定） | 言語非依存の hook ランナー、Go バイナリも呼べる |
| `commitlint` | **撤廃** or **CI のみ Node** | 移行当初は CI で残し、将来は Go 製の代替（[`go-conventional-commits`](https://github.com/conventionalcommits) 系）に置換可。判断を将来に持ち越す |
| `markdownlint-cli2` | 維持（CI のみ Node 一時許容） | Go の代替が薄い。長期的には `mdformat` / 手書きルールへ移行検討 |
| `yamllint` / `yamlfmt` | 維持（Python / Go） | 既に言語横断 |
| `mise` | 維持 | toolchain pin、Go バージョン pin に活用 |
| `pnpm` / `package.json` / `pnpm-workspace.yaml` / `tsconfig*.json` | **撤廃** | TS 撤廃に伴い不要 |
| `@biomejs/biome` / `vitest` / `tsdown` 関連 | **撤廃** | TS 撤廃に伴い不要 |

## 3. ディレクトリ構成（移行後）

`gh extension create --precompiled=go` の正規構造に準拠（`main.go` はリポルート、`cmd/` は cobra コマンド定義、`internal/` はドメインロジック）。

```text
gh-tasks/
├── main.go                   # cobra root エントリ（リポルート、gh extension 慣習）
├── cmd/                      # cobra コマンド定義
│   ├── root.go               # rootCmd + SilenceUsage/SilenceErrors + 共通フラグ
│   ├── add.go                # gh tasks add
│   ├── list.go               # gh tasks list
│   ├── today.go / done.go / standup.go / review.go
│   ├── plan.go / triage.go / link.go
│   ├── projects.go / projects_init.go / projects_init_templates.go
│   └── build_skills.go       # gh tasks build-skills (Hidden: true)
├── internal/
│   ├── github/               # go-gh ラッパ（auth + GraphQL client）
│   │   ├── client.go
│   │   └── queries/          # genqlient .graphql + 生成 .go
│   ├── scope/                # 3-scope 抽象 (repo/org/user)
│   ├── project/              # Projects v2 ロジック
│   ├── projectitem/
│   ├── period/               # 日次 / 週次 / sprint
│   ├── repo/
│   ├── config/               # gh-tasks 設定読み込み
│   ├── i18n/                 # go-i18n ローダ + en/ja JSON (embed)
│   ├── skills/               # SKILL.md パーサ + frontmatter
│   └── adapters/             # 4 agent (claude/codex/copilot/gemini) 出力
├── genqlient.yaml            # ← 新規（GraphQL 型生成設定）
├── skills/               # ★ skill SSOT (markdown)、移行対象外
├── docs/                     # ★ そのまま
├── dist/                     # ★ adapter 出力先（Go から書き出す）
├── .claude/ .agents/         # ★ そのまま
├── .github/workflows/release.yaml  # cli/gh-extension-precompile@v2
├── go.mod / go.sum
├── lefthook.yaml             # ★ 維持（フック内容を Go 向けに書換）
├── mise.toml                 # ★ 維持（go バージョン追記）
└── （★ 旧 `gh-tasks` bash shim は削除 — gh extension 機構が manifest.yml でプラットフォーム別バイナリを解決）
```

## 4. 依存ライブラリ選定

| 用途 | 採用 | 理由 |
| --- | --- | --- |
| CLI フレームワーク | **`spf13/cobra`** | gh 本体と同じ。subcommand 設計・補完生成が枯れている |
| GitHub API | **`cli/go-gh/v2/pkg/api`** | gh 認証（keyring 含む）完全継承、REST + GraphQL、host 解決、cache/timeout ビルトイン |
| Auth トークン解決 | **`cli/go-gh/v2/pkg/auth`** | `auth.TokenForHost` が `GH_TOKEN` / `GITHUB_TOKEN` / `GH_ENTERPRISE_TOKEN` / `oauth_token` / system keyring を統合解決 |
| Repository 検出 | **`cli/go-gh/v2/pkg/repository`** | `GH_REPO` / git remote / 既知 host から `Current()` で自動解決 |
| Go バージョン | **Go 1.25**（`go.mod` の `go` 行）+ `toolchain go1.26.0` | サポート対象は 1.25 / 1.26。最小要件を 1.25 に固定し、新機能利用時は toolchain 行で 1.26 を要求 |
| GraphQL 型生成 | **`Khan/genqlient`**（最初から導入、`go.mod` の `tool` ディレクティブで管理） | Projects v2 は GraphQL only で 10+ クエリ規模。手書き struct はフィールド追加時の追従コストが高い。Go 1.24+ の `tool` ディレクティブで `tools.go` の blank import 不要 |
| 設定 | 環境変数 + 構造体 | viper は不採用。`gh-tasks` は設定ファイルを持たない（gh ext 慣習） |
| i18n | **`nicksnyder/go-i18n/v2`** | go-i18n は最も枯れた選択肢、JSON catalog 互換 |
| i18n キー定数 | 自前 `internal/i18n/keys.go` | TS の型推論代替。`const KeyErrorScopeInvalid = "error.scope.invalid"` を集約 |
| ロギング | **`log/slog`**（標準、Go 1.21+） | デバッグ / verbose 出力。サードパーティ logger は不採用 |
| YAML / Frontmatter | **`goccy/go-yaml`** | パフォーマンス・エラーメッセージが優位 |
| Markdown 解析 | `yuin/goldmark` | frontmatter 抽出は手書きで十分、本文 AST が必要なら goldmark |
| テンプレート | **`text/template`** （標準） | adapter 出力に十分 |
| Embed | **`embed`**（標準） | i18n JSON、SKILL テンプレートを bin に同梱 |
| テスト | **標準 `testing`** + **`google/go-cmp/cmp`**（diff）+ `testify/require`（fail-fast assertion のみ） | Go コミュニティのトレンド: testify を避けて `go-cmp` でリッチな diff 出力。`require` は致命エラー時のみ |
| HTTP モック | **`httptest`**（標準）+ go-gh の test helper | MSW 相当は不要 |
| **リリース** | **`cli/gh-extension-precompile@v2`**（**公式 Action**） | gh extension 専用、`go_version_file: go.mod` 1 行、命名規則 `gh-<name>_<version>_<os>-<arch>` 自動準拠、`generate_attestations: true` で SLSA、`gpg_fingerprint` で署名。**v1 は新 Go と非互換、必ず v2** |
| **Lint** | **`golangci-lint v2`** + `golangci-lint-action@v8` | v1 は 2025 以降メンテのみ。設定は `.golangci.yml` に `version: "2"` ヘッダ必須、`linters.default: standard` を起点に `revive` / `errorlint` / `gosec` / `bodyclose` / `paralleltest` を追加。`formatters` セクションで `gofumpt` + `goimports` + `gci` |
| **脆弱性スキャン** | **`govulncheck`**（公式、`golang.org/x/vuln/cmd/govulncheck`） | コールグラフ追跡で実到達のみ報告。CI 必須。`-format=sarif -mode=source` で Code Scanning 連携、`text` モードで `go test` ジョブ内 fail-fast |

### Cobra ベストプラクティス（rootCmd 必須設定）

```go
var rootCmd = &cobra.Command{
    Use:           "gh-tasks",
    SilenceUsage:  true,   // エラー時に Usage を出さない
    SilenceErrors: true,   // 自前で表示
}

func main() { cobra.CheckErr(rootCmd.Execute()) }
```

- ハンドラ内で **`fmt.Println` 禁止**、必ず `cmd.Println(...)` または `fmt.Fprintln(cmd.OutOrStdout(), ...)`（テスト時 `SetOut` で捕捉可能にする）
- `Run` ではなく **`RunE`** を使い、エラーは `return fmt.Errorf("...: %w", err)` で wrap
- 動的補完は `ValidArgsFunction` / `RegisterFlagCompletionFunc`（scope / period / repo 補完で活用）
- エラー検査は **`errors.Is` / `errors.As`**（`==` 比較禁止）、`%w` ラップを徹底
- **`context.Context` を関数第 1 引数で受け取る**（構造体に格納しない、HTTP ハンドラなら `r.Context()`）
- `WithTimeout` / `WithCancel` の `defer cancel()` 必須

## 5. ADR 更新計画

| ADR | アクション | 内容 |
| --- | --- | --- |
| ADR-0001（Bun --compile） | **Superseded** | 後継 ADR-0006 で Go 1.25 + cobra + `cli/gh-extension-precompile@v2` を記録 |
| ADR-0002（i18n 日本語 SSOT） | 既に Superseded（ADR-0005 後継）。変更なし | — |
| ADR-0003（Octokit） | **Superseded** | 後継 ADR-0007 で `cli/go-gh` 採用を記録 |
| ADR-0004（skill frontmatter schema） | **そのまま** | 言語非依存。Go の YAML パーサで読む |
| ADR-0005（i18n reader-based SSOT） | **そのまま** | en SSOT + ja translation の方針は維持。実装は go-i18n |
| **ADR-0006（new）** | 新規起票 | Go 1.25 + cobra + **`cli/gh-extension-precompile@v2`** を採用、ADR-0001 を Superseded |
| **ADR-0007（new）** | 新規起票 | `cli/go-gh` で auth + GraphQL を統合、ADR-0003 を Superseded |
| **ADR-0008（new）** | 新規起票 | Go テスト / 品質チェーン: `testing` + `go-cmp` + `golangci-lint v2` + `govulncheck` + `go test -race` |

## 6. フェーズ計画

すべて **新ブランチで並走実装**。本ブランチに切り替わるのは Phase 7 のカットオーバー時のみ。

### Phase 0: 設計・ADR 起票（1-2 日）

- [ ] ADR-0006 / 0007 / 0008 起票
- [ ] `main.go` + `cmd/` + `internal/` のスケルトン作成
- [ ] `go.mod` 初期化（`go 1.25` + `toolchain go1.26.0`）、CI に Go matrix を追加
- [ ] `.golangci.yml` 作成（`version: "2"`、`linters.default: standard` + 主要 linter、`formatters` で gofumpt + goimports + gci）
- [ ] `genqlient.yaml` 初期化（schema.graphql は GitHub 公開 schema をフェッチ）、`go.mod` の `tool` ディレクティブで genqlient を管理
- [ ] Renovate 設定に Go module preset を追加
- [ ] **mise.toml** に `go = "1.25"` / `golangci-lint = "v2.12"` / `govulncheck = "latest"` を追加（precompile-action は `go_version_file: go.mod` で同期）
- [ ] **機能追加 freeze** を宣言（`main` への TS 側 PR は致命バグ修正のみ許可、Phase 7 のカットオーバーまで継続）
- [ ] リポトピックに `gh-extension` を追加（`gh ext search` で発見性向上）

### Phase 1: コア基盤（3-5 日）

- [ ] `internal/i18n` 実装（go-i18n + en/ja.json embed）
- [ ] `internal/github` 実装（go-gh ラッパ）
- [ ] `internal/scope` 実装（3-scope 抽象）
- [ ] `internal/config` 実装
- [ ] `internal/period` 実装
- [ ] `internal/project` / `internal/projectitem` 実装
- [ ] CLI エントリ **`main.go`（リポルート）** + `cmd/root.go`（cobra root + `SilenceUsage` / `SilenceErrors`）
- [ ] **テストでの振る舞い保証**: 既存 TS テストの assertion を Go テストへ移植（`go-cmp` で diff 出力、テーブルドリブン + `t.Parallel()`）
- [ ] **`go test -race ./...`** が CI で常時 green

### Phase 2: コマンド移植（5-7 日）

各コマンドを 1 つずつ移植、振る舞いパリティをテストで保証:

- [ ] `list` / `today` / `done` （read-only 系から先に）
- [ ] `standup` / `review`
- [ ] `add` / `link`
- [ ] `plan` / `triage`
- [ ] `projects` / `projects-init` / `projects-init-templates`

### Phase 3: skill build + adapter pipeline（3-4 日）

- [ ] `cmd/build_skills.go` 実装（`gh tasks build-skills` サブコマンド、`Hidden: true`）
- [ ] `internal/skills/` 実装（frontmatter 抽出、SKILL.md / SKILL.en.md ペア解決）
- [ ] `internal/adapters/{claude,codex,copilot,gemini}.go` 実装
- [ ] golden file テストで dist/ 出力をスナップショット固定
- [ ] 既存 TS ビルド出力との diff 検証（CI で担保）

### Phase 4: ハードコード i18n チェック（1 日）

- [ ] `scripts/check-no-hardcoded-i18n.mjs` を Go 移植 or `ripgrep + shell` の薄い実装へ
- [ ] CI / lefthook に組み込み

### Phase 5: ビルド・品質ゲート・リリース基盤（2-3 日）

- [ ] `.github/workflows/release.yaml` を `cli/gh-extension-precompile@v2` ベースに書き換え
  - `go_version_file: go.mod`
  - `generate_attestations: true`（SLSA）
  - `gpg_fingerprint:`（GPG 鍵が GitHub Org Secrets にあれば）
  - `release_android` は付けない（v2 デフォルト無効）
- [ ] `.github/workflows/ci.yaml` の Go ジョブ
  - `actions/setup-go@v5`（`go-version-file: go.mod` で同期）
  - `go test -race -shuffle=on ./...`
  - `go vet ./...`
  - `golangci/golangci-lint-action@v8`（`version: v2.12`、`args: --timeout 5m`）
  - `govulncheck ./...`（text モード、脆弱性検出で fail-fast）
  - 別ステップで `govulncheck -format=sarif -mode=source -scan=symbol ./...` → `github/codeql-action/upload-sarif@v3` で Code Scanning 連携
- [ ] **旧 `gh-tasks` bash entry shim を削除**（gh ext 機構が manifest.yml で OS/Arch 別バイナリを解決するため不要）
- [ ] `gh extension install . --force`（カレントディレクトリ）で動作確認、dogfooding はこのパターンで実施
- [ ] `gh extension install ozzy-labs/gh-tasks --pin v2.0.0-rc.1` でリモート install 確認（rc タグはハイフン付きで自動 prerelease 判定）

### Phase 6: ドキュメント反映（1 日）

- [ ] README.md / README.ja.md 更新
- [ ] docs/manual/{en,ja}/ の build / install 手順更新
- [ ] docs/design/architecture.md 更新（バイナリビルド section）
- [ ] AGENTS.md / CLAUDE.md の Tech Stack section 更新
- [ ] `docs/design/release-process.md` 更新

### Phase 7: カットオーバー（1 日）

- [ ] `packages/gh-tasks/` 削除
- [ ] `scripts/` 削除
- [ ] `package.json` / `pnpm-lock.yaml` / `pnpm-workspace.yaml` / `tsconfig*.json` 削除
- [ ] `.biomeignore` / `biome.json` 削除
- [ ] 旧 `gh-tasks` bash entry shim 削除
- [ ] `lefthook.yaml` のフック内容を Go 化（pre-commit: `gofmt` + `golangci-lint --fix`、pre-push: `go test ./...`）
  - commitlint / markdownlint は CI ジョブのみで実行（dev hook からは外す、Node 依存を dev に持ち込まない）
- [ ] v2.0.0-rc.1 リリース（precompile-action がハイフンタグを自動 prerelease 判定）、フィードバック収集
- [ ] v2.0.0 GA

**目安合計: 17-24 日**（1 人 fullspeed 想定。Phase 0 で機能 freeze 済の前提）。

## 7. 並走戦略

- **ブランチ戦略**: `refactor/migrate-to-go` の単一 long-lived ブランチに各 Phase の小 PR を merge していく（GitHub Flow + squash merge は維持）。`main` は最後のカットオーバーまで TS 版のまま。
- **機能 freeze**: Phase 0 開始時点から Phase 7 まで、`main` への TS 側 PR は **致命バグ修正のみ許可**。新機能・refactor は Go 側にのみ実装する（ダブルメンテを避ける）。
- **dogfooding**: 開発中の動作確認は `gh extension install . --force` でカレントブランチを直接 install。旧 `gh-tasks` bash shim は使わない（Phase 5 で削除）。
- **互換性**: コマンド体系・フラグ・出力フォーマット（特に i18n キー）はパリティ。互換 break は許容しないが、ADR-0005 の en SSOT 方針に沿った文言改善は許容。

## 8. リスクと緩和策

| リスク | 影響 | 緩和策 |
| --- | --- | --- |
| Go の i18n が TS ほど型安全でない | 翻訳キー欠落の検出が弱まる | `internal/i18n/keys.go` でキー定数を集約、`go-i18n` の `bundle.MustLoadMessageFile` 起動チェック、`scripts/check-no-hardcoded-i18n` の Go 移植で CI gate |
| `cli/go-gh` の API 安定度 | 破壊的変更のリスク | v2 系を pin、CHANGELOG を Renovate で監視。`cli/cli` 本体と同じ経路なので大規模 break は稀 |
| GraphQL スキーマ変更時の追従コスト | クエリ書換が必要 | `genqlient` でスキーマ変更時に型エラーで早期検出、`schema.graphql` を定期 Renovate で更新 |
| skill bundle 出力の差分 | adapter ファイルが意図せず変わる | golden file テストで dist/ 出力を CI で固定 |
| 移行中の機能追加凍結による外圧 | バックログ膨張 | Phase 0 で freeze を周知。期間 17-24 日で集中、致命バグのみ TS 側にも反映 |
| Bun --compile 由来の挙動依存 | shebang / file path 解決 / TZ 等の差異 | Phase 2 / Phase 5 の実機検証で潰す。`gh extension install . --force` で dogfooding |
| precompile-action が想定外のターゲットを生成 / 落とす | 配布バイナリ欠落 | `gh extension install` をリリース直後に matrix CI で実行する smoke test ジョブを追加 |
| `govulncheck` の false positive で CI が止まる | リリースブロック | `reflect` 経由は仕様上の限界。SARIF を Code Scanning に流して dismiss 運用、もしくは `paths-ignore` で対象外指定 |
| `golangci-lint v2` の linter 仕様変更 | 突然の error 量増 | `version: "2"` 固定、Renovate で `golangci-lint-action@v8` のバージョン管理。`linters.exclusions.warn-unused: true` で陳腐化検出 |
| markdownlint / commitlint が Node 必須 | toolchain 完全脱 Node にならない | CI ジョブでのみ Node を使用。dev hook からは外し、Go 開発体験には影響させない |

## 9. 完了条件（DoD）

- [ ] `packages/gh-tasks/` および `scripts/` ディレクトリが削除されている
- [ ] `package.json` `pnpm-lock.yaml` `tsconfig*.json` `biome.json` `lefthook.yaml` の Node 系 hook 部分が撤去済
- [ ] 旧 `gh-tasks` bash entry shim が削除されている
- [ ] `gh extension install ozzy-labs/gh-tasks` が precompile-action 出力のすべての OS/Arch で動作（少なくとも darwin x64 / arm64、linux x64 / arm64、windows x64 / arm64）
- [ ] 全コマンド（`list / today / done / standup / review / add / link / plan / triage / projects*`）が TS 版とパリティで動作
- [ ] `dist/{adapter}/` 出力が移行前後で diff なし（フォーマット差を除く）
- [ ] **CI green**:
  - `go test -race -shuffle=on ./...`
  - `go vet ./...`
  - `golangci-lint run --timeout 5m`（v2 設定）
  - `govulncheck ./...`
  - lint:md / lint:yaml / lint:i18n（Node CI ジョブで継続）
  - `cli/gh-extension-precompile@v2` のリリースビルド smoke test
- [ ] バイナリサイズが 15MB 以下、cold start が 30ms 以下（実測）
- [ ] ADR-0006 / 0007 / 0008 が Accepted ステータス、ADR-0001 / 0003 が Superseded
- [ ] README / docs/manual / AGENTS.md / CLAUDE.md が Go 前提で更新されている
- [ ] リポトピックに `gh-extension` が付いている

## 10. 判断事項（公式仕様レビュー後の決定）

| # | 課題 | 決定 | 理由 |
| --- | --- | --- | --- |
| 1 | GraphQL 型: 手書き vs genqlient | **`Khan/genqlient` を最初から導入** | Projects v2 は GraphQL only で 10+ クエリ規模。手書きは Phase 2 で型ズレが頻発。go-gh GraphQL client と統合可能 |
| 2 | commitlint / markdownlint の置換 | **Node を CI ジョブのみで残す**（dev hook からは外す） | Go 代替が薄い（commitlint-rs は Rust、markdownlint の Go 代替なし）。dev 体験への悪影響を避けるため pre-commit には組み込まない。撤廃判断は将来 ADR |
| 3 | build-skills を独立 vs サブコマンド | **`gh tasks build-skills` サブコマンド統合**（`Hidden: true`） | 単一バイナリ distribution の利点が活きる。CI dogfooding 用に `--check-diff` フラグ |
| 4 | frontmatter バリデーションの独立 CLI 化 | **当面は `internal/skills/` の関数のまま**（YAGNI） | 他 agent ecosystem で再利用ニーズが顕在化してから切り出す |

## 11. 公式仕様準拠チェックリスト

`gh extension` / Go / Cobra / `cli/go-gh` の公式パスから外れていないことを確認するチェックリスト。

### gh extension

- [x] リポ名 `gh-tasks` が `gh-` プレフィックス
- [x] リリースアセット命名 `gh-tasks_<version>_<os>-<arch>[.exe]` を precompile-action に任せる
- [x] `cli/gh-extension-precompile@v2` を採用（v1 は新 Go と非互換）
- [x] リポトピックに `gh-extension` を付ける
- [x] `gh-` プレフィックスのコア衝突回避（`gh tasks` は本体未使用）
- [x] CI install 例には `--pin <tag>` を必須化
- [x] entry shim は廃止（precompile-action が manifest.yml を生成）

### Cobra

- [x] `main.go` をリポルート、`cmd/` にコマンド定義、`internal/` にドメイン
- [x] `SilenceUsage: true` / `SilenceErrors: true`
- [x] `RunE` + `cobra.CheckErr(rootCmd.Execute())`
- [x] ハンドラ内 `cmd.Println` / `cmd.OutOrStdout()`（`fmt.Println` 禁止）
- [x] 動的補完 `ValidArgsFunction` / `RegisterFlagCompletionFunc`
- [x] `--version` は `rootCmd.Version` 設定で自動

### `cli/go-gh`

- [x] v2 系を採用（`github.com/cli/go-gh/v2/...`）
- [x] `auth.TokenForHost(host)` で keyring 含む完全解決（手動 `GH_TOKEN` 配線しない）
- [x] `auth.DefaultHost()` で host 解決（`GH_HOST` / config 自動考慮）
- [x] `repository.Current()` で現在リポ検出
- [x] `api.NewGraphQLClient` の `EnableCache` / `CacheTTL` / `Timeout` を活用
- [x] Enterprise Server 対応は `IsEnterprise(host)` で判定可能

### Go 言語慣習

- [x] `go.mod` の `go 1.25` + `toolchain go1.26.0`
- [x] エラーは `%w` ラップ + `errors.Is` / `errors.As`（`==` 比較禁止）
- [x] `context.Context` は関数第 1 引数で受け取り、構造体に格納しない
- [x] `defer cancel()` 必須（`WithTimeout` / `WithCancel`）
- [x] テストは table-driven + `t.Parallel()` + `go-cmp` の diff
- [x] `go test -race -shuffle=on ./...` を CI で常時実行

### 品質ゲート

- [x] `golangci-lint v2`（`version: "2"`、`golangci-lint-action@v8`）
- [x] `govulncheck` を CI 必須化（text モードで fail-fast、別途 SARIF を Code Scanning へ）
- [x] `gofumpt` + `goimports` + `gci` をフォーマッタとして採用（`formatters` セクション）

## References

- [`cli/go-gh`](https://github.com/cli/go-gh) — GitHub CLI 公式 Go ライブラリ（v2 系を採用）
- [`cli/gh-extension-precompile`](https://github.com/cli/gh-extension-precompile) — Go 拡張のクロスコンパイル + リリース公式 Action（**v2 必須**）
- [Creating GitHub CLI extensions](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions)
- [`spf13/cobra`](https://github.com/spf13/cobra)
- [`Khan/genqlient`](https://github.com/Khan/genqlient) — GraphQL 型生成
- [`nicksnyder/go-i18n`](https://github.com/nicksnyder/go-i18n)
- [`golangci-lint`](https://golangci-lint.run/) — v2 必須
- [`govulncheck`](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) — Go 公式脆弱性スキャナ
- [`google/go-cmp`](https://github.com/google/go-cmp) — テスト diff
- ADR-0001 / 0003 / 0005（本計画で更新対象）
