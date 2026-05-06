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
- `--body '<detail>'` / `--body=<detail>`: Issue / draft item の本文を指定(省略時は body なし)

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
- `org` / `user` scope: 期間に対応する Projects v2 Iteration を選択し、期間内のアイテムの Iteration field を更新。Iteration の選択は次の優先順:
  1. 期間タイトルに完全一致する iteration
  2. today を含む iteration
  3. 開始日が直近未来の iteration
  4. 上記いずれもなければ最後に利用可能な iteration
- `--dry-run`: 候補表示のみで mutation は行わない
- 期間境界は IANA タイムゾーン(`TZ` env → システム tz → UTC フォールバック)のローカル 0 時に揃える。`daily` は 1 日、`weekly` は月曜開始 7 日、`sprint` は今日始まり 14 日
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

### `gh tasks projects init` ✅

Projects v2 ボードを YAML テンプレートから作成し、custom field を一括追加する。

```bash
gh tasks projects init [<yaml-path> | --template user|org] --title '<project-title>' [--owner <login>|@me] [--dry-run]
```

- 位置引数: YAML パス(`templates/projects-v2/{user,org}.yaml` 形式)
- `--template user|org`: バンドル済み YAML を使う(位置引数と排他)
- `--owner <login>`: project owner(user / org login、デフォルト `@me`)
- `--title <string>`: 必須
- `--dry-run`: 作成予定の field 一覧のみを表示し、mutation を発行しない
- field type: `text` / `number` / `date` / `single_select` / `iteration` / `repository`(`repository` は built-in のため自動でスキップ)
- `single_select` の options は `color: GRAY` 固定で作成(UI で色変更可)

戻り値: 作成した Project の URL を stdout に出力、exit 0。

### `gh tasks projects init-templates` ✅

バンドル済みの `user` / `org` Projects v2 field テンプレートを stdout に出力する。ローカルで baseline をカスタマイズする前に取り出す用途。

```bash
gh tasks projects init-templates
```

- 引数 / フラグなし
- 出力は両テンプレートを単一 stream として出力し、それぞれ `# --template user` / `# --template org` のヘッダ行で区切られる。YAML splitter にパイプするかファイルへリダイレクトして利用する
- テンプレートはバイナリに同梱されており、`templates/projects-v2/{user,org}.yaml` の内容と同一

### `gh tasks link <pr> <task>` ✅

PR と Issue / Project アイテムの紐付け。

```bash
gh tasks link <pr> <task> [--scope ...] [--repo ...] [--project ...]
```

- `repo` scope: PR body に `Closes #<task>` を追記(冪等 — 既にリンク済の場合はそれを報告)
- `org` / `user` scope: PR と Issue を同じ Projects v2 ボードに追加し、両者を同じビューに並べる(Issue ↔ PR の relation は PR body の `Closes` キーワードから GitHub が導出)

### `gh tasks install-skills` ✅

gh-tasks の skill bundle を consumer リポへ配置する。リポ内のエージェント痕跡(`.claude/` / `AGENTS.md` / `.gemini/` / `.github/copilot-instructions.md`)を auto-detect し、各 adapter の所定パスにファイルを書き出す。adapter ごとの manifest で provenance を追跡するため、再実行は冪等。

```bash
gh tasks install-skills [--agent <name>[,<name>...]] [--target <path>] \
                        [--namespace <prefix>] [--force] \
                        [--dry-run] [--check] [--uninstall]
```

フラグ:

- `--agent <name>[,<name>...]`: 明示的な agent 指定(`claude-code` / `codex-cli` / `gemini-cli` / `copilot`)。カンマ / 空白区切り。省略時は auto-detect
- `--target <path>`: 配置先 (consumer リポルート)。既定は cwd
- `--namespace <prefix>`: prefix で skill 名を rename(例: `--namespace gh-tasks` で `task-add` → `gh-tasks-add`)。on-disk のディレクトリ名と SKILL.md frontmatter `name:` の両方が書き換わる
- `--force`: 非管理の既存 skill を上書き。原本は `<path>.bak` に退避
- `--dry-run`: 書き込まずに実行予定のみ表示
- `--check`: on-disk が embedded SSOT と乖離していれば非ゼロ終了(CI dogfooding 用)
- `--uninstall`: manifest 記載のファイルを削除。共有集約ファイル(`AGENTS.md` / `.gemini/settings.json` / `.github/copilot-instructions.md`)は adapter 間で reference count されるため、partial uninstall でも他 adapter が必要としているファイルは保持される

adapter 別の振る舞い:

| Agent | 所有ファイル | 共有ファイル | Manifest |
| --- | --- | --- | --- |
| `claude-code` | `.claude/skills/<name>/SKILL.md` | (なし) | `.claude/skills/.gh-tasks-manifest.json` |
| `codex-cli` | `.agents/skills/<name>/SKILL.md` | `AGENTS.md` (marker block) | `.agents/skills/.gh-tasks-manifest.json` |
| `gemini-cli` | (なし) | `.gemini/settings.json` (union merge) / `AGENTS.md` (marker block) | `.gemini/.gh-tasks-manifest.json` |
| `copilot` | (なし) | `.github/copilot-instructions.md` (marker block) | `.github/.gh-tasks-copilot-manifest.json` |

衝突処理: 既存の非管理ファイルが配置先にあると `ActionConflict` を発生させ、デフォルトでは上書きを拒否して `--namespace` / `--force` を解決策として表示する。共有集約ファイルは衝突を発生させない — gh-tasks が所有するのは marker block(`<!-- begin: @ozzylabs/gh-tasks --> ... <!-- end: ... -->`)または特定の JSON キーのみ。

戻り値: 実行予定のアクションと adapter ごとのサマリ(`{created} 件新規 / {updated} 件更新 / {skipped} 件変更なし`)を stdout に出力。成功時は exit 0、衝突 / エラー / `--check` drift 時は非ゼロ。

Renovate auto-sync 経路(`configs/skills-sync/<adapter>` preset)も同じ on-disk layout と marker tag を target にしているため、両経路は相互運用可能。最短 UX は `install-skills`、自動更新派は Renovate を選択する。

## Exit code

`gh tasks` は非ゼロ exit を 2 種類に分けている:

- `0` — 成功
- `1` — runtime 失敗: GitHub API エラー、token / auth 不在、repo / project / issue が API レスポンスで見つからなかった、その他の実行時失敗
- `2` — argument validation 失敗: `--scope` / `--project` / `--period` の値が不正、設定ファイルの構文エラー、必須 positional arg の欠落(例: `gh tasks add` に `<title>` なし)、API 呼び出し前に拒否される template / yaml 入力エラー

shell script では `$?` で分岐できる:

```bash
gh tasks list --scope=invalid
case $? in
  0) echo OK ;;
  2) echo "flag を直してね" ;;
  *) echo "ネットワーク / API のリトライ余地あり" ;;
esac
```

## skill 連携

各コマンドには対応する skill SSOT が `skills/{name}/SKILL.md`(ja)+ `SKILL.en.md`(en)に存在する。adapter で 4 エージェント(claude-code / codex-cli / gemini-cli / copilot)向けに `dist/{adapter}/` へ配信される。詳細は repo-internal [ADR-0004](../../../adr/0004-skill-frontmatter-schema.md)。

## 関連

- [scope-detection.md](./scope-detection.md): `--scope` の優先順
- [locale-detection.md](./locale-detection.md): `--lang` の優先順
- [projects-v2-setup.md](../guides/projects-v2-setup.md): `org` / `user` scope に必要な Projects v2 field 定義
- [skills/](../../../../skills/): 各コマンドに対応する skill SSOT
