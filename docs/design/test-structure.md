# cmd test の構成

`cmd/` パッケージのテストは「cobra root を経由した flow / integration テスト」を主体に組み立てている。本ドキュメントは新規テストの追加先と命名規則を一意に決めるためのナビゲーション。

## ファイル分担

flow テストはコマンドごとに `cmd/<cmd>_flow_test.go` に分割している(`cmd_flow_test.go` という単一ファイルは存在しない)。

| File | 役割 |
| --- | --- |
| `cmd/testhelpers_test.go` | shared fixture (mock 型ラッパ、helper、payload builder)。テスト本体は持たない。 |
| `cmd/<cmd>_flow_test.go` | cobra root 経由の flow テスト。`add` / `list` / `today` / `done` / `standup` / `review` / `plan` / `triage` / `link` / `projects` / `build_skills` / `check_i18n` / `root` ごとに 1 ファイル。 |
| `cmd/<cmd>_internal_test.go` | `_internal_test.go` 接尾辞で「`package cmd` の private 型 / helper を白箱で叩く」ことを明示するテスト(`done` / `errors` / `link` / `review` / `standup` / `triage` で利用)。 |
| `cmd/errors_test.go` | sentinel error の包含関係 (`errors.Is`) と、各 cmd における arg-validation vs runtime の exit code 分類を pin するテスト。 |
| `cmd/<cmd>_test.go` | cmd の小さな helper / parser テスト。`package cmd_test` (external、`link_test.go` 等)と `package cmd` (internal、`plan_test.go` / `projects_test.go` / `build_skills_test.go` 等)の両方が使われる。internal 寄りに大きなホワイトボックスを書きたい時は `_internal_test.go` 接尾辞で意図を明示する。 |
| `cmd/cmd_transport_error_test.go` | GraphQL transport error が cmd 全体で `ErrSilentRuntime` に正規化されることを横断的に pin。 |
| `cmd/deps_resolve_test.go` | `Deps.Resolve` の locale/config 解決の単体テスト。 |

## mock 戦略

flow テスト全件が **同じ mock 表面** を共有する。GraphQL 共通フェイクは `internal/testfake/` に集約され、`cmd/testhelpers_test.go` がそれを薄くラップして cmd テスト固有の構文糖を提供する。REST フェイクは現状 cmd テスト内に閉じている。

- `internal/testfake.FakeGraphQL`: GraphQL 応答を `query <Name>(` の substring でマッチングする JSON-driven なキュー。`Responses` を登録順に消費する。
  - 接頭辞かぶり (例 `ListRepoIssues` vs `ListRepoIssuesWithLabels`) を避けるため、必ず開きカッコ `(` まで含めた substring を指定する。
- `internal/testfake.RecordingGraphQL`: 1 つの Resp / Err を毎回返しつつ、各 Do 呼び出しの `(query, vars)` を `Calls` に記録する。adapter / genqlient の境界を 1 回叩きで pin する用途。
- `cmd/testhelpers_test.go` 内 (cmd-only、private 型):
  - `captureGraphQL`: `*testfake.FakeGraphQL` をラップして outbound query / vars を覗く wrapper。フラグの伝播 (`--limit`, `--body` など) を assert したいときに使う。
  - `fakeREST`: 何もしない REST client。GraphQL のみ叩く cmd で `testDeps` の default として使われる。
  - `recordingREST`: REST 呼び出しを `<METHOD> <path-substring>` で照合し、応答を記録。`plan` の milestone 作成のように REST も叩く path で使う。`internal/testfake/` には移管されていない。
  - `testDeps(g, opts...)`: `cmd.Deps` を baseline で構築して opts で必要箇所を上書き。`Now` を `2026-05-04 12:00 UTC` に固定し、scope=org/user 化は `HasGitRemote=false` + `LoadConfig` で `Ref{Owner, Number}` を返すパターン。
  - `runCmd(t, d, args...)`: `RootWithDeps(d)` で cobra root を生成し、`SetArgs/SetOut/SetErr` を経由して `Execute` を呼び、`(stdout, stderr, err)` を返す。flag parse は cobra が単独で担う。

assertion は基本的に substring 比較 (`strings.Contains`)。i18n キーの値そのものを文字列で比較しているのは、English SSOT の安定キーをユーザー文言レベルで pin して PR のミスを catch する意図。

## 命名規則

`Test<Cmd>_<Scenario>` 形式に統一。例:

- `TestList_RepoEmpty`
- `TestList_LimitDefault`
- `TestStandup_OrgScope_DoneSplit_DraftExcludedUnderMine` (複合シナリオは `_` を続ける)
- `TestProjectsInit_DryRunHeader` (sub-cmd は CamelCase で繋げる: `Projects` + `Init`)

`<Cmd>` は cobra cmd 名を CamelCase 化したもの。`<Scenario>` は `RepoEmpty`, `OrgScope`, `MineFiltersAndAnnotates` のように **挙動の主語 + 形容** を短く書く。

## mock テストの限界（重要）

`internal/testfake.FakeGraphQL` および cmd 側の flow テストは、GraphQL リクエストの **クエリ名 substring** でマッチして固定 JSON を返す。**送信側 input の JSON 形状は一切検証していない**。これが意味するのは:

- input struct を `genqlient` が生成する型のままで送ると `null` が出るような fields があっても、flow テストは PASS する
- 例: `addProjectV2DraftIssue.assigneeIds` は genqlient で `[]string`（`omitempty` 無し）。nil で送ると JSON `"assigneeIds": null` だが、GitHub はこれを拒否する。flow テストは PASS、実機で初めて失敗
- 同型のバグ class（[`docs/design/genqlient-quirks.md`](./genqlient-quirks.md) の「パターン 1 / 2」）は **flow テストで検出不能**

→ **mutation を増やすときは、必ず Layer 1 の wire format pin test を `internal/github/queries/wire_<op>_test.go` に追加する** こと（[ADR-0009](../adr/0009-wire-format-and-e2e-strategy.md)）。flow テストが通ったことだけを根拠に「動いている」と判断しない。

新規 mutation 追加時のチェックリストは [`genqlient-quirks.md`](./genqlient-quirks.md) を参照。

## 新規テストを追加するとき

1. cobra root を経由する flow テスト → `cmd/<cmd>_flow_test.go` に追加(対象 cmd のファイルがまだ無い場合は新規作成)
2. exit code (`ErrSilent` / `ErrSilentArgs` / `ErrSilentRuntime`) の分類だけを pin する → `cmd/errors_test.go`
3. cmd 内部の helper / parser を unit-test したい
   - exported 経由でテスト可能: `cmd/<cmd>_test.go` (`package cmd_test`) に追加(例: `link_test.go`)
   - `package cmd` の private 型 / 関数を直接触る必要があり、ファイルが小さく済む: `cmd/<cmd>_test.go` (`package cmd`) でも可(例: `plan_test.go` / `projects_test.go`)
   - 同じ cmd 内で internal なホワイトボックスを意図的に集約したい: `cmd/<cmd>_internal_test.go` 接尾辞で意図を明示する
4. 共通 helper / payload builder が必要
   - cmd テスト固有: `cmd/testhelpers_test.go`(2 箇所以上で使われる場合のみ抽出)
   - cmd / internal 双方で使う共通 GraphQL フェイク: `internal/testfake/` に追加(現状は GraphQL のみ。REST が複数パッケージで必要になったら同様に移管する)

## カバレッジ表記の読み方（重要）

`go test ./...` の raw 出力でカバレッジ % を見るとき、特に `internal/github/queries` の **4.4%** という極端に低い数字に騙されないこと。実態は CI ゲートが見る通りで健全。

3 つの数字の関係:

| 数値 | 何を測っているか | どこで見える |
| --- | --- | --- |
| `4.4%` | パッケージ raw（`genqlient.go` の auto-gen 1600+ getter 込み） | `go test ./internal/github/queries/` の出力 |
| `~80%` | パッケージ実質（`exclude.paths` 適用後、hand-coded のみ） | `.testcoverage.yaml` ゲート判定値 |
| `86%` 前後 | プロジェクト total（`exclude.paths` 適用後） | `go-test-coverage --config .testcoverage.yaml` の "Total test coverage" |

`internal/github/queries` package には:

- `genqlient.go` — `Khan/genqlient` が `operations.graphql` から自動生成。型ごとに `GetX() X { return v.X }` 形式の getter を 1600 個以上含む。実コードからほとんど呼ばれず分母を大きく押し下げる
- `pagination.go` — hand-coded GraphQL pagination loop
- `input_constructors.go` — hand-coded null serialization workaround（[`genqlient-quirks.md`](./genqlient-quirks.md)）
- `rest_types.go` — REST 応答型宣言のみ、関数なし

`.testcoverage.yaml` の `exclude.paths` で `genqlient.go` と `generate.go` が除外される。CI ゲート (`go-test-coverage --config .testcoverage.yaml`) はこの除外後の数字で閾値判定するため、**raw 4.4% は CI を通過しない原因にはならない**（実質 ~80% で `^internal/github/queries$` の閾値を超えている）。

PR レビューでカバレッジを評価するときは:

1. **CI artifact `coverage-profile`** をダウンロードしてローカルで `go-test-coverage --config .testcoverage.yaml` を実行
2. 出力末尾の `Total test coverage: NN.N% (M/N)` を確認（こちらが除外後の honest な数字）
3. raw `go test` の per-package % は無視してよい

`genqlient.go` のような auto-gen ファイルを coverage 集計から除外する設計判断の経緯は [ADR-0008](../adr/0008-go-test-and-quality-chain.md) と `.testcoverage.yaml` のコメントを参照。

## 関連ドキュメント

- repo-internal ADR-0008: `go test -race -shuffle=on` を CI 必須化
- repo-internal ADR-0009: GraphQL wire format テストと E2E 戦略
- `docs/design/architecture.md`: cobra コマンド / Deps インジェクション全体像
- `docs/design/genqlient-quirks.md`: genqlient の null 直列化罠と対症療法
- `docs/design/e2e-test-plan.md`: E2E テスト戦略（Layer 1-7）
