# 0001. Bun `--compile` をバイナリビルドに採用する

- Status: Accepted
- Date: 2026-04-30
- Deciders: ozzy
- Tags: cli, build, language, distribution

## Context

[handbook ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md) で `ozzy-labs/gh-tasks` を gh extension として配布する方針が確定し、Open question #4「実装言語の選定」が残っていた。本 ADR で Open Q4 を確定する(根拠は [handbook/reviews/2026-04-30-gh-tasks-design.md](https://github.com/ozzy-labs/handbook/blob/main/reviews/2026-04-30-gh-tasks-design.md) 5 を引用)。

要件:

1. クロスプラットフォーム単一バイナリ配布(darwin/linux/windows × amd64/arm64)— `gh extension install` で precompiled が選好される
2. GitHub GraphQL クライアントの型を再利用したい(Octokit、`@octokit/graphql`)
3. テスト基盤を OzzyLabs 他リポ(`road` / `skills` / `knowledge-mcp-server`)と共通化(Vitest)
4. 開発時の起動コストを最小化(REPL / dev mode で `bun run` 即起動)
5. OzzyLabs ecosystem は TypeScript 中心。新言語導入のコンテキストスイッチを避けたい

候補比較は handbook design review 5 で実施済。本 ADR は決定と repo 内根拠の記録に留める。

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

- OzzyLabs の TS 中心ポリシーと整合(`road` / `skills` / `knowledge-mcp-server` / `presets` と共通)
- Octokit / GraphQL の型を CLI / scripts / tests で再利用可能
- Vitest テスト基盤を他リポと共有
- 開発時は `bun run src/cli.ts` で即起動、ビルド不要
- 単一バイナリのため、ユーザー側に Node.js / Bun 環境を要求しない

### Negative / Trade-offs

- Bun のクロスコンパイル成熟度はやや若い(2026-04 時点)。release worker の Bun バージョン pin が必要
- バイナリサイズが ~50MB 程度(Go の ~10MB と比較して大)。`gh extension install` ダウンロード帯域に微影響
- Bun 固有 API(`Bun.file` 等)に依存しすぎると将来 Node 移行時の切替コストが上がる — `node:fs` / `node:path` 中心で書く

## Alternatives considered

- **Go** — gh extension の事実上の標準(`gh-dash` / `gh-poi`)、precompiled テンプレートあり。GraphQL 型を別生成、TS で書いた lib 群と再利用不可。OzzyLabs の言語コンテキストと分離するため不採用
- **Node + shim** — ユーザーに Node 要求、起動が遅い。gh extension の標準 UX(`gh extension install` 後に即実行)を損なう
- **Bash + `gh api`** — GraphQL クエリ・状態管理・i18n に不向き、テストも困難
- **Deno --compile** — Bun と概ね同等だが、OzzyLabs に既存の Deno 採用例がなく、新ランタイム導入コストに見合わない

## References

- Related handbook ADR: [ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md)(Open Q4 を本 ADR で確定)
- Related design review: [reviews/2026-04-30-gh-tasks-design.md](https://github.com/ozzy-labs/handbook/blob/main/reviews/2026-04-30-gh-tasks-design.md) 5(候補比較表)
- External: [Bun build executables](https://bun.sh/docs/bundler/executables)、[GitHub CLI extensions](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions#creating-a-precompiled-extension)
