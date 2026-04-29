# 0003. GraphQL は Octokit 経由で叩き、`gh api graphql` shell-out は採用しない

- Status: Accepted
- Date: 2026-04-30
- Deciders: ozzy
- Tags: cli, github-projects, graphql, auth

## Context

`gh tasks` の中核は GitHub Projects v2 GraphQL API へのアクセス([handbook ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md) Decision 1)。GraphQL クライアントの実装方法に 2 案ある。

1. **Octokit GraphQL クライアント**(`@octokit/graphql`)を直接使う
2. `gh api graphql` への shell-out で `gh` のリクエスト処理を再利用する

認証は handbook ADR-0022 で「`gh auth login` 取得済トークンを `GH_TOKEN` 環境変数で extension が継承」と確定済。トークン取得導線は固定なので、本 ADR は「取得済トークンで GraphQL を叩く方法」を扱う。

## Decision

CLI 内部の GraphQL リクエストは **Octokit GraphQL クライアント(`@octokit/graphql`)** を直接使用する。`gh api graphql` への shell-out は採用しない。

- 認証: gh extension 起動時に `GH_TOKEN` 環境変数経由で受け取ったトークンを Octokit に渡す
- クエリ集約: `packages/gh-tasks/src/lib/queries/` に GraphQL クエリ群を配置
- エラー型: `@octokit/request-error` を使用、HTTP / GraphQL エラーを区別して i18n キー(`error.graphql.*`)へ写像

例外: `gh tasks list --interactive` 等で人間視認性のある TTY 出力が欲しい場面のみ、`gh issue list` 等を子プロセスで起動して出力をパイプ(GraphQL クエリではなく gh の整形出力を借りる目的)。

## Consequences

### Positive

- TypeScript 型の恩恵: GraphQL レスポンスをそのまま型付け可能(`@octokit/graphql` の generic、または別途 codegen)
- パフォーマンス: shell プロセス起動コスト(`gh` 呼び出し毎に ~50-100ms)を回避
- レスポンスキャッシュ: HTTP 層で in-memory キャッシュを差し込み可能(`gh api` だと不能)
- エラー処理: `RequestError` で HTTP / GraphQL エラーを構造化、リトライポリシーや i18n エラーメッセージへの写像が容易
- テスト: MSW 等で HTTP モックが可能、shell-out だと統合テスト難

### Negative / Trade-offs

- `gh` 既存の認証フロー(企業 SSO 等の追加ヘッダ)を完全には継承できない可能性がある — 顕在化したら本 ADR を再検討
- GraphQL スキーマ変更時、`gh api` だとサーバ側追従に乗れるが、Octokit ではクエリ書換えが必要。スキーマ変更は repo-internal ADR で記録する運用とする
- バンドルに `@octokit/graphql` を含めるためバイナリサイズが微増(数 MB)

## Alternatives considered

- **`gh api graphql` shell-out** — `gh` の認証/プロキシ設定を完全継承できる利点はあるが、型/キャッシュ/テスト/エラー処理がすべて劣る。不採用
- **`fetch` + 手書きリクエスト** — `@octokit/graphql` の auth ヘッダ / リトライ / エラー型を再実装するコストが見合わない。不採用
- **`@octokit/rest`(REST API)** — Projects v2 は GraphQL 専用、REST API では取れないフィールドが多い。不採用

## References

- Related handbook ADR: [ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md)(Decision 1: 認証は `GH_TOKEN` 継承)
- Related design review: [reviews/2026-04-30-gh-tasks-design.md](https://github.com/ozzy-labs/handbook/blob/main/reviews/2026-04-30-gh-tasks-design.md) 6
- External: [Octokit graphql.js](https://github.com/octokit/graphql.js)、[GitHub Projects v2 GraphQL](https://docs.github.com/en/graphql/reference/objects#projectv2)
