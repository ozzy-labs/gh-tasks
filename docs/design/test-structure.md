# cmd test の構成

`cmd/` パッケージのテストは「cobra root を経由した flow / integration テスト」を主体に組み立てている。本ドキュメントは新規テストの追加先と命名規則を一意に決めるためのナビゲーション。

## ファイル分担

flow テストはコマンドごとに `cmd/<cmd>_flow_test.go` に分割している(`cmd_flow_test.go` という単一ファイルは存在しない)。

| File | 役割 |
| --- | --- |
| `cmd/testhelpers_test.go` | shared fixture (mock 型ラッパ、helper、payload builder)。テスト本体は持たない。 |
| `cmd/<cmd>_flow_test.go` | cobra root 経由の flow テスト。`add` / `list` / `today` / `done` / `standup` / `review` / `plan` / `triage` / `link` / `projects` / `build_skills` / `check_i18n` / `root` ごとに 1 ファイル。 |
| `cmd/<cmd>_internal_test.go` | `package cmd` 内に置く必要がある internal な型 / helper のホワイトボックステスト(`done` / `errors` / `link` / `review` / `standup` / `triage` で利用)。 |
| `cmd/errors_test.go` | sentinel error の包含関係 (`errors.Is`) と、各 cmd における arg-validation vs runtime の exit code 分類を pin するテスト。 |
| `cmd/<cmd>_test.go` | cobra root を介さない pure helper テスト(代表例: `link_test.go` の URL parser。新規追加もこの命名規則を使う)。 |
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

## 新規テストを追加するとき

1. cobra root を経由する flow テスト → `cmd/<cmd>_flow_test.go` に追加(対象 cmd のファイルがまだ無い場合は新規作成)
2. exit code (`ErrSilent` / `ErrSilentArgs` / `ErrSilentRuntime`) の分類だけを pin する → `cmd/errors_test.go`
3. cmd 内部の helper / parser を unit-test したい
   - exported 経由でテスト可能: そのファイル名と対になる `cmd/<cmd>_test.go` を追加
   - `package cmd` 内の private 型 / 関数を直接触る必要がある: `cmd/<cmd>_internal_test.go` に追加
4. 共通 helper / payload builder が必要
   - cmd テスト固有: `cmd/testhelpers_test.go`(2 箇所以上で使われる場合のみ抽出)
   - cmd / internal 双方で使う共通 GraphQL フェイク: `internal/testfake/` に追加(現状は GraphQL のみ。REST が複数パッケージで必要になったら同様に移管する)

## 関連ドキュメント

- repo-internal ADR-0008: `go test -race -shuffle=on` を CI 必須化
- `docs/design/architecture.md`: cobra コマンド / Deps インジェクション全体像
