# 0006. CLI を Go 1.25 + cobra + `cli/gh-extension-precompile@v2` に完全移行する

- Status: Accepted
- Date: 2026-05-04
- Deciders: ozzy
- Tags: cli, build, language, distribution
- Supersedes: [ADR-0001](./0001-use-bun-compile-for-binary.md)

## Context

ADR-0001 は CLI 本体を TypeScript で書き、Bun `--compile` で単一バイナリ化する判断を下した。1 クォーター運用した結果、`gh extension` という製品ジャンルにおける以下のミスマッチが顕在化した。

1. **`cli/go-gh` の非利用**: gh 本体と同じ auth / GraphQL 経路に乗れず、ADR-0003 の "企業 SSO ヘッダ継承不能" Negative はこれが直接の原因
2. **配布物特性の劣後**: バイナリ ~50MB / cold start 30-100ms（Go なら ~10MB / 5-20ms）
3. **クロスコンパイル成熟度**: ADR-0001 自身が「Bun のクロスコンパイルはやや若い」と認めていた
4. **公式パスからの逸脱**: `gh extension create --precompiled=go` が正規ルート、`gh-dash` / `gh-poi` 等の事実上の標準も Go

詳細な動機・移行範囲・フェーズ計画は [`docs/design/go-migration-plan.md`](../design/go-migration-plan.md) を参照。

## Decision

CLI 本体・build / sync スクリプト・toolchain をすべて Go へ完全移行する。

- **言語**: Go — `go.mod` の `go 1.25` 行で最小要件、`toolchain go1.26.0` で新機能利用時の自動切替
- **CLI フレームワーク**: `spf13/cobra`（gh 本体と同じ、subcommand と補完が枯れている）
  - `SilenceUsage: true` / `SilenceErrors: true` を root に設定
  - ハンドラ内 `fmt.Println` は禁止し `cmd.OutOrStdout()` 経由で書く（テストで `SetOut` 捕捉可）
  - `Run` ではなく `RunE` を使い、エラーは `%w` ラップ
- **ディレクトリ構成**: `main.go`（リポルート）+ `cmd/`（cobra コマンド定義）+ `internal/`（ドメインロジック）
- **リリース**: GitHub 公式 Action `cli/gh-extension-precompile@v2` を採用（v1 は新 Go と非互換）
  - `go_version_file: go.mod` で setup-go と同期
  - `generate_attestations: true` で SLSA、`gpg_fingerprint` で署名
  - 命名規則 `gh-tasks_<version>_<os>-<arch>[.exe]` は Action が自動生成
- **エントリ shim 廃止**: 旧 `gh-tasks` bash shim は precompile-action が manifest.yml を生成するため不要、Phase 5 で削除

## Consequences

### Positive

- gh extension エコシステムの公式パスに整合（precompile-action / `cli/go-gh` / cobra / 公式 manifest）
- バイナリサイズ・cold start の大幅改善（実測目標: <15MB / <30ms）
- `cli/go-gh` 経由で gh 本体と同じ auth・host 解決・cache・user agent が手に入る（ADR-0003 の Negative を解消）
- Go の cross-compile / 標準 testing / `go vet` / `govulncheck` 等、ツールチェインが枯れている
- `gh extension install . --force` で dogfooding が成立、shim メンテが不要

### Negative / Trade-offs

- TS 中心の開発環境から Go 専用環境への切替コスト（17-24 日見込み）
- Go の i18n は TS ほど型安全でない — `internal/i18n/keys.go` のキー定数集約 + `scripts/check-no-hardcoded-i18n` の Go 移植で補う
- markdownlint / commitlint は Go 代替が薄く、CI ジョブで Node を残す（dev hook からは外す）

## Alternatives considered

- **Bun --compile 継続（現状維持）** — ADR-0001 の判断。`cli/go-gh` を使えず、配布物特性も劣後するため不採用
- **TypeScript + Node + shim** — ユーザーに Node 要求、起動が遅い。gh extension の標準 UX を損なう
- **Rust** — `gh extension` の事実上の標準は Go。エコシステム整合性で劣後、本体 (`cli/cli`) も Go なので公式ライブラリ供給が薄くなる
- **goreleaser** — gh extension 専用ではない汎用ツール。`cli/gh-extension-precompile@v2` の方がリリースアセット命名規則・SLSA 連携・gh ext 慣習に最適化されている

## References

- Related repo ADR: [ADR-0001](./0001-use-bun-compile-for-binary.md) (Superseded by this ADR), [ADR-0007](./0007-go-gh-graphql-client.md), [ADR-0008](./0008-go-test-and-quality-chain.md)
- Related design doc: [`docs/design/go-migration-plan.md`](../design/go-migration-plan.md)
- External: [gh extension precompile (v2)](https://github.com/cli/gh-extension-precompile), [Creating GitHub CLI extensions](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions), [`spf13/cobra`](https://github.com/spf13/cobra)
