# CLI リファレンス

`gh tasks` の全コマンド / フラグ。

## 共通フラグ

- `--scope repo|org|user`: 対象スコープ。省略時は自動判定([scope-detection.md](./scope-detection.md))
- `--repo <owner>/<name>`: repo scope のターゲット。省略時は git remote `origin` から推定
- `--project <owner>/<number>`: org / user scope の Projects v2 ターゲット。省略時は config の `org_project` / `user_project`
- `--lang ja|en`: 出力言語。`--lang` フラグ → config `lang` → `LC_ALL` → `LANG` → `en` の順で解決([locale-detection.md](./locale-detection.md))
- `--help`, `-h`: ヘルプ表示
- `--version`, `-v`: バージョン表示

## コマンド

### `gh tasks add <title>` ✅

Issue(`repo`)/ Projects v2 draft item(`org` / `user`)を追加する。

```bash
gh tasks add '<title>' [--scope repo|org|user] [--repo <owner>/<name>] [--project <owner>/<number>] [--body '<detail>']
```

- `repo` scope: GitHub Issue を作成
- `org` / `user` scope: 解決された Project に draft item を作成

戻り値: 作成した Issue の URL / draft item id を stdout に出力、exit 0。

### `gh tasks list` ✅

scope 別のタスク一覧。

```bash
gh tasks list [--scope ...] [--repo ...] [--project ...] [--limit <n>]
```

- `repo` scope: open Issue を一覧
- `org` / `user` scope: Projects v2 のアイテムを一覧
- `--limit` のデフォルトは 30

### `gh tasks today` ✅

今日(UTC 0 時始まり `[start, end)`)に更新されたアイテム一覧。

```bash
gh tasks today [--scope ...] [--repo ...] [--project ...]
```

### `gh tasks plan [--period daily|weekly|sprint]` ✅

日次 / 週次 / sprint 計画。

```bash
gh tasks plan [--period daily|weekly|sprint] [--scope ...] [--repo ...] [--project ...] [--dry-run]
```

- `repo` scope: 期間に対応する Milestone を作成 / 再利用し、`updatedAt` が期間内の open Issue を bind
- `org` / `user` scope: 期間に対応する Projects v2 Iteration を選択(無ければ今日を含む iteration にフォールバック)し、期間内のアイテムの Iteration field を更新
- `--dry-run`: 候補表示のみで mutation は行わない
- 期間境界は IANA タイムゾーン(`TZ` env → システム tz → UTC フォールバック)のローカル 0 時に揃える
- `--period` のデフォルトは `weekly`

### `gh tasks triage` ✅

未トリアージ一覧(`repo` scope: ラベル無しの Issue。`org` / `user` scope: `Status` 未設定または `Triage` のアイテム)。

```bash
gh tasks triage [--scope ...] [--repo ...] [--project ...] [--limit <n>]
```

- `--limit` のデフォルトは 20

### `gh tasks done <id>` ✅

Issue を close(`repo`: `<id>` は Issue 番号)、または Projects v2 アイテムの `Status` を `Done` に変更(`org` / `user`: `<id>` は project item node id、例 `PVTI_xxx`)。

```bash
gh tasks done <id> [--scope ...] [--repo ...] [--project ...]
```

### `gh tasks review [--period daily|weekly|sprint]` ✅

振り返りサマリを Markdown で生成。

```bash
gh tasks review [--period daily|weekly|sprint] [--scope ...] [--repo ...] [--project ...]
```

- `repo` scope: 期間内に `closedAt` した Issue と `mergedAt` した PR を集計
- `org` / `user` scope: `Status` が `Done` かつ `updatedAt` が期間内の Projects v2 アイテムを集計
- `--period` のデフォルトは `weekly`

### `gh tasks standup [--mine]` ✅

スタンドアップ用の活動サマリを Markdown で生成(Yesterday / Today / Blockers の 3 セクション)。

```bash
gh tasks standup [--mine] [--since <iso8601>] [--scope ...] [--repo ...] [--project ...]
```

- `--since` のデフォルトは 24h 前
- `--mine` は viewer が author または assignee のアイテムに絞る。DraftIssue は author/assignee を持たず `--mine` 下では除外される

### `gh tasks link <pr> <task>` ✅

PR と Issue / Project アイテムの紐付け。

```bash
gh tasks link <pr> <task> [--scope ...] [--repo ...] [--project ...]
```

- `repo` scope: PR body に `Closes #<task>` を追記(冪等 — 既にリンク済の場合はそれを報告)
- `org` / `user` scope: PR と Issue を同じ Projects v2 ボードに追加し、両者を同じビューに並べる(Issue ↔ PR の relation は PR body の `Closes` キーワードから GitHub が導出)

## skill 連携

各コマンドには対応する skill SSOT が `src/skills/{name}/SKILL.md`(ja)+ `SKILL.en.md`(en)に存在する。adapter で 4 エージェント(claude-code / codex-cli / gemini-cli / copilot)向けに `dist/{adapter}/` へ配信される。詳細は repo-internal [ADR-0004](../adr/0004-skill-frontmatter-schema.md)。

## 関連

- [scope-detection.md](./scope-detection.md): `--scope` の優先順
- [locale-detection.md](./locale-detection.md): `--lang` の優先順
- [projects-v2-setup.md](./projects-v2-setup.md): `org` / `user` scope に必要な Projects v2 field 定義
- [src/skills/](../../src/skills/): 各コマンドに対応する skill SSOT
