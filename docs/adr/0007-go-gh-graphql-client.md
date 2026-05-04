# 0007. GitHub API は `cli/go-gh/v2` 経由で叩く

- Status: Accepted
- Date: 2026-05-04
- Deciders: ozzy
- Tags: cli, github-projects, graphql, auth, go
- Supersedes: [ADR-0003](./0003-graphql-via-octokit.md)

## Context

ADR-0006 で CLI 本体を Go へ完全移行する判断を下した。それに伴い、ADR-0003 で採用していた `@octokit/graphql` を Go の代替で置き換える必要がある。Go 側の選択肢は次の 3 つ:

1. **`cli/go-gh/v2/pkg/api`** — gh 本体と同じパスで auth / host / GraphQL / REST を統合
2. **`shurcooL/githubv4` + 自前 auth 配線** — タイプセーフな GraphQL クライアントだが auth は自前
3. **`gh api graphql` への shell-out** — ADR-0003 で却下した案。プロセス起動コストとテストの困難さは Go でも同じ

ADR-0003 の Negative セクションに「企業 SSO 等の追加ヘッダを完全には継承できない可能性がある — 顕在化したら本 ADR を再検討」と明記してあった。これは Octokit が gh 本体の認証経路（`gh auth login` で keyring 等に保存される token、`hosts.yml` の OAuth、`GH_ENTERPRISE_TOKEN`、追加ヘッダ）を解決できないためで、`cli/go-gh` はこの問題を構造的に解消する。

## Decision

Go CLI の GitHub API アクセスは **`cli/go-gh/v2/pkg/api`** に統一する。

- **GraphQL クライアント**: `api.NewGraphQLClient(api.ClientOptions{...})`
  - `EnableCache` / `CacheTTL` を活用（Octokit に MSW で別途差し込んでいた相当機能が標準）
  - `Timeout` を必須設定（`context.Context` と併用）
- **REST クライアント**: 必要時のみ `api.NewRESTClient`
- **Auth トークン解決**: `auth.TokenForHost(host)` — `GH_TOKEN` / `GITHUB_TOKEN` / `GH_ENTERPRISE_TOKEN` / `oauth_token` / system keyring を統合解決
- **Host 解決**: `auth.DefaultHost()`（`GH_HOST` / hosts.yml 自動考慮）、Enterprise は `auth.IsEnterprise(host)` で判定
- **Repository 検出**: `repository.Current()`（`GH_REPO` / git remote / 既知 host から自動）
- **GraphQL クエリ集約**: `internal/github/queries/` に `.graphql` ファイル + `Khan/genqlient` で型生成
  - `go.mod` の `tool` ディレクティブで genqlient を管理（Go 1.24+、`tools.go` の blank import 不要）
- **エラー処理**: `errors.Is` / `errors.As` + `%w` ラップ。`api.HTTPError` を i18n キー（`error.graphql.*`）へ写像

例外: `gh tasks list --interactive` 等で TTY 整形が欲しい場面のみ、`gh issue list` を子プロセスで起動して出力をパイプ（クエリではなく gh の整形を借りる目的）。

## Consequences

### Positive

- gh 本体と完全に同じ auth 経路 — keyring / 企業 SSO ヘッダ / Enterprise / トークンスコープ警告がそのまま継承される（ADR-0003 の主要 Negative を構造的に解消）
- `EnableCache` / `Timeout` / 既定 user agent / Retry が標準装備
- `genqlient` でスキーマ変更時に型エラーで早期検出、`schema.graphql` を Renovate で定期更新
- gh 本体と同じ HTTP transport なので、proxy 設定や Custom CA も流用可能
- `httptest` + `go-gh` の test helper で統合テストが容易

### Negative / Trade-offs

- `cli/go-gh` の API は安定だが破壊的変更ゼロではない — v2 系を pin、CHANGELOG を Renovate で監視
- GraphQL クエリの追加には `genqlient generate` の手順が要る — `pnpm` 風のショートカットを `go generate ./...` で提供
- `shurcooL/githubv4` のような構造体ベース DSL ではなく `.graphql` テキストを書く — ただし genqlient によりレスポンス型は完全に推論される

## Alternatives considered

- **`shurcooL/githubv4`** — 構造体ベースの GraphQL クエリ DSL でタイプセーフだが auth と host 解決を自前で書く必要がある。`cli/go-gh` の方が gh 本体との整合性で優位。不採用
- **`gh api graphql` shell-out** — `gh` 認証フローを完全継承できるが、プロセス起動コスト / 型 / cache / テスト容易性で不利。ADR-0003 の判断と同じ理由で不採用
- **生 `net/http` + 手書きリクエスト** — auth ヘッダ・リトライ・user agent を再実装するコストに見合わない。不採用
- **Octokit を Go から子プロセスで使う** — 単一バイナリ distribution の利点を毀損。論外

## References

- Related repo ADR: [ADR-0003](./0003-graphql-via-octokit.md) (Superseded by this ADR), [ADR-0006](./0006-go-and-cobra-migration.md)
- Related design doc: [`docs/design/go-migration-plan.md`](../design/go-migration-plan.md)
- External: [`cli/go-gh`](https://github.com/cli/go-gh), [`Khan/genqlient`](https://github.com/Khan/genqlient), [GitHub Projects v2 GraphQL](https://docs.github.com/en/graphql/reference/objects#projectv2)
