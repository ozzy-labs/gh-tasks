---
description: gh-tasks の End-to-End テスト (実 GitHub API 叩き) を実行する。リリース前 / 大改変後の網羅検証用。
disable-model-invocation: true
allowed-tools: Bash, AskUserQuestion, Read
---

# e2e

gh-tasks の End-to-End テストを実 GitHub API に対して実行する skill。詳細仕様は `docs/design/e2e-test-plan.md` を参照。

> ⚠ 本 skill は実 GitHub API に書き込みを発生させる（ozzy-labs/3, ozzy-3/5 の Project に `[E2E]-` prefix 付き Issue / draft item を作る）。普段の開発ループでは使わず、リリース前 / 大改変後の検証で使う。

## 入力（引数）

- 引数なし: 対話で範囲を選択する
- `all`: 全フロー（~15-20 分）
- `smoke`: 最短経路（~2 分）
- 任意の Go test pattern (例: `TestE2E_FlowA`)

## 手順

### 1. 事前チェック (必須、いずれか失敗で中断)

`mise` / `git status` がいずれもリポルートを cwd 前提とするので、必ず先頭で repo root に移動してから順に実行する:

```bash
cd "$(git rev-parse --show-toplevel)"   # cwd をリポルートに統一
gh auth status                          # 認証済かを確認
gh api graphql -f query='query{viewer{projectsV2(first:1){totalCount}}}' >/dev/null
                                        # project scope 付与の真の検証
                                        # （scope 不足時は 403、E2E 中盤での
                                        # mutation 失敗を pre-flight で前倒し）
gh project view 3 --owner ozzy-labs --format json --jq .title
gh project view 5 --owner ozzy-3 --format json --jq .title
git status --porcelain
```

- `cd` 失敗（git 管理外で起動）→ skill を中断
- `gh auth status` 失敗 → ユーザーに `gh auth login --scopes repo,project,read:org,workflow` を促して中断
- `gh api graphql ... projectsV2` が 403 → project scope 不足、`gh auth refresh -s project` を案内して中断
- Project 未存在 → `docs/design/e2e-test-plan.md` §2 のセットアップ手順を提示して中断
- working tree dirty → `AskUserQuestion` で続行可否を確認（E2E はローカルファイルを書き換えないが、未 commit の変更があると「テスト後の挙動」と「自分の変更による挙動」の切り分けが困難になるため警告）

### 2. 範囲確認

引数が無い場合は `AskUserQuestion` で範囲を確認する:

| 選択肢 | 内容 |
| --- | --- |
| 全フロー (all) | `mise run e2e` を実行 (~20 min) |
| smoke (推奨) | `mise run e2e:smoke` を実行 (~2 min) |
| 特定 flow | パターンを別 AskUserQuestion で確認後 `mise run e2e:run -- <pattern>` |

引数が `all` / `smoke` の場合は確認なしで該当 task を実行する。

### 3. 実行

該当する mise task を Bash で実行:

```bash
mise run e2e          # all
mise run e2e:smoke    # smoke
mise run e2e:run -- TestE2E_FlowA  # 特定 flow
```

実行中の出力はそのままユーザーに見せる（`go test -v` の各行）。

### 4. 結果サマリ

実行終了後、以下を集計してユーザーに報告:

- PASS / FAIL / SKIP の件数
- 所要時間
- 失敗があれば失敗テスト名と stderr の最後 20 行

### 5. cleanup 検証

[E2E] prefix のリソースで close されていないものがないかチェック:

```bash
gh issue list -R ozzy-labs/gh-tasks -S "[E2E] in:title is:open" --limit 5
gh pr list   -R ozzy-labs/gh-tasks -S "[E2E] in:title is:open" --limit 5
```

漏れがあれば警告し、`AskUserQuestion` で「自動 close するか / 手動確認するか」を聞く。

### 6. 次のアクション提案

`AskUserQuestion` で次を提示する（自動実行はしない）:

**全 PASS の場合:**

- リリース PR を作成（ユーザーに `/ship` 起動を案内）
- 本番 release-please 待ちで終了
- 失敗の手動再現 → 終了

**FAIL がある場合:**

- 失敗を修正（ユーザーに `/implement` 起動を案内、失敗ログを context として共有）
- ログを詳細に確認 → 終了
- Flake (1 回だけ落ち) と判断 → `mise run e2e:run -- <落ちたテスト>` で再実行

## 失敗時のフォールバック

- 認証エラー: `gh auth login --scopes repo,project,read:org,workflow` を案内
- Project 未存在: 計画書 §2 のセットアップ手順（GraphQL `createProjectV2` + Iteration field 設定）を提示
- timeout: GitHub API rate limit を `gh api rate_limit` で確認、`mise run e2e:smoke` への範囲縮小を提案
- 累積した [E2E] item が 100 件を超える: `archive/sweep` の必要性を警告（手動 archive を案内）
- mise tool 未インストール: `mise install` を促す

## 参考

- 計画書: `docs/design/e2e-test-plan.md`
- mise tasks: `.mise.toml` の `[tasks.e2e*]` セクション
- GHA からの起動: `.github/workflows/e2e.yaml` (workflow_dispatch)
