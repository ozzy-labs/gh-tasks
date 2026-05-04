# Architecture

`gh-tasks` の全体俯瞰。モジュール境界、ディレクトリ構成、主要データフローを記述する。各意思決定の根拠は `docs/adr/` に分離。

## ハイレベル構成

`gh-tasks` は **3 つの柱** から成る:

```text
                    ┌─────────────────────────────┐
                    │   src/skills/{name}/        │ ← skill SSOT (ja)
                    │     SKILL.md / SKILL.en.md  │
                    └────────────┬────────────────┘
                                 │ build-skills.mjs (per-adapter transform)
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
│  packages/gh-tasks/src/cli.ts                              │ ← CLI 本体
│    ├─ commands/*.ts  (add/list/today/plan/triage/done/    │
│    │                   review/standup/link/projects)       │
│    ├─ lib/*.ts       (config/repo/scope/project/period/    │
│    │                   github/projectItem)                  │
│    ├─ lib/queries/   (GraphQL fragments + types)            │
│    └─ i18n/          (en SSOT + ja translation)             │
└────────────────────────────────────────────────────────────┘
                                 │ bun build --compile
                                 ▼
                    bin/gh-tasks-{os}-{arch}
                    (5 ターゲット: darwin/linux/windows × x64/arm64)
```

## ディレクトリ構成

```text
gh-tasks/
├── packages/
│   ├── gh-tasks/         # CLI 本体(TypeScript、Bun --compile 対象)
│   │   ├── src/
│   │   │   ├── cli.ts             # entry point + dispatch
│   │   │   ├── commands/          # 各サブコマンド実装
│   │   │   ├── lib/               # 共通ヘルパー
│   │   │   ├── lib/queries/       # GraphQL クエリ定義
│   │   │   └── i18n/              # ja.json / en.json + index.ts
│   │   ├── bin/                   # ローカルビルド出力(.gitignore)
│   │   └── package.json
│   └── templates/        # Projects v2 フィールド定義 YAML
│       └── projects-v2/{user,org}.yaml
├── src/
│   └── skills/{name}/    # skill SSOT(ja: SKILL.md、en mirror: SKILL.en.md)
├── scripts/
│   ├── build-cli.mjs              # 5 ターゲットの cross-compile
│   ├── build-skills.mjs           # adapter pipeline orchestrator
│   ├── check-no-hardcoded-i18n.mjs # 非 ASCII 文字列リテラル検知 lint
│   ├── adapters/                  # 4 adapter 実装
│   └── lib/                       # adapter 共通ヘルパー
├── docs/
│   ├── manual/{en,ja}/   # ユーザーマニュアル(en SSOT、ja mirror)
│   ├── adr/              # 意思決定記録(ja 単一)
│   └── design/           # 設計ドキュメント(本ディレクトリ、ja 単一)
├── dist/{adapter-id}/    # adapter 出力(.gitignore、build:skills で再生成)
├── .claude/skills/       # ローカル staged コピー(Claude Code dogfooding)
├── .agents/skills/       # ローカル staged コピー(Codex CLI dogfooding)
└── skills-sync/          # consumer 向け Renovate preset
```

## 主要モジュール境界

### `packages/gh-tasks/src/cli.ts`

エントリポイント。責務:

- argv 先頭の subcommand を dispatch する(`commands/{name}.ts` への委譲)
- `loadConfig()` を呼ぶ(失敗時は `ConfigError` を捕捉して localized stderr を出力)
- 中央 catch block で typed error(`AuthError` / `RepoError` / `ScopeError` / `ProjectError` / `PeriodError`)を localize して出力、exit code 2 を返す
- `--help` / `--version` を処理

設計判断:

- 各 command は副作用ベース(stdout / stderr / exit code を返す)で、同じ `argv: readonly string[]` シグネチャに揃える
- `deps` パラメータで `client` / `hasGitRemote` / `getRemoteUrl` / `stdout` / `stderr` / `config` を注入可能にし、テストは決定論的にセットアップできる

### `commands/{name}.ts`

各サブコマンドの実装。共通パターン:

1. `resolveLocale(argv, env, config)` で出力言語を決める
2. 必要に応じて `detectScope({ argv, hasGitRemote, config })` で scope 判定
3. scope に応じて分岐(repo は GitHub Issues + Milestones、org/user は Projects v2)
4. GraphQL は `createClient(resolveToken())` 経由 → `client.request<T>(query, vars)`
5. 結果を localized メッセージで stdout に出力、exit code 0

### `lib/*.ts`

純粋関数中心のヘルパー群。各モジュールは「1 つの解決責務」を持つ:

| モジュール | 責務 | 主な関数 / 型 |
| --- | --- | --- |
| `config.ts` | TOML config 読込 + 検証 | `loadConfig`、`ConfigError` |
| `repo.ts` | `<owner>/<name>` 解決 | `resolveRepo`、`RepoError` |
| `scope.ts` | `--scope` 自動判定 | `detectScope`、`ScopeError` |
| `project.ts` | `<owner>/<number>` 解決 | `resolveProjectRef`、`ProjectError` |
| `period.ts` | `daily`/`weekly`/`sprint` の境界計算(IANA tz 対応) | `rangeOf`、`PeriodError` |
| `github.ts` | Octokit client 抽象、token 解決、エラー型 | `createClient`、`AuthError` |
| `projectItem.ts` | Project v2 item 解決(node id 検索等) | `resolveProjectNodeId` |
| `queries/` | GraphQL クエリ + レスポンス型 | `GET_ORG_PROJECT_V2` 等 |

### `i18n/`

CLI 出力 / エラーメッセージの key ベース translation。

- `en.json` が **SSOT**(repo-internal ADR-0005)、`ja.json` が translation
- `t(locale, key, args?)` で参照、locale 解決は `resolveLocale(argv, env, config)`
- フォールバック chain: 指定 locale → en → key 文字列(デバッグ用)
- locale 解決順: `--lang` フラグ → config `lang` → `LC_ALL` → `LANG` → fallback `en`(POSIX 標準で `LC_ALL` が `LANG` より優先)

## エラー設計

`lib/{config,repo,scope,project,period,github}.ts` のエラーは **i18nKey + i18nArgs パターン**:

```ts
class XxxError extends Error {
  readonly i18nKey: string;
  readonly i18nArgs: I18nArgs;
  constructor(i18nKey: string, args: I18nArgs = {}) {
    super(i18nKey);  // err.message は key そのもの
    this.name = 'XxxError';
    this.i18nKey = i18nKey;
    this.i18nArgs = args;
  }
}
```

- `super(i18nKey)` で `err.message` に key を入れる(localize は出力時に行う)
- catch block(`cli.ts` の中央 catch block / 一部 commands)で `t(locale, err.i18nKey, err.i18nArgs)` で localize して stderr 出力
- ハードコード ja 文字列は `scripts/check-no-hardcoded-i18n.mjs`(`pnpm run lint:i18n`、lefthook pre-commit、CI)で検知して reject

## 配布モデル

### CLI バイナリ

- `bun build --compile --target=bun-{os}-{arch}` で 5 ターゲット(darwin/linux × x64/arm64、windows × x64)を発行
- GitHub Releases に attach、`gh extension install ozzy-labs/gh-tasks` でユーザー側にダウンロード
- リポルートの `gh-tasks` shim が `uname -s` / `uname -m` で該当バイナリを exec(repo-internal ADR-0001)

### skill bundle

詳細は `docs/design/adapter-pipeline.md` を参照。要約:

1. `src/skills/{name}/SKILL.md`(ja SSOT)を `scripts/build-skills.mjs` が読み込み
2. 4 adapter(claude-code / codex-cli / gemini-cli / copilot)が `dist/{adapter-id}/` に各エージェント形式で出力
3. consumer リポは `skills-sync/{adapter}.json` Renovate preset を extend し、`sync-skills.sh` で `dist/` 内容を取り込む

## テスト構成

- `*.test.ts` は同階層に配置(`lib/scope.test.ts` 等)
- `vitest run` で実行(現状 19 ファイル / 203 テスト)
- 副作用注入(client / hasGitRemote / readFile)で決定論的に検証
- 統合テスト相当は実機で `gh tasks <subcommand> --lang={en,ja}` を手動確認

## 関連 ADR

- [ADR-0001](../adr/0001-use-bun-compile-for-binary.md): Bun --compile 採用
- [ADR-0002](../adr/0002-i18n-japanese-ssot.md): i18n は Japanese SSOT(Superseded by 0005)
- [ADR-0003](../adr/0003-graphql-via-octokit.md): GraphQL は Octokit 経由
- [ADR-0004](../adr/0004-skill-frontmatter-schema.md): SKILL.md frontmatter スキーマ
- [ADR-0005](../adr/0005-i18n-reader-based-ssot.md): i18n SSOT を読み手ベースに再設計、docs/ 構造再編
