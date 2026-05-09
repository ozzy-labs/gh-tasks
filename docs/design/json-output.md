# JSON Output Design

`gh tasks` の構造化出力 (`--json` / `--jq`) の採用方針・契約・実装ガイドライン。元 issue: #367。

## Goal / Non-goals

### Goal

- `gh tasks` を「人間向け CLI」から「自動化 building block」に格上げする
- shell pipeline / 他ツール / agent (MCP 含む) からの構造化アクセスを可能にする
- `gh` 本体と作法を揃え、ユーザーが既存の手癖 (`gh issue list --json id,title --jq '.[].id'`) をそのまま使えるようにする

### Non-goals

- yaml / xml 等の追加フォーマットサポート(`| yq -P` 等の標準ツールで変換すれば十分)
- Go template による出力カスタマイズ(`--jq` で代替できる、学習コストが重い)
- TTY 検出による自動切替(pipe で挙動が変わる罠を避ける)
- mutation 系コマンド全体への即時適用(Phase 1 は read 系 6 + `add` のみ。残りは別 issue で Phase 2)

## ハイレベル方針

```text
                ┌───────────────────────────────────┐
                │ gh tasks <cmd> ...                │
                └────────────────┬──────────────────┘
                                 │
                ┌────────────────┴────────────────┐
                │                                 │
        --json なし                       --json [fields]
        (default、人間向け)               (gh 流の opt-in)
                │                                 │
                ▼                                 ▼
         localized text                    JSON (stdout)
         (i18n.T 経由、stderr に           + diagnostics は stderr
          warning / progress)              + (--jq <query> で filter)
                                                  │
                                                  ▼
                                        | jq / | yq -P / etc.
```

`gh extension` として `gh` 本体と完全に作法を揃える。具体的には:

- `--json [fields]`(コンマ区切り、フィールド指定で payload を最小化)
- `--jq <query>`(`itchyny/gojq` 内蔵、pipe 不要で軽量フィルタ)
- `--json` 引数なし → 利用可能フィールド一覧を stderr に表示し exit 1(gh と同じ discoverability)

Pure Go の `gojq` は gh 本体も採用しており、Pure Go のため CGO 不要、license も MIT で extension の制約に合致する。

## 対象コマンドと phase 分け

| コマンド | Phase 1 (#367) | Phase 2 (#376) | 理由 |
| --- | :---: | :---: | --- |
| `list` / `today` / `standup` / `review` | ✅ | — | read 系、JSON 出力の本命 |
| `plan`(preview) / `triage` | ✅ | — | preview default のため candidates 構造化が特に有用 |
| `add` | ✅ | — | 返却 ID を script で後続 mutation に使える |
| `done` / `link` / `plan --write` | — | ✅ | mutation ack を script で確認可能に。Phase 1 で carve-out していた `plan --write` も解除 |
| `projects init` / `init-templates` | — | ✅ | created project + fields を構造化(`projectInitJSONFields` 新設) |

両 phase 完了で、`gh tasks list --json` → script → `gh tasks add --json` → `gh tasks done --json` → `gh tasks link --json` の完結したループが成立する。Phase 2 では加えて `--paginate` (read 系)・shell completion・catalog の `state` field 拡張・`title` / `updatedAt` の GraphQL backfill も入れた。

## 共通契約

### ストリーム分離

| ストリーム | テキストモード | JSON モード |
| --- | --- | --- |
| stdout | localized 結果メッセージ + データ | **JSON のみ**(色なし、warning なし) |
| stderr | warning / progress / error(localized) | warning / error(localized、`--lang` 尊重) |
| exit code | 成功 0、失敗 非 0 | テキストと同じ |

`--json` 指定時でも stderr の warning / error は localized message のままで構わない。**stdout には JSON 以外のいかなるバイトも書かない**(jq / yq pipeline 互換のため)。

### エラー時

- exit code 非 0
- stderr に既存の localized message (`error.*` キー)
- **stdout は空**(`{"error": "..."}` を出さない)

理由: `gh tasks list --json | jq` のような pipeline で、エラー時に jq が空 input を gracefully に handle する設計が標準的。stdout に partial JSON / error JSON を混ぜると、jq が parse error を起こすか、success / failure の区別が exit code とずれる。

### null vs omitempty

**null を残す**(`omitempty` を使わない)。

```json
// good
{"id": "I_a", "milestone": null, "labels": []}

// bad (script が .milestone.title でクラッシュ or 存在チェック必要)
{"id": "I_a", "labels": []}
```

理由: フィールド存在性を contract として固定する。script が `.milestone.title // "none"` のように nil-safe で書ける。`gh` / `kubectl` も同方針。

例外: 配列は **空配列 `[]`** で出す(null にしない)。これも script で `.labels[]` がエラーにならないため。

### locale 非依存

- フィールド名は **英語のみ**(`title`, `status`, `updatedAt` 等)
- 値は **GitHub 上の実体値そのまま**(Project の Status field がユーザー命名「進行中」ならそのまま `"status": "進行中"`)
- `--lang en|ja` は **テキスト出力のみに作用**、JSON は不変

理由: stable contract のため。フィールド名を locale で変えると script が壊れ、値だけを翻訳すると元 GitHub 上の表示と乖離する。

### `--limit` 尊重

JSON モードでも既存の `--limit` を尊重する。全件取得は `--limit <大きな値>` で対応する。将来 `--paginate` フラグ(gh と同じ全件取得モード)を追加する余地は残す。

### NO_COLOR / TTY

- テキストモードでは [NO_COLOR](https://no-color.org) 環境変数を尊重する(現在テキスト出力に色を使っていないが、将来導入する場合は必須)
- JSON モードでは ANSI escape を **絶対に**混ぜない。pipe / file / TTY のいずれでも、stdout は純粋な JSON

## フィールド命名規則 / 型

| ルール | 例 |
| --- | --- |
| **camelCase** | `updatedAt`、`projectNumber`、`isDraft` |
| **ID は string**(数値 ID も string で出す) | `"id": "I_kwDO..."`(GraphQL global ID) |
| **time は RFC 3339(UTC)** | `"updatedAt": "2026-05-09T05:17:54Z"` |
| **URL は absolute** | `"url": "https://github.com/owner/repo/issues/42"` |
| **enum は GitHub 実体値** | `"state": "OPEN"`(GraphQL の値そのまま、case 変換しない) |
| **配列は空でも `[]`** | `"labels": []` |
| **欠損は `null`** | `"milestone": null` |

scope 跨ぎで意味が同じフィールドは同名にする(`id`, `title`, `url`, `updatedAt` 等)。`repo` / `org` / `user` で構造が異なる部分(`milestone` vs `iteration`)は別フィールドとして共存させる(出力時に該当しない側を `null`)。

## 安定性ポリシー

| 操作 | 0.x 内 | 1.0 以降 |
| --- | --- | --- |
| フィールド追加 | OK(non-breaking) | OK(non-breaking) |
| フィールド削除 / rename | OK(BREAKING、`feat!:`) | **不可**(major bump 必要) |
| 値の型変更(string → object 等) | OK(BREAKING、`feat!:`) | **不可** |
| 既存フィールド意味の変更 | 非推奨 | **不可** |

1.0 以降は `gh` 本体と同じく、フィールド削除を伴う変更は避け、新フィールド追加で対応する(deprecated を docs で明示)。

`docs/manual/{en,ja}/reference/json-output.md` に各コマンドのスキーマを pin し、CHANGELOG にフィールド追加 / 削除を必ず記載する。

## CLI surface

### `--json [fields]` の挙動

```bash
# 空値 → 利用可能フィールド一覧を stderr に出して exit 1
$ gh tasks list --json=
Specify one or more comma-separated fields for `--json`:
  id          Issue / draft item の GraphQL global ID
  number      Issue / Project item の番号
  title       タイトル
  url         GitHub URL
  ...
exit status 1

# フィールド指定 → 指定された field のみ JSON 出力
$ gh tasks list --json id,number,title
[
  {"id": "I_a", "number": 42, "title": "Task A"},
  ...
]

# --jq と併用
$ gh tasks list --json id --jq '.[].id'
"I_a"
"I_b"
```

cobra/pflag の `String` フラグは bare `--json` (値なし) を `flag needs an argument` で拒否するため、ユーザーには **空値の明示** (`--json=` または `--json ""`) を案内する。help text の `Empty value (`--json=`) lists available fields.` がガイド。`StringSlice` + `NoOptDefVal` で bare `--json` を許容する案も検討したが、`NoOptDefVal` が value-bearing 形式 (`--json id,title`) を hijack する pflag 既知の罠があるため `String` + 手動 CSV split を採用。

検証は cmd 側 (`runList` 冒頭の `resolveJSONRequest`) で集約して実装する。`PreRunE` ではなく `RunE` 冒頭でやる理由は、`Resolve(c)` が返す `r.T` を localized error 表示で使うため(deps の解決順序を破らない)。

### `--jq <query>` の挙動

- `itchyny/gojq` を内部で実行
- input は `--json` で生成される配列 / オブジェクト
- output は jq が出す形式そのまま(string は引用符付き、数値はそのまま)
- syntax error は stderr に `jq: ...` で出して exit 1

```bash
$ gh tasks list --json id,title --jq '.[] | select(.title | startswith("[E2E]")) | .id'
"I_x"
"I_y"
```

### `--json` を持つコマンドの help 表示

cobra の help text では下記で固定する:

```text
      --json string    output as JSON. Empty value (`--json=`) lists available fields.
      --jq string      filter JSON output via jq expression
```

`--json` の help は long-form のみ短く保ち、利用可能フィールドの discoverable は引数なし実行に任せる(help text に全 field を貼ると drift する)。

## 設計トレードオフ(決定事項)

| 論点 | 採択 | 不採択案 | 理由 |
| --- | --- | --- | --- |
| フラグ命名 | `--json [fields]` | `-o json/yaml` | gh 本体と整合(本リポは gh extension) |
| yaml サポート | なし | `-o yaml` | `\| yq -P` で十分、test matrix を倍増させる価値なし |
| field 指定 | あり(`--json id,title`) | 固定スキーマ | gh 整合、Projects v2 GraphQL の cost 削減に効く |
| `--jq` 内蔵 | あり | pipe 前提 | 1 バイナリ完結、pipe 非対応環境(Windows PowerShell 等)で価値 |
| Go template | なし | `--format <go-template>` | `--jq` で十分、学習コスト高 |
| TTY 自動切替 | なし | `gh issue list` 流 | pipe で挙動が変わる罠 |
| デフォルト変更 | しない | JSON default | 直接打つユーザーが圧倒的多数 |

## 実装ガイドライン

### パッケージ構成

```text
internal/jsonout/
├── jsonout.go         # Renderer + 主要 API
├── fields.go          # FieldList / Field 型
└── jq.go              # gojq 実行 wrapper
```

### 公開 API(暫定)

```go
package jsonout

// Field はコマンドが公開する 1 フィールドのメタデータ。
type Field struct {
    Name        string  // JSON output のキー名(camelCase)
    Description string  // help text 表示用(英語、ascii)
    Default     bool    // --json 引数なし非対応時の default 出力に含めるか
}

// FieldList はコマンド単位のフィールドカタログ。
type FieldList []Field

// Render は items を JSON でシリアライズして w に書く。
//   fields: 公開する field 名のリスト(空なら ListFields() でエラー)
//   jq:     gojq query。空なら無加工で出力
//   items:  scope 跨ぎの DTO スライス(map[string]any 推奨、struct は反射経由)
func Render(w io.Writer, items any, fields []string, jq string, catalog FieldList) error

// ListFields は --json 引数なし時に呼ぶ。
// stderr に field 一覧を出力する責務は呼び出し元(cmd/*)に置く。
func ListFields(w io.Writer, catalog FieldList)
```

### cobra integration パターン

各コマンドは下記スニペットで `--json` / `--jq` を有効化する。`String`(comma-separated CSV)を採用するのは pflag `StringSlice` の `NoOptDefVal` がバグるため(上記)。検証は `RunE` 冒頭の `resolveJSONRequest` で集約して、`Resolve(c)` 後の locale を使った localized error 表示と整合させる:

```go
c.Flags().String("json", "", "output as JSON. Empty value (`--json=`) lists available fields.")
c.Flags().String("jq", "", "filter JSON output via jq expression")

// runList 冒頭:
jsonReq, jsonOn, err := resolveJSONRequest(c, r, listJSONFields)
if err != nil { return err }  // ErrSilentArgs (空値 → field list 表示済み)

// RunE で:
//   - --json="" or --json=  → ListFields(stderr) + ErrSilentArgs
//   - --json=id,title       → 通常 JSON 出力
//   - --jq= without --json  → error.json.jqWithoutJson + ErrSilentArgs
```

`ErrSilentArgs` は `cmd/errors.go` 既存の sentinel(`ErrSilent` の sub-class、exit 2)を流用。runtime 系エラー(API 失敗、unknown field)とは区別する。

### DTO 命名

scope 跨ぎ共通の DTO を `internal/jsonout/dto/` に置くか、`cmd/*.go` 内に local 定義するかは PR 1 で決める。**MVP は local 定義**で始め、Phase 2 で共通化を検討する(YAGNI)。

## テスト戦略

### 既存 flow_test.go との関係

`cmd/*_flow_test.go` は i18n.T() 文言を assertion している。JSON path 用には別系統の helper を `cmd/testhelpers_test.go` に追加する:

```go
// assertJSONFieldEquals は stdout を JSON として parse し、
// 指定インデックス + フィールドが期待値と一致することを assert する。
func assertJSONFieldEquals(t *testing.T, stdout string, idx int, field string, want any) { ... }

// assertJSONLength は stdout を JSON 配列として parse し、長さを assert する。
func assertJSONLength(t *testing.T, stdout string, want int) { ... }
```

### 各コマンドで追加するケース

PR 1 以降、各コマンドの `*_flow_test.go` に下記 3 系統を追加する:

1. **`--json` 引数なし**: stderr に field list が出て exit 1
2. **`--json id,title`**: stdout が `[]` を含む JSON 配列、各要素が指定 field のみ持つ
3. **`--json id --jq '.[].id'`**: jq 適用後の結果が想定通り

### CI gate

`docs/design/json-output.md` の安定性ポリシーを CI で部分的に守る:

- (Phase 2 で着手、Phase 3 で完成予定)スキーマ snapshot test: 各コマンドの `--json <all-fields>` 出力構造を golden file と比較し、不意の field 削除 / rename を検知。Phase 2 では Hidden コマンド `gh tasks check-json-schema` を提供して catalog を markdown table 化(手動 diff 用)。CI / pre-commit drift gate と markdown marker 経由の自動上書きは Phase 3 候補

## 関連

- 元 issue: [#367 feat(cli): unified --json / --jq output across read commands](https://github.com/ozzy-labs/gh-tasks/issues/367)
- 参考実装: [`gh issue list --json`](https://cli.github.com/manual/gh_issue_list)
- 業界標準ガイド: [Command Line Interface Guidelines (clig.dev)](https://clig.dev/)
- 内部依存(予定): [`itchyny/gojq`](https://github.com/itchyny/gojq) — Pure Go の jq 実装、gh 本体採用
