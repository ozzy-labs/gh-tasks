# gh-tasks E2E tests

実 GitHub API を叩く End-to-End テスト。`internal/testfake` を使う既存の flow / unit テスト（`cmd/`, `internal/`、詳細は [`docs/design/test-structure.md`](../docs/design/test-structure.md)）とは独立した、ユーザー視点の網羅検証レイヤ。

設計の SSOT は [`docs/design/e2e-test-plan.md`](../docs/design/e2e-test-plan.md)。

## 前提

- `gh auth status` が `repo` + `project` + `read:org` scope 付きでログイン済
- 下記 2 Project が ozzy-labs / ozzy-3 配下に存在し、Iteration field が設定済
  - `ozzy-labs/3` (`gh-tasks dev test`)
  - `ozzy-3/5` (`gh-tasks dev test`)
- mise 経由で Go 1.25+ がインストール済

## 実行方法

| ケース | コマンド |
| --- | --- |
| 全フロー (~20min) | `mise run e2e` |
| smoke のみ (~2min) | `mise run e2e:smoke` |
| 単一テスト | `mise run e2e:run -- TestE2E_FlowA` |
| Skill 経由 | `/e2e` (Claude Code) |
| GitHub Actions | `gh workflow run e2e.yaml -f flow=smoke` |

## 慣習

- すべての test ファイルは `//go:build e2e` を付ける（デフォルトの `go test ./...` から除外）
- ファイル名は `<topic>_e2e_test.go`、テスト関数は `TestE2E_<Scenario>` 形式
- 作成リソースには必ず `[E2E]` prefix を付け、`t.Cleanup` で close する（物理削除しない）
- Project は共有資産のため、同 Project を触るテストは `t.Parallel()` しない
- 失敗時の出力は `e2e/testdata/_output/<test-name>.log` に保存（GHA artifact 化のため）

## ファイル構成

```text
e2e/
├── doc.go                     # package doc (build tag なし、常に存在)
├── README.md                  # このファイル
├── helpers_e2e_test.go        # 共通ヘルパ (Env / Tracker / runCmd) — TODO
├── smoke_e2e_test.go          # smoke (--version, auth check) — TODO
├── add_e2e_test.go            # add コマンド E2E — TODO
├── list_e2e_test.go           # ... — TODO
└── ...
```

すべての test ファイルは `_e2e_test.go` 接尾辞 + `//go:build e2e` build tag を持つ。共通ヘルパも例外なく `helpers_e2e_test.go` とすることで、デフォルトの `go test ./...` から完全に除外される（ヘルパが `_test.go` だけだと unit test と一緒に compile されて意図しない依存が混入する可能性があるため）。

> 現状: テスト本体は未実装。インフラ（mise tasks / .claude/skills/e2e/SKILL.md / .github/workflows/e2e.yaml）のみ先行配置済。実装は `docs/design/e2e-test-plan.md` §12 の「実行前 TODO」を解決してから。

## トラブルシューティング

| 症状 | 対処 |
| --- | --- |
| `gh auth status` が project scope 不足 | `gh auth refresh -s project` |
| Project が見つからない | 計画書 §2 に従い 2 Project を再作成 |
| timeout で落ちる | `gh api rate_limit` で remaining 確認、smoke に切り替える |
| `[E2E]` Issue が大量に滞留 | 月次で手動 archive、または `gh issue list -S "[E2E] in:title is:closed" --limit 100` で確認 |
