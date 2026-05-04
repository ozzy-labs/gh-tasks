# 0001. Bun `--compile` をバイナリビルドに採用する

- Status: Accepted
- Date: 2026-04-30
- Deciders: ozzy
- Tags: cli, build, language, distribution

## Context

`gh-tasks` を gh extension として配布するにあたり、実装言語を決める必要があった。要件は次のとおり:

1. クロスプラットフォーム単一バイナリ配布(darwin/linux/windows × amd64/arm64)— `gh extension install` で precompiled が選好される
2. GitHub GraphQL クライアントの型を再利用したい(Octokit、`@octokit/graphql`)
3. テスト基盤として Vitest を使う
4. 開発時の起動コストを最小化(REPL / dev mode で `bun run` 即起動)
5. 既存の TypeScript 中心開発環境と整合させ、新言語導入のコンテキストスイッチを避けたい

## Decision

CLI 本体は **TypeScript で記述し、Bun の `--compile` でクロスプラットフォーム単一バイナリを生成する**。

- Entry: `packages/gh-tasks/src/cli.ts`
- Release build: `bun build --compile --target=<triple> --outfile=bin/gh-tasks-{os}-{arch}`(`scripts/build-cli.mjs` 経由、`pnpm run build:all` で全 5 ターゲットを発行)
- Dev build: `pnpm run build`(`packages/gh-tasks/package.json`)はホスト 1 ターゲットのみを `bin/gh-tasks` に発行する開発用ショートカット。CI / release では使わない
- Targets: `bun-darwin-x64` / `bun-darwin-arm64` / `bun-linux-x64` / `bun-linux-arm64` / `bun-windows-x64`
- Release ワークフローは GitHub Releases に上記 5 バイナリを attach
- gh extension entry shim(リポ直下 `gh-tasks`)が `uname -s` / `uname -m` から該当バイナリを exec

## Consequences

### Positive

- TypeScript 中心の開発環境と整合
- Octokit / GraphQL の型を CLI / scripts / tests で再利用可能
- Node 互換 API のテストツール(Vitest)を CLI / scripts 双方で利用可能
- 開発時は `bun run src/cli.ts` で即起動、ビルド不要
- 単一バイナリのため、ユーザー側に Node.js / Bun 環境を要求しない

### Negative / Trade-offs

- Bun のクロスコンパイル成熟度はやや若い(2026-04 時点)。release worker の Bun バージョン pin が必要
- バイナリサイズが ~50MB 程度(Go の ~10MB と比較して大)。`gh extension install` ダウンロード帯域に微影響
- Bun 固有 API(`Bun.file` 等)に依存しすぎると将来 Node 移行時の切替コストが上がる — `node:fs` / `node:path` 中心で書く

## Alternatives considered

- **Go** — gh extension の事実上の標準(`gh-dash` / `gh-poi`)、precompiled テンプレートあり。GraphQL 型を別生成、TS で書いた lib 群と再利用不可。TS 中心の開発環境と分離するため不採用
- **Node + shim** — ユーザーに Node 要求、起動が遅い。gh extension の標準 UX(`gh extension install` 後に即実行)を損なう
- **Bash + `gh api`** — GraphQL クエリ・状態管理・i18n に不向き、テストも困難
- **Deno --compile** — Bun と概ね同等だが、既存の Deno 採用例がなく、新ランタイム導入コストに見合わない

## References

- [Bun build executables](https://bun.sh/docs/bundler/executables)
- [GitHub CLI extensions](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions#creating-a-precompiled-extension)
