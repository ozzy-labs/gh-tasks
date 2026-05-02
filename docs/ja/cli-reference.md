# CLI リファレンス

`gh tasks` の全コマンド / フラグ。v0.1.0 ターゲット。

凡例:

- ✅ 実装済
- 🚧 v0.1.0 計画(未実装)

## 共通フラグ

- `--scope repo|org|user`: 対象スコープ。省略時は自動判定([scope-detection.md](./scope-detection.md))
- `--repo <owner>/<name>`: repo scope のターゲット。省略時は git remote `origin` から推定
- `--lang ja|en`: 出力言語。省略時は `LC_ALL` → `LANG` env → デフォルト `en` の順で解決([locale-detection.md](./locale-detection.md))
- `--help`, `-h`: ヘルプ表示
- `--version`, `-v`: バージョン表示

## コマンド

### `gh tasks add <title>` ✅ repo scope 実装済

Issue / Project draft item を追加する。

```bash
gh tasks add '<title>' [--scope repo] [--repo <owner>/<name>] [--body '<detail>']
```

- `repo` scope: GitHub Issue を作成
- `org` / `user` scope: Projects v2 draft item を作成(🚧 v0.1.0 後続)

戻り値: 作成した Issue / item の URL を stdout に出力、exit 0。

### `gh tasks list` 🚧

scope 別のタスク一覧。

### `gh tasks today` 🚧

今日の todo を抽出。

### `gh tasks plan [--period daily|weekly|sprint]` 🚧

週次 / イテレーション計画。`repo` scope は Milestone を更新、`org` / `user` scope は Project v2 Iteration を更新。

### `gh tasks triage` 🚧

未トリアージ整理。ラベル付け、scope 振り分け、close 判断を補助。

### `gh tasks done <id>` 🚧

完了化(`repo`: Issue close、`org` / `user`: Status → Done)。

### `gh tasks review [--period daily|weekly|sprint]` 🚧

振り返りサマリを生成。

### `gh tasks standup [--mine]` 🚧

活動サマリ。

### `gh tasks link <pr> <task>` 🚧

PR と Issue / Project 項目の紐付け。

## skill 連携

各コマンドには対応する skill SSOT が `src/skills/{name}/SKILL.md`(ja)+ `SKILL.en.md`(en)に存在する。adapter で 4 エージェント(claude-code / codex-cli / gemini-cli / copilot)向けに `dist/{adapter}/` へ配信される。詳細は repo-internal [ADR-0004](../adr/0004-skill-frontmatter-schema.md)。

## 関連

- [scope-detection.md](./scope-detection.md): `--scope` の優先順
- [src/skills/](../../src/skills/): 各コマンドに対応する skill SSOT
