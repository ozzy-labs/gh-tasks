# 0010. `--json [fields]` / `--jq <query>` を全コマンドの構造化出力契約として採用

- Status: Accepted
- Date: 2026-05-09
- Deciders: ozzy
- Tags: cli, json, jq, output, contract, automation

## Context

`gh-tasks` は元々人間向けのテキスト出力のみを提供しており、shell スクリプト / agent (Claude Code、Codex CLI 等) / 他ツール (`jq`、`yq`) と組み合わせる際に、ユーザーは `grep` / `awk` で文字列を削る必要があった。これは:

- 出力文言の変更で script が壊れる(stable contract が無い)
- `--lang` の影響を受け、ロケール跨ぎの自動化が不安定
- agent が解釈する際に余計な LLM コストがかかる

という不都合を生む。一方、`gh` 本体は `--json [fields]` / `--jq <query>` という慣習を確立しており、`gh issue list --json id,title --jq '.[].id'` のような idiom は GitHub CLI ユーザーの間で定着している。本リポは `gh extension` として配布されるため、**作法を gh 本体と揃えることで学習コストを最小化**できる ROI が大きい。

加えて、Phase 1 〜 3 で 11 コマンド (`list` / `today` / `triage` / `plan` / `standup` / `review` / `add` / `done` / `link` / `projects init` / `init-templates`) に展開する規模なので、初期に契約を確定させ後戻りを避ける必要がある。1.0 で freeze する前提で 0.x のうちに固める。

## Decision

`gh` 本体と同じ作法を全面採用する:

- **`--json [fields]`(CSV)** + **`--jq <query>`(`itchyny/gojq` 内蔵)**
- 出力は **stdout 専用の flat JSON 配列**(single-record の mutation 結果も 1-element array)
- カタログ SSOT は `cmd/jsonpath.go`(`itemJSONFields` / `activityJSONFields` / `linkJSONFields` / `projectInitJSONFields`)
- 値は **ロケール非依存**(field 名は英語 camelCase、値は GitHub 実体値)
- **null 残し**(`omitempty` しない)、**空配列は `[]`**
- **stable contract**: 0.x で breaking 許容(`feat!:`)、1.0 で freeze。field 追加は non-breaking
- **drift gate**: pre-commit(lefthook) と CI(`go` job) で `gh tasks check-json-schema --check` を走らせ、catalog と user manual の field 表が乖離した時に exit 非 0
- 既定動作は **テキストのまま維持**(BREAKING しない、opt-in)

## Consequences

### Positive

- `gh issue list --json id,title --jq '.[].id'` の手癖が `gh tasks list ...` でそのまま使える(学習コスト最小化)
- jq / yq との連携で自動化ループが script 側で完結
- catalog 中心の SSOT + drift gate で docs と実装の乖離が CI で検出される
- agent / MCP / skill からの構造化アクセスが open(LLM の token 消費削減)
- mutation 系も同じ flat array で扱えるため、`gh tasks add --json | gh tasks done --json` のような pipeline が可能

### Negative / Trade-offs

- 全コマンドに `--json` / `--jq` の配線が必要(各 cmd ~30 行の boilerplate)
- 4 catalog(`item` / `activity` / `link` / `projectInit`)の管理コスト。catalog 拡張時は `--update` で再生成・コミットが必要
- `linkedTo` や `fields` のように object / object 配列を row 内に nested で持つ箇所があり、`jq` 式が一部複雑になる
- `itchyny/gojq` 依存追加によるバイナリサイズ増(~500KB、Pure Go なので CGO は不要)
- `plan --write --json` は人間向けテキスト進捗 + 末尾 JSON 配列の混在 stdout になる(documented exception。consumer は trailing `[` を locator にする)

### 実装範囲(参考)

3 phase / 17 PR で完了:

- Phase 1 (#367, 5 PR): read 系 6 + `add`、helper パッケージ、設計 doc
- Phase 2 (#376, 8 PR + #385 docs): mutation 系、`--paginate`、shell completion、operations.graphql backfill、Hidden `check-json-schema`
- Phase 3 (#386, 3 PR): marker-based `--update` / `--check` モード、pre-commit + CI gate、maintainer 手順 docs

## Alternatives considered

- **`-o json/yaml/jsonpath`**(kubectl / aws-cli 流) — フォーマット拡張は容易だが、`gh extension` という配布形態と作法が乖離する。yaml サポートで test matrix が倍増し、`| yq -P` で代替できることを踏まえて不採用。
- **Object 構造をデフォルトにする**(mutation 結果を `{milestone: {...}, linked: [...]}` 等で返す) — 1 record の意味が直接的だが、`jq` pipeline で flat array より扱いが難しく、`gh` 本体との作法乖離も大きい。本リポでは linked task など nested 情報を row 内 object field(`linkedTo`、`fields`)として持つことで、外側は flat array で統一した。
- **JSON をデフォルト出力にし `--pretty` で人間向け**(`jq` / `yq` 流) — 人間が直接打つケースが圧倒的多数の task CLI には不適。既存 user 体験の breaking が大きい。
- **Go template (`--format`)** — 学習コストが重く、`--jq` で十分カバーできる。
- **TTY 自動切替**(pipe で JSON、TTY でテキスト) — pipe で挙動が変わる罠を避けるため不採用。明示的 opt-in (`--json`)を選択。
- **scope=repo / scope=org/user で別 catalog** — scope 間で意味的にズレるフィールド(`number=0` for draft、`url=""` for draft)はあるが、共通カタログ + null 残しでカバー可能。試行した結果、catalog 数を絞った方が consumer の認知負荷も低い。

## References

- Related repo ADR: [ADR-0009](./0009-wire-format-and-e2e-strategy.md)(GraphQL wire format / e2e 戦略 — `--json` 出力の信頼性は genqlient 経由の query / mutation 経路に依存)
- 設計 doc: [`docs/design/json-output.md`](../design/json-output.md)
- ユーザーマニュアル: [`docs/manual/en/reference/json-output.md`](../manual/en/reference/json-output.md) / [ja mirror](../manual/ja/reference/json-output.md)
- Phase 1 tracking: [#367](https://github.com/ozzy-labs/gh-tasks/issues/367)
- Phase 2 tracking: [#376](https://github.com/ozzy-labs/gh-tasks/issues/376)
- Phase 3 tracking: [#386](https://github.com/ozzy-labs/gh-tasks/issues/386)
- 採用した jq 実装: [`itchyny/gojq`](https://github.com/itchyny/gojq)(MIT、Pure Go、`gh` 本体も採用)
- 業界標準ガイド: [Command Line Interface Guidelines (clig.dev)](https://clig.dev/)
