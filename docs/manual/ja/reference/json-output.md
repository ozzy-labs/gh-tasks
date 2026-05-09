# JSON 出力

`gh tasks` は `gh` 本体の `--json [fields]` / `--jq <query>` インタフェースに合わせており、シェルの作法をそのまま流用できる。

## 概要

```bash
# 利用可能フィールドを一覧表示(空値)
gh tasks list --json=

# 指定したフィールドの JSON 配列を出力
gh tasks list --json id,number,title,type

# 内蔵 jq でフィルタ(Pure Go の gojq、外部依存なし)
gh tasks list --json id --jq '.[].id'
```

`--json=`(空値)は利用可能フィールドを stderr に表示して exit 1 する。1 つ以上のフィールド名をカンマ区切りで渡すと、stdout に JSON 配列が出る。`--jq <query>` は [gojq](https://github.com/itchyny/gojq) 互換フィルタを配列に適用する。値は 1 行ずつ出力され、object / array は 2 スペースインデント。

## サポートコマンド(Phase 1)

| コマンド | カタログ |
| --- | --- |
| [`list`](./cli.md#gh-tasks-list) / [`today`](./cli.md#gh-tasks-today--period-dailyweeklysprint) | item |
| [`triage`](./cli.md#gh-tasks-triage) | item |
| [`plan`](./cli.md#gh-tasks-plan--period-dailyweeklysprint)(preview のみ) | item |
| [`standup`](./cli.md#gh-tasks-standup---mine---since-iso8601) / [`review`](./cli.md#gh-tasks-review--period-dailyweeklysprint) | activity(= item + `category`) |
| [`add`](./cli.md#gh-tasks-add-title) | item |

mutation 系経路(`done`、`link`、`plan --write`、`projects init` の mutation 部分)は `--json` 未対応。`--json` と `plan --write` の併用は localized error が出る(Phase 2 で解放予定)。

## フィールドカタログ

### `item`(list / today / triage / plan-preview / add)

| フィールド | 型 | 備考 |
| --- | --- | --- |
| `id` | string | Issue / PR / Project item の GraphQL global ID |
| `number` | int | 番号。draft item は `0` |
| `title` | string | タイトル |
| `type` | string | `"ISSUE"` \| `"PULL_REQUEST"` \| `"DRAFT_ISSUE"` |
| `updatedAt` | string \| null | 最終更新 (RFC 3339)。source response が持たない場合(例: `add` の mutation 戻り値)は null |
| `url` | string | github.com の絶対 URL。draft item は空文字列 |

### `activity`(standup / review)

`activity` は `item` に 1 フィールド追加した形:

| フィールド | 型 | 備考 |
| --- | --- | --- |
| `category` | string | アクティビティ分類。コマンドごとの値は下記 |

#### `category` の値

| コマンド | scope | 値 |
| --- | --- | --- |
| standup | repo | `closed`、`merged`、`in-progress` |
| standup | org / user | `done`、`in-progress` |
| review | repo | `closedIssue`、`mergedPR` |
| review | org / user | `completedProjectItem` |

## 挙動と契約

### ストリーム分離

- **stdout** = データのみ。JSON 配列または `--jq` で絞り込んだ値
- **stderr** = 警告、localized エラー、フィールドカタログ(`--json=` 指定時)
- エラー時は **stdout は空** にする(`... | jq` で partial JSON が混ざらない)
- バリデーション / 実行時エラーは exit code 非 0

### ロケール非依存

`--json` 出力は `--lang` の影響を受けない:

- フィールド名は英語(camelCase)固定
- 値は GitHub 上の実体値(例: Project の Status field がユーザー命名「進行中」なら、`--json` 出力もそのまま `"進行中"`)
- `--lang en|ja` はテキストモードの出力と stderr エラーメッセージのみに作用

### null と空配列

- 指定したフィールドは常に出力される。値がない場合は省略ではなく JSON `null`
- 配列フィールドは空でも `[]`(`null` にしない)

これにより `jq` 式 `.[].milestone.title // "none"` や `.labels[]` が状態に依らず安全に動く。

### 安定性

- `0.x` 系の間は破壊的変更(フィールド名変更 / 削除 / 値型変更)を許容。`feat!:` commit と CHANGELOG への明記を伴う
- **フィールド追加**は non-breaking で、minor release で随時行う
- `1.0.0` 以降は破壊的変更に major bump が必要

詳細は [`docs/design/json-output.md`](../../../design/json-output.md) を参照。

### この reference の保守

カタログ本体は `cmd/jsonpath.go`(`itemJSONFields` / `activityJSONFields` / `linkJSONFields` / `projectInitJSONFields`)が SSOT。Hidden コマンド `gh tasks check-json-schema` で全カタログを markdown table として出力できるので、本ページの field 表との diff 比較で同期確認できる(コピペ不要):

```bash
go run . check-json-schema
```

新規カタログを追加した場合は `cmd/check_json_schema.go` の `jsonSchemaCatalogs()` にも追加する。

## 利用例

### jq に流す

```bash
# 進行中のアイテムの title だけを抽出
gh tasks today --json title,type --jq '.[] | select(.type=="ISSUE") | .title'
```

### standup 出力を category で絞る

```bash
# 昨日 merge した PR のみ
gh tasks standup --json number,title,category \
  --jq '.[] | select(.category=="merged") | "\(.number): \(.title)"'
```

### 作成した Issue の ID を後続コマンドで使う

```bash
issue_id=$(gh tasks add "Bug: /api/foo が 404" --json id --jq '.[0].id')
echo "$issue_id"
# I_kwDOSQTNsM8AAAAB...
```

### `yq` で YAML 出力にする

```bash
# yq -P が JSON を読んで YAML にする
gh tasks list --json id,title | yq -P
```
