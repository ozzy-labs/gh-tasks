# 0008. Go のテスト・品質チェーン: `testing` + `go-cmp` + `golangci-lint v2` + `govulncheck`

- Status: Accepted
- Date: 2026-05-04
- Deciders: ozzy
- Tags: testing, lint, security, ci, go

## Context

ADR-0006 で CLI を Go へ完全移行することを決めた。Go 側のテスト DSL・lint・脆弱性スキャンの選定が必要になる。Go コミュニティの 2025-2026 トレンドは:

- **`testify` 離れ**: assertion DSL より標準 `testing` + `google/go-cmp` の diff 出力の方がリッチで、依存も浅い
- **`golangci-lint v2`**: v1 は 2025 以降メンテのみ。v2 は設定スキーマが変わり `version: "2"` ヘッダ必須、`linters.default: standard` を起点にプリセット運用
- **`govulncheck`**: Go 公式の脆弱性スキャナ。コールグラフ追跡で実到達のみ報告するため false positive が少ない、Code Scanning 連携可

加えて、Go の `testing` パッケージは並列実行と data race 検出を組み込みでサポートする。Go 1.17+ の `-shuffle=on` でテスト順序依存も検出できる。これらを CI 必須にするべきかが論点。

## Decision

Go の品質チェーンを次の構成に固定する。

### テスト DSL

- **標準 `testing`** + **`google/go-cmp/cmp`**（diff 出力、`cmp.Diff` 主体）
- **`testify/require`** は致命エラー時の fail-fast assertion のみに限定（`require.NoError(t, err)` 等）。`assert` は使わない
- **テーブルドリブン** + **`t.Parallel()`** をデフォルトに（`paralleltest` linter で強制）
- **`t.Run` のサブテスト名**は kebab-case で揃える（`go test -run` でフィルタしやすい）

### CI 必須項目

- `go test -race -shuffle=on ./...` — data race + テスト順序依存を CI で常時検出
- `go vet ./...` — 標準静的解析
- `golangci-lint run --timeout 5m`（v2 設定）
- `govulncheck ./...`（text モード、脆弱性検出で fail-fast）
- `govulncheck -format=sarif -mode=source -scan=symbol ./...` → `github/codeql-action/upload-sarif@v3` で Code Scanning 連携

### golangci-lint v2 設定（`.golangci.yml`）

```yaml
version: "2"
linters:
  default: standard
  enable: [revive, errorlint, gosec, bodyclose, paralleltest, misspell, gocritic]
  exclusions:
    warn-unused: true   # 陳腐化した除外ルールを検出
formatters:
  enable: [gofumpt, goimports, gci]
```

- `linters-action`: `golangci/golangci-lint-action@v8`（v8 は v2 サポート、`version: latest` で最新追従）
- `formatters` セクションで `gofumpt` + `goimports` + `gci` を一元管理（`run --fix` で適用可能）

### Hooks

- **pre-commit**: `golangci-lint run --fix` のみ(`gofumpt` を含む `formatters` セクションを v2 が一括適用するため、別途の `gofmt -l` は競合 / 二重実行になり外す)
- **pre-push**: `go test ./...`(`-race` は CI でのみ実行、ローカルは速度優先) + branch 名チェック(`<type>/<slug>` 形式の検証、`main` 直接 push を防ぐ)

## Consequences

### Positive

- `cmp.Diff` のリッチな diff 出力で、testify より失敗時の調査効率が高い
- `paralleltest` を強制することで、テストが意図せず順次実行になり遅くなる事態を防げる
- `-race` + `-shuffle=on` の組み合わせで、共有 state の race / 順序依存バグが CI で早期検出される
- `govulncheck` のコールグラフ追跡により false positive が少ない、CI を止めにくい
- `golangci-lint v2` の `formatters` セクションで `go fmt` が単一の真実ソースになり、エディタごとの揺れが消える

### Negative / Trade-offs

- testify からの移行コスト — 本リポは TS 由来なので影響は限定的、新規コードのみ go-cmp で書く
- `golangci-lint v2` は v1 と設定互換性なし — 公式 [migration guide](https://golangci-lint.run/product/migration-guide/) を参考に最初から v2 で書く
- `govulncheck` の SARIF 出力は `reflect` 経由など仕様上の限界がある — false positive は Code Scanning 上で dismiss、必要なら `paths-ignore` で対象外指定

## Alternatives considered

- **`testify` 全面採用** — assertion DSL は冗長、`assert.Equal` の diff 出力は `cmp.Diff` に劣る。コミュニティもトレンドから外れている。不採用
- **`golangci-lint v1` 据え置き** — メンテのみ、新 linter 追加なし。新規プロジェクトで v1 を選ぶ理由はない。不採用
- **`gosec` を独立で実行** — `golangci-lint` に統合した方が CI ジョブ数が減る。`gosec` は v2 の `enable` リストで有効化
- **`Snyk` / `Trivy` を追加** — `govulncheck` が Go 専用に最適化、コールグラフ追跡で精度が高い。サードパーティ追加はコストに見合わない。不採用
- **race detector を CI でスキップ** — race は本番で顕在化すると致命的。CI で 5-10% の追加実行時間は許容

## References

- Related repo ADR: [ADR-0006](./0006-go-and-cobra-migration.md), [ADR-0007](./0007-go-gh-graphql-client.md)
- Related design doc: [`docs/design/go-migration-plan.md`](../design/go-migration-plan.md)
- External: [`google/go-cmp`](https://github.com/google/go-cmp), [`golangci-lint` v2 migration](https://golangci-lint.run/product/migration-guide/), [`govulncheck`](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck), [Go testing guidelines](https://go.dev/wiki/TestComments)
