[English](README.md) | 日本語

# gh-tasks

GitHub Projects v2 / Issues / Milestone を横断するタスク管理のための GitHub CLI extension + skill bundle。個人 todo、単体プロジェクト、プロジェクト横断調整の 3 用途を 1 つの抽象でカバーする。

`gh tasks` は 3 スコープ(`repo` / `org` / `user`)を統一的に扱うため、同じコマンドが個人タスク、単体リポのバックログ、OzzyLabs Platform 全体の調整いずれにも使える。

本パッケージは [OzzyLabs handbook ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md) の決定を実装し、[ADR-0018](https://github.com/ozzy-labs/handbook/blob/main/adr/0018-agent-adapter-architecture.md) の agent adapter 機構経由で Claude Code / Codex CLI / GitHub Copilot / Gemini CLI 向けに skill を配布する。skill SSOT は [`@ozzylabs/skills`](https://github.com/ozzy-labs/skills)([handbook ADR-0016](https://github.com/ozzy-labs/handbook/blob/main/adr/0016-create-skills-repo.md))、v0.1.0 実装仕様は [handbook reviews/2026-04-30-gh-tasks-design.md](https://github.com/ozzy-labs/handbook/blob/main/reviews/2026-04-30-gh-tasks-design.md) で確定。

## ステータス

初期 scaffold 段階。下記 CLI / skill は v0.1.0 のターゲット。repo 内の判断は [docs/adr/](docs/adr/)、設計ドキュメントは [docs/ja/](docs/ja/) を参照。

## インストール

```bash
gh extension install ozzy-labs/gh-tasks
```

短縮 alias(任意):

```bash
gh alias set task tasks
```

認証は `gh auth login` で取得済のトークンを extension が継承する(追加の認証導線なし)。

## CLI コマンド(v0.1.0 ターゲット)

| コマンド | 用途 |
| --- | --- |
| `gh tasks add <title>` | Issue / Project draft item の追加(`--scope repo\|org\|user`、`--repo <name>`) |
| `gh tasks list` / `gh tasks today` | 一覧表示(`--scope` でスコープ切替) |
| `gh tasks plan` | 週次 / イテレーション計画(repo は Milestone、org/user は Iteration) |
| `gh tasks triage` | 未トリアージ Issue / draft の整理 |
| `gh tasks done <id>` | 完了化(repo: Issue close、org/user: Status → Done) |
| `gh tasks review [--daily\|--weekly\|--sprint]` | 振り返り |
| `gh tasks standup [--mine]` | 個人 / チーム活動サマリ |
| `gh tasks link <pr> <task>` | PR と Issue / Project 項目の紐付け |

`--scope` のデフォルトは「作業ディレクトリの git remote から推論 → `~/.config/ozzylabs/gh-tasks.toml` の `default_scope` → `repo`」の順で決定。

## Skills(v0.1.0 ターゲット)

| Skill | 用途 |
| --- | --- |
| `task-add` | 会話文脈からタスク化 |
| `task-plan` | 日次 / 週次 / スプリント計画 |
| `task-triage` | inbox triage |
| `task-review` | daily / weekly retrospective |
| `task-standup` | 活動サマリ |
| `task-link-pr` | PR と項目の紐付け |

Renovate auto-sync で 4 エージェント分の SKILL.md が consumer リポに配信される。consumer の `renovate.json` に以下を追加する:

```jsonc
{
  "extends": ["github>ozzy-labs/gh-tasks//skills-sync"]
}
```

## スコープ対応

| Scope | 用途 | データ源 |
| --- | --- | --- |
| `repo` | 単体プロジェクトの実装作業 | Issues + Milestones |
| `org` | プロジェクト横断調整 | `OzzyLabs Platform` Project v2 |
| `user` | 個人 todo / 日次計画 | 個人 Project v2 |

## 規約

- **コミット**: [Conventional Commits](https://www.conventionalcommits.org/)
- **ブランチ**: GitHub Flow + squash merge のみ
- **ブランチ命名**: `<type>/<short-description>`

## License

[MIT](LICENSE)
