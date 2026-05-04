[English](README.md) | 日本語

# gh-tasks

GitHub Projects v2 / Issues / Milestone を横断するタスク管理のための GitHub CLI extension + skill bundle。個人 todo、単体プロジェクト、プロジェクト横断調整の 3 用途を 1 つの抽象でカバーする。

`gh tasks` は 3 スコープ(`repo` / `org` / `user`)を統一的に扱うため、同じコマンドが個人タスク、単体リポのバックログ、共有 Project v2 によるプロジェクト横断調整いずれにも使える。

本 CLI は Projects v2 アクセスを単一バイナリ + skill bundle に集約し、agent adapter 機構経由で Claude Code / Codex CLI / GitHub Copilot / Gemini CLI 向けに skill を配布する。

## ステータス

v0.1.0 — feature-complete。下記 CLI コマンドおよび skill は 3 スコープ(`repo` / `org` / `user`)すべてで実装・テスト済み。リリースタグは release-please で発行する。repo 内の判断は [docs/adr/](docs/adr/)、設計ドキュメントは [docs/ja/](docs/ja/) を参照。

## インストール

```bash
gh extension install ozzy-labs/gh-tasks
```

短縮 alias(任意):

```bash
gh alias set task tasks
```

認証は `gh auth login` で取得済のトークンを extension が継承する(追加の認証導線なし)。

## CLI コマンド

| コマンド | 用途 |
| --- | --- |
| `gh tasks add <title>` | Issue / Project draft item の追加(`--scope repo\|org\|user`、`--repo <name>`、`--project <id>`) |
| `gh tasks list` / `gh tasks today` | 一覧表示(`--scope` でスコープ切替、`--limit <N>` 既定 30) |
| `gh tasks plan [--period daily\|weekly\|sprint] [--dry-run]` | 週次 / イテレーション計画(repo は Milestone、org/user は Iteration) |
| `gh tasks triage [--limit <N>]` | 未トリアージ Issue / draft の整理(既定 20) |
| `gh tasks done <id>` | 完了化(repo: Issue close、org/user: Status → Done) |
| `gh tasks review [--period daily\|weekly\|sprint]` | 振り返り |
| `gh tasks standup [--mine] [--since <iso8601>]` | 個人 / チーム活動サマリ(既定 直近 24h) |
| `gh tasks link <pr> <task>` | PR と Issue / Project 項目の紐付け |
| `gh tasks projects init` | yaml テンプレートから Project v2 を bootstrap(`--template`、`--owner`、`--title`) |

`--scope` の解決順は「明示の `--scope` フラグ → 作業ディレクトリの git remote(`origin` があれば `repo`)→ `~/.config/ozzylabs/gh-tasks.toml` の `default_scope` → `user`」。フラグの詳細は [docs/ja/cli-reference.md](docs/ja/cli-reference.md) を参照。

## Skills

| Skill | 用途 |
| --- | --- |
| `task-add` | 会話文脈からタスク化 |
| `task-plan` | 日次 / 週次 / スプリント計画 |
| `task-triage` | inbox triage |
| `task-review` | daily / weekly retrospective |
| `task-standup` | 活動サマリ |
| `task-link-pr` | PR と項目の紐付け |

Renovate auto-sync で 4 エージェント分の SKILL.md が consumer リポに配信される。利用するアダプターの sub-preset を extend する:

```jsonc
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": [
    "github>ozzy-labs/gh-tasks//skills-sync/claude-code",
    "github>ozzy-labs/gh-tasks//skills-sync/codex-cli"
  ]
}
```

詳細(全 adapter preset 一覧、`gh_tasks_commit:` を `@ozzylabs/skills` と並走で tracking する仕組み)は [`skills-sync/README.md`](skills-sync/README.md)。

## スコープ対応

| Scope | 用途 | データ源 |
| --- | --- | --- |
| `repo` | 単体プロジェクトの実装作業 | Issues + Milestones |
| `org` | プロジェクト横断調整 | Organization Project v2 |
| `user` | 個人 todo / 日次計画 | 個人 Project v2 |

## 規約

- **コミット**: [Conventional Commits](https://www.conventionalcommits.org/)
- **ブランチ**: GitHub Flow + squash merge のみ
- **ブランチ命名**: `<type>/<short-description>`

## License

[MIT](LICENSE)
