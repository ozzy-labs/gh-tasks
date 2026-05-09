# gh-tasks E2E テスト計画

本ドキュメントは gh-tasks の **実 GitHub API を叩く End-to-End テスト** の設計と実行計画。`internal/testfake` を使う既存の flow / internal test (mock 完結、`docs/design/test-structure.md` 参照) とは独立した、ユーザー視点の網羅検証レイヤを定義する。

> **ステータス**: 計画段階（実装・実行はまだしない）。本ドキュメントの「実行前 TODO」がすべて埋まり次第、`scripts/` ではなく後述の `e2e/` パッケージとして実装に入る。

---

## 1. 目的とスコープ

### 目的

- 実 GitHub API（GraphQL + REST）に対するエンドツーエンドの動作確認
- 3-scope 抽象（repo / org / user）が現実の Issue / Project v2 / Milestone / Iteration / PR 上で破綻しないことを保証
- i18n SSOT（en）+ ja translation の両 locale で UX が壊れていないことの確認
- adapter 機構（install-skills / build-skills）の dogfood
- リリース直前 smoke の補完（既存の `gh extension install .` smoke は最低限のスモークで、機能網羅は別途必要）

### 含むもの（網羅対象）

```text
ユーザー向け 9 コマンド    add / list / today / done / standup / review / plan / triage / link
projects サブコマンド       projects init / projects init-templates
hidden コマンド             check-i18n / build-skills / install-skills
横串                        --scope / --project / --repo / --lang / --version / --help
```

### 含まないもの（明示的にスコープ外）

- 高負荷・障害注入（rate limit、429、5xx 強制）
- 4 agent それぞれの実 IDE 統合（出力ファイルの整合性チェックで代替）
- 非対応 locale（zh / ko 等）
- precompile 後の SLSA attestation 検証（リリース pipeline 側で担保）

---

## 2. テスト基盤

### 2.1 ターゲット資産

| Scope | 資産 | 場所 |
| --- | --- | --- |
| repo | Issues / Milestones / PRs | `ozzy-labs/gh-tasks` 本リポ |
| org | Project v2 #3 (`gh-tasks dev test`) | `ozzy-labs/3` Iteration: Sprint 1〜3 (14 day) |
| user | Project v2 #5 (`gh-tasks dev test`) | `ozzy-3/5` Iteration: Sprint 1〜3 (14 day) |

### 2.2 環境要件

- `gh auth status`: `ozzy-3` がアクティブで `repo`, `project`, `read:org`, `workflow` scope 付与済
- Go 1.25 以上（mise 経由）
- `gh` CLI v2.x
- E2E ランごとに `XDG_CONFIG_HOME=$(mktemp -d)` で隔離
- `git config --global` は触らない

### 2.3 隔離戦略

```bash
export XDG_CONFIG_HOME="$(mktemp -d)/xdg"
export GH_CONFIG_DIR="$HOME/.config/gh"   # 認証は流用
mkdir -p "$XDG_CONFIG_HOME/ozzylabs"
```

ホスト config (`~/.config/ozzylabs/gh-tasks.toml`) は **読み書きしない**。テストごとに必要な config を一時 `XDG_CONFIG_HOME` に書き込む。

---

## 3. 実装方式（推奨）

### 3.1 配置: `e2e/` パッケージ + Go build tag

```text
e2e/
├── doc.go                    # build tag 無し、package decl のみ常に存在
├── helpers_e2e_test.go       # t.Helper、setup/teardown shared fixtures
├── smoke_e2e_test.go         # smoke (--version、auth check)
├── add_e2e_test.go           # add コマンド E2E
├── list_e2e_test.go
├── today_e2e_test.go
├── done_e2e_test.go
├── plan_e2e_test.go
├── triage_e2e_test.go
├── standup_e2e_test.go
├── review_e2e_test.go
├── link_e2e_test.go
├── projects_e2e_test.go
├── install_skills_e2e_test.go
├── check_i18n_e2e_test.go
├── build_skills_e2e_test.go
├── lifecycle_e2e_test.go     # Flow A/B（コマンド連鎖シナリオ）
└── README.md
```

`doc.go` 以外はすべて `//go:build e2e` を付ける（共通ヘルパ含む）。ファイル名も `_e2e_test.go` 接尾辞に統一し、デフォルトの `go test ./...` から完全に除外する。

### 3.2 実行コマンド

```bash
go test -tags=e2e -v -count=1 -timeout=15m ./e2e/...
go test -tags=e2e -v -run TestE2E_Add ./e2e/...           # 単体
go test -tags=e2e -v -run TestE2E_Lifecycle ./e2e/...     # フロー
```

CI からは別 workflow (`.github/workflows/e2e.yaml`、手動 dispatch のみ、secret 必須) で trigger する案だが、**今回の計画ではローカル実行のみ**（CI 化は次フェーズ）。

### 3.3 テスト方式: 実バイナリ呼び出し

```go
// 各テストの先頭で 1 回だけビルド
binary := buildBinary(t)              // go build -o $TMPDIR/gh-tasks ./
out, stderr, err := runBinary(t, binary, "add", "[E2E] foo", "--scope", "user", "-p", "ozzy-3/5")
```

- `gh extension install .` 経由の `gh tasks <cmd>` ではなく、ビルドしたバイナリを **直接呼ぶ**（gh extension wrapper の影響を排除し、出力を pin しやすい）
- ただし最低 1 ケース（smoke 用）だけ `gh extension install .` + `gh tasks --version` を経由する確認テストを残す
- `XDG_CONFIG_HOME` / 環境変数はバイナリ呼び出し時に明示注入

### 3.4 cleanup の保証

- すべての作成系テストで `t.Cleanup(func(){ ... })` を登録
- Cleanup は **失敗しても他テストを巻き込まない**（`t.Logf` でログだけ）
- 物理削除はせず、Issue は `gh issue close`、Project item は `Status=Done`、Milestone は `due_on` を過去に倒すのみ
- すべての作成リソースに `[E2E]` prefix を必須化（後追い目視・ヒトの誤操作回避）

---

## 4. 共通ヘルパ設計

```go
// e2e/helpers_test.go (要約)

type Env struct {
    Bin     string         // built binary path
    XDG     string         // temp XDG_CONFIG_HOME
    Now     time.Time
}

func setupEnv(t *testing.T) *Env                       // build + tempdir + auth precheck
func writeConfig(t *testing.T, env *Env, toml string)  // XDG_CONFIG_HOME に gh-tasks.toml 配置
func runCmd(t *testing.T, env *Env, args ...string) (stdout, stderr string, exit int)
func runCmdEnv(t *testing.T, env *Env, extraEnv []string, args ...string) (...)

// Created resource トラッカー（cleanup 用）
type Tracker struct {
    Issues       []IssueRef
    DraftItems   []ProjectItemRef
    Milestones   []MilestoneRef
    PRs          []PRRef
}
func (tr *Tracker) Track(...)
func (tr *Tracker) CleanupAll(t *testing.T)            // close-only

// 認証チェック
func requireGHAuth(t *testing.T)                       // gh auth status を pre-flight check

// Project / Iteration 確認
func iterationFieldOf(t *testing.T, owner string, num int) (fieldID string, sprints []Sprint)
```

---

## 5. テストマトリクス

凡例: ✅=実施 ／ ⚠=エッジケースのみ ／ —=該当なし

| Command | repo | org | user | flag/edge ケース |
| --- | --- | --- | --- | --- |
| `add <title> [--body]` | ✅ Issue 作成 | ✅ draft item 作成 | ✅ draft item 作成 | 空 body / 5KB body / 非 ASCII title / `-p` override / `-r` override / config 由来 default |
| `list [--limit N]` | ✅ | ✅ | ✅ | `--limit 1` / `--limit 100` / 0 件 / closed 含むか |
| `today` | ✅ | ✅ | ✅ | 当日該当ゼロ / Iteration の current 切替境界 |
| `done <id>` | ✅ | ✅ | ✅ | 既に done / 未存在 ID / `--scope` mismatch / 他人作成 |
| `plan --period {daily,weekly,sprint}` | ✅ Milestone | ✅ Iteration | ✅ Iteration | preview(default)の差分を golden で pin / `--write` 経路は Flow A/B でカバー / `sprint` × `repo` の組合せ拒否 |
| `triage [--limit]` | ✅ | ✅ | ✅ | 全件トリアージ済 / `--limit` 上限 |
| `standup [--mine] [--since]` | ✅ | ✅ | ✅ | `--since` ISO-8601 / 未来日 / `--mine` フィルタ |
| `review --period {daily,weekly,sprint}` | ✅ closed Issue + merged PR | ✅ Project 完了 | ✅ Project 完了 | 期間境界 / 0 件 |
| `link <pr> <task>` | ✅ PR body に `Closes #N` | ✅ PR / Issue を同 Project に bind | ✅ 同左 | idempotent / 既に Closes 済 / cross-repo PR / 不正番号 / `Fixes` `Resolves` 検出 |
| `projects init [--template] --title` | — | ✅ org template | ✅ user template | `--dry-run` / 既存 title 衝突 / 任意 YAML 指定 |
| `projects init-templates` | static | static | static | en/ja で stdout 同一を確認 |
| `install-skills` | dogfood | dogfood | dogfood | auto-detect / `--agent` / `--namespace` / `--force` (`.bak`) / `--uninstall` (refcount) / `--check` non-zero / 再実行 idempotent |
| `build-skills` | dogfood | — | — | `dist/` 差分なし（CI dogfooding と同等） |
| `check-i18n` | dogfood | — | — | `--refs` で en/ja catalog 整合 / 非 ASCII 故意混入で fail |
| root `--version` `--help` `completion` | static | static | static | `gh-tasks --version` の値、`completion bash` などの shell 出力 |

---

## 6. ライフサイクル（連鎖）シナリオ

### 階層構造

ADR-0009 に従い、Flow を 3 階層に分けて運用する:

```text
Flow 0   smoke (read + write roundtrip)        ~3-5 min   release 前 + 任意
Flow 1   per-command isolation smoke           ~10 min    release 前
Flow A〜H lifecycle (連鎖)                      ~20 min    major / minor release 前
```

各層は **下位層が PASS していることを前提条件として skip 制御** する。`Flow A` を走らせる前に `Flow 1 の Add/Done/Link/Plan` が通っていなければ、Flow A はそのテストランで skip し、原因切り分けを Flow 1 に委ねる。

### Flow 0 — smoke（read + 最小 write roundtrip、~3-5 min）

`/e2e smoke` および `mise run e2e:smoke` で実行される最小経路。**全 mutation が動くこと** + **i18n / scope 自動検出 / 認証** を最短時間で確認する。

`-run TestE2E_Smoke` で前方一致するテスト関数すべてを smoke と見なす。

| 関数名 | 内容 | 副作用 |
| --- | --- | --- |
| `TestE2E_SmokeVersion` | `gh-tasks --version` を呼び、semver らしい文字列が返ることを assert | なし |
| `TestE2E_SmokeAuth` | `gh auth status` + `viewer.projectsV2(first:1)` の probe で認証 + project scope 付与済を確認 | なし |
| `TestE2E_SmokeReadOnly` | `gh tasks list -p ozzy-labs/3` / `-p ozzy-3/5` で 2 Project が読み取れること | 読み取りのみ |
| `TestE2E_SmokeI18nGraphQL` | GraphQL に到達した後の locale 化エラーを `--lang en` / `--lang ja` で diff 検証 | 読み取りのみ |
| `TestE2E_SmokeWriteRoundtrip_{Org,User}` | `add` で `[E2E] smoke ...` draft を作成 → 即 `done` で Status=Done | draft item 1 件 (Status=Done で残す) |

**狙い**: 全 mutation を最低 1 回 roundtrip させ、wire format バグ（[`genqlient-quirks.md`](./genqlient-quirks.md)）を実機経路で検出する。テストパイプライン本体（mise / skill / GHA）の導通も同時に確認。Layer 1 の wire format pin test と相補的（Layer 1 は CI 常時、Flow 0 は release 直前の保険）。

**repo scope の write smoke** は別扱い: 共有リポ `ozzy-labs/gh-tasks` に Issue を作るのは履歴汚染になるため、Flow 0 では割愛し Flow 1 (`TestE2E_AddSmoke_Repo`) で `--repo` flag による別リポ指定または明示的な `[E2E]` ラベル運用に委ねる。

### Flow 1 — per-command isolation smoke（~10 min）

各 cmd を **単独で** 検証する。Flow A-H の連鎖の前提条件として通っていることを保証する。

| 関数名 | 対象 | 検証内容 |
| --- | --- | --- |
| `TestE2E_AddSmoke_{Repo,Org,User}` | `add` | 最小 add → 即 cleanup |
| `TestE2E_DoneSmoke_{Org,User}` | `done` | 事前に作った draft の Status 更新（repo は Flow A に集約） |
| `TestE2E_LinkSmoke_{Repo,Org,User}` | `link` | ephemeral PR + Issue を作って link |
| `TestE2E_PlanSmoke_{Repo,Org,User}` | `plan` | preview(default、`--write` を渡さず副作用なし)、`--period weekly` |
| `TestE2E_TriageSmoke_{Repo,Org,User}` | `triage` | read 中心 |
| `TestE2E_StandupSmoke_{Repo,Org,User}` | `standup` | read 中心 |
| `TestE2E_ReviewSmoke_{Repo,Org,User}` | `review` | read 中心 |
| `TestE2E_ListSmoke_{Repo,Org,User}` | `list` | read 中心、`--limit` バリエーション |
| `TestE2E_TodaySmoke_{Repo,Org,User}` | `today` | read 中心 |
| `TestE2E_ProjectsInitSmoke` | `projects init` | `--dry-run` のみ |

各テストは **t.Cleanup で全て巻き戻す**（Issue close、draft Status=Done、ephemeral PR close + branch delete、Milestone 過去 due_on）。同 Project を触るテストは serialize（`t.Parallel()` を付けない）。

連鎖がないので、`add` が壊れても `done` / `link` の単独 smoke が通れば「`add` だけ壊れている」と即判定可能。Flow A-H と違って **failure cascade を起こさない** のがポイント。

### Flow A — repo scope ライフサイクル

1. `add "[E2E] flow A issue"` → Issue # 取得
2. `list` で表示確認
3. `triage` 一覧に登場
4. `today` に登場
5. `plan --period weekly`(preview) → Milestone 提案を golden 比較
6. `plan --period weekly --write` → Milestone 作成 + Issue bind
7. テスト用 PR を `gh pr create --draft` で作成（`feat/e2e-stub` ブランチ、空 commit）
8. `link <pr> <issue>` → PR body に `Closes #N` が追記される
9. `link` 再実行で **idempotent**（重複追記しない）を確認
10. `done <issue>` で close
11. `review --period daily` で集計
12. `standup` で活動表示
13. Cleanup: PR close + branch delete、Issue は close 済み（履歴保持）、Milestone は `due_on` を 2000-01-01 に倒して archive 同等扱い

### Flow B — org/user scope ライフサイクル（並走）

各 Project（org #3, user #5）に対して独立に：

1. `add -s <scope> "[E2E] flow B item"`
2. `list -s <scope>` 確認
3. `triage -s <scope>` 確認
4. `plan -s <scope> --period sprint`(preview) → Sprint 1 への提案を golden
5. `plan -s <scope> --period sprint --write` → Iteration field に bind されたか GraphQL で再 fetch して assert
6. ダミー PR と `link -s <scope> <pr> <draft-id>` で同 Project bind を確認
7. `done -s <scope> <id>` で Status=Done
8. `review -s <scope> --period sprint` で完了集計
9. `standup -s <scope> --mine --since <ISO>`
10. Cleanup: draft item は Status=Done、PR は close

### Flow C — scope 自動検出

| 状況 | 期待 scope |
| --- | --- |
| 本リポ内で実行 | `repo` |
| `/tmp/<empty>/` で実行 + config `default_scope = "user"` | `user` |
| `/tmp/<empty>/` で実行 + config 無し | fallback `user` |
| `--scope team`（不正） | `scope.ScopeError` `error.scope.invalid` で非 0 exit |
| config の `default_scope = "team"`（不正） | `error.config.invalidDefaultScope` |

### Flow D — i18n / locale

- 全コマンドを `--lang en` / `--lang ja` で実行（マトリクスは "コマンド × scope × locale"）
- 主要メッセージ（`plan.proposed`, `plan.empty`, `link.added`, `error.scope.invalid` 等）が catalog を経由しているか substring assert
- `LANG=ja_JP.UTF-8` / `LC_ALL=C` でも `--lang` 指定時は flag 優先になることを確認
- 並行: `check-i18n --refs` で en/ja catalog の参照漏れがゼロを保証

### Flow E — install-skills（dogfood）

`mktemp -d` で consumer リポ（fake）を作成し：

| Step | 期待 |
| --- | --- |
| auto-detect (`.claude/` のみ存在) | claude-code adapter のみ書き込み |
| `--agent codex-cli,gemini-cli` | AGENTS.md に marker block を書き込み（idempotent） |
| `--namespace gh-tasks` | `task-add` → `gh-tasks-add` rename |
| `--force` 既存ファイル衝突 | `.bak` 退避 + 上書き |
| 同コマンド再実行 | byte 差分なし（idempotent） |
| 別 adapter 追加で `AGENTS.md` 共有 | marker block の他コンテンツが byte for byte 保護 |
| `--check` 差分あり | exit code 非 0 |
| `--uninstall`（複数 adapter 同居） | reference count で AGENTS.md は最後の adapter 削除時にのみ block 除去 |

### Flow F — build-skills

```bash
go run . build-skills
git status -- dist/
```

- `dist/` に diff が出ないことを assert（既に CI で担保しているが E2E でも回帰検出）
- 一時ディレクトリで `--out <tmp>` 系の flag があれば併せて検証（無ければ skip）

### Flow G — projects init

- `projects init-templates` で出力 YAML を取得
- `projects init --template user --title "[E2E] init test" --dry-run` で計画ログ確認
- `--dry-run` 無しで実 Project を作成 → 直後に Project を archive（GraphQL `archiveProjectV2`）して終了
- 作成済 Project への重複 init で衝突エラー確認

### Flow H — root / completion

- `gh-tasks --version` の出力フォーマット（`v` prefix の有無）
- `gh-tasks completion bash` `gh-tasks completion zsh` `gh-tasks completion fish` `gh-tasks completion powershell` がエラーなく出力されること
- `gh-tasks --help` が全 subcommand を列挙
- `gh-tasks <unknown>` で usage error

---

## 7. エラー / 異常系（必須カバー）

| ケース | 期待 |
| --- | --- |
| `--scope team` | `scope.ScopeError`、exit 非 0、stderr に i18n メッセージ |
| `-p no-slash` | `project.ProjectError` |
| `-r bad@@name` | `repo.RepoError` |
| `--period yearly` | `period.PeriodError` |
| `--lang xx` | i18n.UnknownLocale 系のフォールバック挙動を確認 |
| `gh auth logout` 状態 | `github.AuthError`、exit 非 0 |
| 存在しない Project (`-p ozzy-labs/9999`) | NotFound エラー（i18n 経由） |
| 存在しない Issue ID | NotFound |
| `done` 他人の Issue | 403 を i18n 経由 |
| 不正 TOML | `config.ConfigError` |
| `default_scope` 文字列以外 | `config.ConfigError` |
| `org_project = "no-slash"` | `config.ConfigError` |

---

## 8. 性能・ページング

- Project に 30+ item を `add` で投入し `list --limit 30` のページング遷移を観察
- `list --limit 100` で 1 ページに収まるかを assert
- 投入は専用ヘルパで一括（最後に Status=Done で履歴保持）

---

## 9. 並行性

- `t.Parallel()` は **scope ごとに排他**: org/user は Project が共有資産のため、同 Project を触るテストは serialize
- repo scope はテストごとに `[E2E]-<uuid>` でユニーク化し、Issue は並列可
- Project ごとに `sync.Mutex` を含む shared registry を `helpers_test.go` に置く

---

## 10. 失敗時のデバッグ

- すべての `runCmd` 出力を `testdata/_output/<test-name>.log` に書き出し、失敗時のみ `t.Logf` で要約 dump
- GraphQL リクエスト / レスポンスのログは debug build (`-tags=e2e,e2edebug`) で raw 出力可能に

---

## 11. リリース運用との接続

### テストアーキテクチャ全体（ADR-0009）

本ドキュメントが扱う Flow 0 / Flow 1 / Flow A-H は、ADR-0009 が定義する 7 層アーキテクチャの **Layer 4-6** に対応する。残る Layer は本書の対象外だが、責任分担を明示するため一覧:

| Layer | 内容 | 場所 | このドキュメント |
| --- | --- | --- | --- |
| L1 | wire format pinning | `internal/github/queries/wire_*_test.go` | 対象外（ADR-0009 / `genqlient-quirks.md`） |
| L2 | schema-aware mock + replay | （未採用） | 対象外 |
| L3 | schema drift detection | `.github/workflows/schema-drift.yaml`（将来） | 対象外 |
| **L4** | **smoke (read + write roundtrip)** | `e2e/smoke_e2e_test.go` | **§6 Flow 0** |
| **L5** | **per-command isolation smoke** | `e2e/<cmd>_e2e_test.go` | **§6 Flow 1** |
| **L6** | **lifecycle (Flow A-H)** | `e2e/lifecycle_*_e2e_test.go` | **§6 Flow A-H** |
| L7 | 本番テレメトリ | （未採用） | 対象外 |

### 起動経路（Layer 4-6 の運用上の 3 入口）

E2E は 3 つの入口（mise / skill / GHA）から起動でき、すべて同じ go test を呼ぶ:

```text
┌─────────────────────────────────────────────────────────┐
│ Layer 3: GitHub Actions workflow_dispatch (.github/...)  │  cloud manual
│   gh workflow run e2e.yaml -f flow=smoke                 │  PAT secret 必須
├─────────────────────────────────────────────────────────┤
│ Layer 2: /e2e skill (.claude/skills/e2e/)                │  普段の入口
│   pre-flight → AskUserQuestion で範囲選択 → Layer 1 起動 │  失敗時の next action 提示
├─────────────────────────────────────────────────────────┤
│ Layer 1: mise tasks (.mise.toml [tasks.e2e*])           │  全層の SSOT
│   mise run e2e                                           │  go test -tags=e2e ./e2e/...
│   mise run e2e:smoke                                     │
│   mise run e2e:run -- TestE2E_FlowA                      │
└─────────────────────────────────────────────────────────┘
```

### リリース時の運用フロー（推奨）

| タイミング | アクション | 担当 |
| --- | --- | --- |
| 全リリース前 | `gh extension install .` の dev mode smoke | 既存ルール（feedback memory `feedback_release_dev_mode_smoke` を参照） |
| メジャー / minor リリース前 | `/e2e all` または `mise run e2e` | 手動 |
| patch リリース前 | `/e2e smoke`（任意） | 手動 |
| 通常 PR | 該当 flow のみ任意で実行 | 手動 |
| 障害調査 | `gh workflow run e2e.yaml -f flow=<pattern>` | cloud 経由 |

`/e2e` skill は AskUserQuestion で範囲を確認しつつ Layer 1 を起動する設計。普段は skill 経由、CI 経路は cloud で再現したい / オフラインで走らせたい場合のみ。

### GHA secret 要件

`.github/workflows/e2e.yaml` は repo secret `E2E_GH_TOKEN`（`repo` + `project` scope の PAT）を必須とする。GHA の default `GITHUB_TOKEN` は `project: write` を持たないため、Project に書き込む E2E では使えない。

### CI 自動化の方針

- 全 PR で自動実行は **しない**（rate limit と flake のリスク、共有 Project への副作用）
- `workflow_dispatch` のみ提供し、必要に応じて `gh workflow run e2e.yaml` で手動 trigger
- 将来的に nightly cron を追加する場合はメモリ更新と相談する

---

## 12. 実行前 TODO（このプランを実装する前に解決すべき項目）

- [ ] `gh extension install .` smoke を 1 ケースだけ E2E に組み込む方針の最終確認（v0.1.0 直前で衝突しないか）
- [ ] `e2e/` ディレクトリ新設に対する `golangci-lint` / `gci` の影響評価
- [ ] `dist/` 検証を E2E に含めるか（CI と二重化するため、含めない方針なら除外）
- [ ] PR 作成系テスト（Flow A-7、Flow B-6）でテスト用ブランチが残らない仕組み（`t.Cleanup` で必ず branch delete）
- [ ] Project archive 操作（`archiveProjectV2` mutation）が org / user 両方で権限十分か
- [ ] 既存テスト Project (`ozzy-labs/3`, `ozzy-3/5`) のリセット手順（履歴蓄積で重くなった時の対応）
- [ ] `[E2E]` prefix を持つアイテムが累積した場合の archival ポリシー（月次 sweep など）

---

## 13. 参考リンク（リポ内）

- mock test の構造: `docs/design/test-structure.md`
- adapter 機構: `docs/design/adapter-pipeline.md`
- リリース手順: `docs/design/release-process.md`
- アーキテクチャ: `docs/design/architecture.md`
- ADR-0008（テスト戦略）: `docs/adr/0008-*.md`
