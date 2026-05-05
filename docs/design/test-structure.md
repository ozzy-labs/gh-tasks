# cmd test の構成

`cmd/` パッケージのテストは「cobra root を経由した flow / integration テスト」を主体に組み立てている。本ドキュメントは新規テストの追加先と命名規則を一意に決めるためのナビゲーション。

## ファイル分担

| File | 役割 |
| --- | --- |
| `cmd/testhelpers_test.go` | shared fixture (mock 型、helper、payload builder)。テスト本体は持たない。 |
| `cmd/cmd_flow_test.go` | cobra root 経由の flow テスト群。各 cmd (list / today / standup / review / add / done / link / plan / triage / projects) を section コメントで束ねる。 |
| `cmd/errors_test.go` | sentinel error の包含関係 (`errors.Is`) と、各 cmd における arg-validation vs runtime の exit code 分類を pin するテスト。 |
| `cmd/errors_internal_test.go` | `package cmd` 内に置く必要がある internal な error 型のホワイトボックステスト。 |
| `cmd/build_skills_test.go` | `gh tasks build-skills` の adapter pipeline (skill SSOT → dist) flow テスト。 |
| `cmd/link_test.go` | `gh tasks link` の細部 (URL parser など) を扱う pure helper テスト。flow は `cmd_flow_test.go` 側。 |

> 過去の `cmd_test.go` (ozzy-labs/gh-tasks#255 以前の 5 件) は `cmd_flow_test.go` に merge 済。同一 mock 戦略・同一実行経路で実質的な差が無く、新規テストを書く際の「どっちに置けば良いか」という曖昧さを解消するため統合した。

## mock 戦略

flow テスト全件が **同じ mock 表面** を共有する。`testhelpers_test.go` で定義:

- `fakeGraphQL`: GraphQL 応答を `query <Name>(` の substring でマッチングする JSON-driven なキュー。`responses` を登録順に消費する。
  - 接頭辞かぶり (例 `ListRepoIssues` vs `ListRepoIssuesWithLabels`) を避けるため、必ず開きカッコ `(` まで含めた substring を指定する。
- `captureGraphQL`: `fakeGraphQL` をラップして outbound query / vars を覗く wrapper。フラグの伝播 (`--limit`, `--body` など) を assert したいときに使う。
- `fakeREST`: 何もしない REST client。GraphQL のみ叩く cmd で `testDeps` の default として使われる。
- `recordingREST`: REST 呼び出しを `<METHOD> <path-substring>` で照合し、応答を記録。`plan` の milestone 作成のように REST も叩く path で使う。
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

1. cobra root を経由する flow テスト → `cmd_flow_test.go` の対応する `// ===== <Cmd>` section に追加
2. exit code (ErrSilent / ErrSilentArgs / ErrSilentRuntime) の分類だけを pin する → `errors_test.go`
3. cmd 内部の helper / parser を unit-test したい → そのファイル名と対になる `*_test.go` (例 `link.go` の helper → `link_test.go`)
4. 共通 helper / payload builder が必要 → `testhelpers_test.go` に追加 (重複が 2 箇所以上ある場合のみ抽出する)

## 関連ドキュメント

- repo-internal ADR-0008: `go test -race -shuffle=on` を CI 必須化
- `docs/design/architecture.md`: cobra コマンド / Deps インジェクション全体像
