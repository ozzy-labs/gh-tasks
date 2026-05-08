# 0009. GraphQL wire format テストと E2E 戦略

- Status: Accepted
- Date: 2026-05-09
- Deciders: ozzy
- Tags: testing, e2e, graphql, genqlient

## Context

ADR-0007 で `cli/go-gh` + `Khan/genqlient` を GraphQL クライアントとして採用した。genqlient は `operations.graphql` から型を自動生成し、`internal/testfake.FakeGraphQL` は flow テスト用に GraphQL 応答をモックする。これらの設計は read 系 query / 単純な mutation には十分機能してきたが、2026-05-09 の調査で **書き込み系 mutation の wire format バグが既存テスト群を通り抜けていた** 事実が判明した。

具体的には:

- `addProjectV2DraftIssue.assigneeIds` が genqlient 生成型 `[]string`（`omitempty` なし）→ nil で `null` 直列化 → GitHub が拒否（"Something went wrong"）
- `updateProjectV2ItemFieldValue.value` (`ProjectV2FieldValue`) の oneOf サブフィールドが全 `*string`（`omitempty` なし）→ 4 つが explicit null で送られ、GitHub の "exactly one" 制約違反

両者とも:

- `gh tasks add` / `gh tasks done` の org / user scope で **必ず再現** していた（環境依存の flake ではない）
- `cmd/*_flow_test.go` 群は **PASS していた**（`testfake.FakeGraphQL` がクエリ名 substring でマッチして固定 JSON を返す設計であり、送信側 input の JSON 形状を一切検証していないため）
- 既存の E2E smoke は **読み取り専用** に絞って設計したため検出経路が無かった

つまり、既存テストアーキテクチャには次の 3 構造的盲点が併存していた:

1. **wire format 盲点**: mock テストが送信 JSON を検証しない
2. **mutation 盲点 (smoke)**: smoke が mutation を発火しない
3. **schema drift 盲点**: GitHub の挙動変更（explicit null 拒否は仕様変化）を検出する仕組みが無い

これを mutation を増やすたびに対症療法で潰すのは不健全。テスト戦略として何を **構造で保証** し、何を **手動レビュー** に委ねるかを明確にする必要がある。

## Decision

テストを 7 層に再整理し、mutation バグの検出責任を **層ごとに分離** する。詳細仕様は [`docs/design/e2e-test-plan.md`](../design/e2e-test-plan.md) に置き、本 ADR ではアーキテクチャ判断のみを固定する。

### Tier 1（必須、CI 常時 + 手動 release 前）

| Layer | 責任 | 実装場所 |
| --- | --- | --- |
| **L1: wire format pinning** | 全 mutation の input JSON 形状を per-op snapshot test で pin | `internal/github/queries/wire_*_test.go` |
| **L3 (subset): schema drift watch** | GitHub schema 変更を nightly 検出 | `.github/workflows/schema-drift.yaml`（将来） |
| **L4: smoke (read + write roundtrip)** | 全 mutation を最低 1 回 roundtrip | `e2e/smoke_e2e_test.go`（`//go:build e2e`） |

### Tier 2（次フェーズ、リリース運用に組み込む）

| Layer | 責任 | 実装場所 |
| --- | --- | --- |
| **L5: per-command isolation smoke** | 各 cmd 単独の mutation roundtrip | `e2e/<cmd>_e2e_test.go` |
| **L6: lifecycle (Flow A-H)** | 多段ライフサイクル、release 直前に手動実行 | `e2e/lifecycle_*_e2e_test.go` |

### Tier 3（必要が出たら、現時点では実装しない）

- **L2: schema-aware mock + replay** — Layer 1 と検出範囲が重複するため不採用
- **L7: 本番テレメトリ** — OSS のプライバシー設計コストに見合わない

### genqlient null 直列化の対症療法ポリシー

新規 mutation を追加した際、生成された input 型に下記のいずれかが含まれていれば **必ず** `internal/github/queries/input_constructors.go` に対応を書き、Layer 1 wire test を追加する:

- `[]T \`json:"X"\``（`omitempty` 無し）→ `New<Op>Input(...)` コンストラクタで非 nil 空スライス初期化
- 同一 struct 内に 3+ の nullable pointer（`*T`）→ `MarshalJSON` で nil 除外

`scripts/audit-mutations.sh` が pre-commit / CI でこれを機械検出する。

### genqlient 移行のトリガ

null 直列化バグが **3 種類目** を超えた時点で、`shurcooL/githubv4` への移行 RFC を起こす。それまでは対症療法 + Layer 1 で抑制する。

## Consequences

### Positive

- mutation 系バグが unit-test 速度（数百 ms）で検出できる。実 GitHub に当たる前に CI で fail する
- testfake の wire 盲点が文書化されることで、`flow テスト通った = OK` の誤解が起きない
- 新規 mutation 追加時のチェックリスト（コンストラクタ + wire test + `audit-mutations.sh`）が機械強制される
- smoke が mutation を含むことで、E2E インフラ自体のヘルスチェックが意味を持つ
- Tier 3 を意図的に外すことで、個人 OSS で維持不能な複雑さを抱え込まずに済む

### Negative / Trade-offs

- 新規 mutation を追加するたびに wire test と（必要なら）コンストラクタを書く義務が増える — 工数 1 mutation あたり 10-15 分
- smoke の所要時間が ~3 秒から ~3-5 分に伸びる — release 前 smoke のスキップ案は出さない（[`feedback_release_dev_mode_smoke`](../../) 既存ルール）
- L2 (schema-aware mock) を不採用にしたため、flow テストは引き続き wire format に盲目 — 補強は L1 が担う前提
- L7 (telemetry) を不採用にしたため、本番ユーザーが踏んだ未知のバグはユーザー報告経由でしか検出されない — 個人 OSS では許容

## Alternatives considered

- **対症療法のみ（戦略なし）** — null 直列化バグが起きたら個別に直す。検出までのリードタイムが数日〜数ヶ月と長く、release 直前で発覚する事故が継続する。不採用
- **`shurcooL/githubv4` への即時移行** — typed null handling が genqlient より強い。ただし全 query / mutation の書き換えコストが過大、`go.mod` 経由の `tool` ディレクティブ統合も再構成必要。罠が 3 種類目を超えるまで保留
- **Layer 2 (schema-aware mock) を Tier 1 に含める** — wire format 検証が Layer 1 と重複するため、検出 ROI が低い。テスト基盤を二重に維持するコストに見合わない
- **mock を全廃して全テストを E2E 化** — テスト所要時間が爆発、rate limit に当たる、CI 常時実行不可。mock を残しつつ wire format を別 layer で補強する本案が現実解
- **GitHub schema を local に snapshot し、全 mutation を schema validate する pre-commit** — Layer 1 (wire format pin) と検出範囲が大きく重複し、schema 更新時のメンテ負担が大きい。drift 検知だけ nightly cron で薄く実装する形に縮退

## References

- Related repo ADR: [ADR-0006](./0006-go-and-cobra-migration.md), [ADR-0007](./0007-go-gh-graphql-client.md), [ADR-0008](./0008-go-test-and-quality-chain.md)
- Related design doc: [`docs/design/e2e-test-plan.md`](../design/e2e-test-plan.md), [`docs/design/genqlient-quirks.md`](../design/genqlient-quirks.md), [`docs/design/test-structure.md`](../design/test-structure.md)
- External: [genqlient](https://github.com/Khan/genqlient), [shurcooL/githubv4](https://github.com/shurcooL/githubv4), [GitHub Projects v2 API](https://docs.github.com/en/issues/planning-and-tracking-with-projects/automating-your-project/using-the-api-to-manage-projects)
