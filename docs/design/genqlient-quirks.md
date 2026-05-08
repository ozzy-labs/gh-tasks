# genqlient quirks と対症療法

`Khan/genqlient` (v0.x) で生成された input 型のうち、JSON 直列化時に **明示的な `null`** を出力するケースが複数あり、GitHub Projects v2 mutation でそれらが拒否される。本ドキュメントは検出済の罠と、回避策の置き場所、新規 mutation 追加時のチェックリストをまとめる。決定の背景は [ADR-0009](../adr/0009-wire-format-and-e2e-strategy.md)。

## 検出済みパターン

### パターン 1: nullable list (`[T!]`) → `[]T` に omitempty 無し

GraphQL schema 上 `[T!]`（nullable list of non-null）として宣言されたフィールドは、genqlient で `[]string`（またはそれ相当）として生成され、struct tag は `` `json:"fieldName"` `` のみ（`omitempty` なし）。Go の zero value (nil slice) は JSON で `null` として直列化される。

**該当 input 型 / フィールド**（gh-tasks が呼ぶ範囲）:

| Mutation | Input 型 | 該当フィールド |
| --- | --- | --- |
| `addProjectV2DraftIssue` | `AddProjectV2DraftIssueInput` | `assigneeIds` |
| `createIssue` | `CreateIssueInput` | `assigneeIds`, `labelIds`, `projectIds`, `projectV2Ids` |
| `updateIssue` | `UpdateIssueInput` | `assigneeIds`, `labelIds`, `projectIds` |
| `updatePullRequest` | `UpdatePullRequestInput` | `assigneeIds`, `labelIds`, `projectIds` |

**GitHub の挙動**: 上記フィールドに `null` が来ると mutation 全体が拒否され、generic な 500 系エラー `"Something went wrong while executing your query"` が返る（参照番号付き）。診断には `gh api graphql` で同一 input から個別フィールドを抜く / 空配列に置き換える二分探索が必要。

### パターン 2: oneOf 入力 (`ProjectV2FieldValue`) のサブフィールド全 nullable

`ProjectV2FieldValue` は GraphQL spec 的には oneOf 風の input で、`text` / `number` / `date` / `singleSelectOptionId` / `iterationId` のうち **正確に 1 つ** が指定されることを期待する。genqlient はこれを 5 つの `*T` フィールドを持つ Go struct に展開し、各 tag は `\`json:"name"\`` のみ。1 つだけ set しても残り 4 つは explicit `null` で送信される。

**GitHub の挙動**: `"ProjectV2FieldValue must include exactly one of the following arguments: text, number, date, singleSelectOptionId, iterationId."` が返る（present-but-null を「指定された」と解釈する）。

## 対症療法

すべて `internal/github/queries/input_constructors.go` に集約してある。新規 mutation がパターン 1 / 2 を踏むときは同ファイルに追記する。

### コンストラクタ（パターン 1 用）

```go
func NewAddProjectV2DraftIssueInput(projectID, title string) *AddProjectV2DraftIssueInput {
    return &AddProjectV2DraftIssueInput{
        ProjectId:   projectID,
        Title:       title,
        AssigneeIds: []string{},   // nil → null を回避
    }
}
```

呼び出し側は `&queries.XxxInput{...}` 直接ではなく、必ず `queries.NewXxxInput(...)` を使う。

### MarshalJSON（パターン 2 用）

```go
func (v *ProjectV2FieldValue) MarshalJSON() ([]byte, error) {
    out := map[string]any{}
    if v.Date != nil { out["date"] = *v.Date }
    // ... 全 5 フィールドを nil チェック
    return json.Marshal(out)
}
```

oneOf 系 input が他に増えたら、同型の MarshalJSON を該当 type に追加する。

## 新規 mutation 追加時のチェックリスト

1. `operations.graphql` に mutation を追記し `go generate ./...` で `genqlient.go` を再生成
2. `scripts/audit-mutations.sh` を走らせて新規 input 型を確認
3. パターン 1 / 2 に該当する fields があれば `input_constructors.go` にコンストラクタ / MarshalJSON を追記
4. `internal/github/queries/wire_<op>_test.go` を新規作成（必要なら）し、各 input variant を snapshot test で pin
5. cmd 側の use site から `&queries.XxxInput{...}` を `queries.NewXxxInput(...)` に置き換え
6. `mise run e2e:smoke` で WriteRoundtrip が新規 mutation も含めて通ることを確認

これは pre-commit hook + audit script で機械強制する（[ADR-0009](../adr/0009-wire-format-and-e2e-strategy.md)）。

## 上流への移行トリガ

null 直列化バグが **3 種類目** を超えた時点で `shurcooL/githubv4` への移行 RFC を起こす。それまでは本ドキュメントの対症療法で抑制する。

理由: 対症療法は per-mutation のコストが固定（10-15 分 / 件）で漸増するが、ライブラリ移行は 1 回限りの大きな投資。罠の累積数を見て「対症療法の累積コスト > 移行コスト」になったタイミングで切り替える。

## 参考

- 検出履歴: [`memory/feedback_genqlient_nullable_list_pitfall.md`](.) (private memory)
- ADR: [ADR-0009](../adr/0009-wire-format-and-e2e-strategy.md)
- Related: [ADR-0007 GraphQL client 選定](../adr/0007-go-gh-graphql-client.md)
- 上流: [genqlient issue tracker](https://github.com/Khan/genqlient/issues)
