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

# コマンド既定の取得上限を超えて全件取得
gh tasks list --paginate --json id
```

`--json=`(空値)は利用可能フィールドを stderr に表示して exit 1 する。1 つ以上のフィールド名をカンマ区切りで渡すと、stdout に JSON 配列が出る。`--jq <query>` は [gojq](https://github.com/itchyny/gojq) 互換フィルタを配列に適用する。値は 1 行ずつ出力され、object / array は 2 スペースインデント。

field 名の補完も動作する: `gh tasks list --json id,<TAB>` で残りカタログ名が prefix 付きで提示される。

## サポートコマンド

| コマンド | カタログ |
| --- | --- |
| [`list`](./cli.md#gh-tasks-list) / [`today`](./cli.md#gh-tasks-today--period-dailyweeklysprint) | item |
| [`triage`](./cli.md#gh-tasks-triage) | item |
| [`plan`](./cli.md#gh-tasks-plan--period-dailyweeklysprint)(preview / `--write` 両方) | item |
| [`standup`](./cli.md#gh-tasks-standup---mine---since-iso8601) / [`review`](./cli.md#gh-tasks-review--period-dailyweeklysprint) | activity(= item + `category`) |
| [`add`](./cli.md#gh-tasks-add-title) / [`done`](./cli.md#gh-tasks-done-id) | item |
| [`link`](./cli.md#gh-tasks-link-pr-task) | link(= item + `linkType` + `linkedTo`) |
| [`projects init`](./cli.md#gh-tasks-projects-init-yaml-path) / [`projects init-templates`](./cli.md#gh-tasks-projects-init-templates) | projectInit |

`--paginate` は read 系コマンド(`list` / `today` / `triage` / `standup` / `review`)で利用可能。他コマンドは単一レコードまたは固定小数のレコードを扱うため paginate に意味的な必要性がない。

## フィールドカタログ

### `item`(list / today / triage / plan / add / done)

下記の表は `gh tasks check-json-schema --update` で自動生成される(英語、catalog の Description が SSOT のため):

<!-- begin: jsonout-catalog item -->

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | GraphQL global ID of the Issue or Project item |
| `number` | int | Issue / Project item number (0 for draft items) |
| `state` | string | Issue / PR state (`OPEN` / `CLOSED` / `MERGED`); empty string for draft items where it does not apply |
| `title` | string | Title of the Issue or Project item |
| `type` | string | ISSUE \| PULL_REQUEST \| DRAFT_ISSUE |
| `updatedAt` | string | Last-update timestamp (RFC 3339) |
| `url` | string | Absolute URL on github.com (empty for draft items) |

<!-- end: jsonout-catalog item -->

### `activity`(standup / review)

`activity` は `item` に `category` discriminator を追加した拡張:

<!-- begin: jsonout-catalog activity -->

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | GraphQL global ID of the Issue or Project item |
| `number` | int | Issue / Project item number (0 for draft items) |
| `state` | string | Issue / PR state (`OPEN` / `CLOSED` / `MERGED`); empty string for draft items where it does not apply |
| `title` | string | Title of the Issue or Project item |
| `type` | string | ISSUE \| PULL_REQUEST \| DRAFT_ISSUE |
| `updatedAt` | string | Last-update timestamp (RFC 3339) |
| `url` | string | Absolute URL on github.com (empty for draft items) |
| `category` | string | Activity bucket the row belongs to (e.g. closed / merged / in-progress / done / completed) |

<!-- end: jsonout-catalog activity -->

#### `category` の値

| コマンド | scope | 値 |
| --- | --- | --- |
| standup | repo | `closed`、`merged`、`in-progress` |
| standup | org / user | `done`、`in-progress` |
| review | repo | `closedIssue`、`mergedPR` |
| review | org / user | `completedProjectItem` |

### `link`(link)

`link` は `item` に link 確立情報を追加した拡張:

<!-- begin: jsonout-catalog link -->

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | GraphQL global ID of the Issue or Project item |
| `number` | int | Issue / Project item number (0 for draft items) |
| `state` | string | Issue / PR state (`OPEN` / `CLOSED` / `MERGED`); empty string for draft items where it does not apply |
| `title` | string | Title of the Issue or Project item |
| `type` | string | ISSUE \| PULL_REQUEST \| DRAFT_ISSUE |
| `updatedAt` | string | Last-update timestamp (RFC 3339) |
| `url` | string | Absolute URL on github.com (empty for draft items) |
| `linkType` | string | How the link was established: `closesAdded` (PR body got `Closes #N`) or `projectBind` (PR + task bound to the same Project v2) |
| `linkedTo` | object \| null | Target task that the PR was linked to. Object `{id, number, type, url}` or null when the link was already in place |

<!-- end: jsonout-catalog link -->

### `projectInit`(projects init / init-templates)

<!-- begin: jsonout-catalog projectInit -->

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | GraphQL global ID of the created Project v2 (empty for --dry-run / init-templates) |
| `number` | int | Project v2 number (0 for --dry-run / init-templates) |
| `title` | string | Project title |
| `url` | string | Project URL on github.com (empty for --dry-run / init-templates) |
| `owner` | string | Owner login (`@me` resolved to the actual viewer login at runtime; empty for init-templates) |
| `template` | string | Template name (`user` / `org`) or empty when a custom yaml path was used |
| `fields` | array | Array of `{name, dataType, options?}` for the configured field set |

<!-- end: jsonout-catalog projectInit -->

`projects init --json` は single-element array、`projects init-templates --json` は 2-element array(user → org)を返す。`fields` 各要素の `options` は single-select 以外の field では `null`。

## 挙動と契約

### ストリーム分離

- **stdout** = データのみ。JSON 配列または `--jq` で絞り込んだ値
- **stderr** = 警告、localized エラー、フィールドカタログ(`--json=` 指定時)
- エラー時は **stdout は空** にする(`... | jq` で partial JSON が混ざらない)
- バリデーション / 実行時エラーは exit code 非 0

`plan --write --json` は明示的な例外: 人間向け mutation 進捗テキストが stdout に流れ、その**後ろ**に bound items の JSON 配列が並ぶ。terminal user は milestone 作成ログをそのまま見れる一方、script は末尾の `[` を locator にして JSON だけ取れる。

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
# OPEN state のアイテムの title だけを抽出
gh tasks today --json title,state --jq '.[] | select(.state=="OPEN") | .title'
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

### close した Issue の state を script で確認する

```bash
gh tasks done 42 --json state,url --jq '.[0]'
# {
#   "state": "CLOSED",
#   "url": "https://github.com/owner/repo/issues/42"
# }
```

### `--limit` を超えて全件 untriaged を取得する

```bash
gh tasks triage --paginate --json id,title --jq 'length'
```

### PR を bind した後にリンク先を読む

```bash
gh tasks link 12 42 --json linkType,linkedTo \
  --jq '.[0] | "\(.linkType) → #\(.linkedTo.number)"'
# closesAdded → #42
```

### `yq` で YAML 出力にする

```bash
# yq -P が JSON を読んで YAML にする
gh tasks list --json id,title | yq -P
```
