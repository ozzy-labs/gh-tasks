# インストール

## 前提

- `gh` CLI がインストール済で `gh auth login` 完了済
- macOS / Linux / Windows のいずれか(`gh extension` 対応プラットフォーム)

## インストール手順

```bash
gh extension install ozzy-labs/gh-tasks
```

短縮 alias を設定する場合(任意):

```bash
gh alias set task tasks
```

## 認証

`gh-tasks` は `gh auth login` で取得済のトークンを `GH_TOKEN` 環境変数経由で継承する(repo-internal ADR-0003)。追加の認証手順は不要。

GitHub Actions 等の自動環境では `GITHUB_TOKEN` env でも動作する。

## 設定ファイル

`~/.config/ozzylabs/gh-tasks.toml`(`$XDG_CONFIG_HOME` を尊重):

```toml
lang = "ja"
default_scope = "repo"
org_project = "ozzy-labs/5"
user_project = "ozzy-3/2"
```

省略可能なキー:

- `lang`: `ja` / `en`。出力言語の既定値([locale-detection.md](../reference/locale-detection.md))
- `default_scope`: `repo` / `org` / `user`。スコープの既定値([scope-detection.md](../reference/scope-detection.md))
- `org_project`: `<owner>/<number>` 形式。`--scope=org` の既定 Project v2
- `user_project`: `<owner>/<number>` 形式。`--scope=user` の既定 Project v2

設定ファイルが存在しない場合は無視され、フラグ / 環境変数 / 自動推定にフォールバックする。TOML が壊れている / キーが不正値のときは分かりやすいエラーで終了する。

## Projects v2 セットアップ

`org` / `user` scope を使う場合、Projects v2 のフィールド定義が必要。手順は [projects-v2-setup.md](./projects-v2-setup.md)。

`repo` scope のみ使う場合、追加セットアップは不要(GitHub Issues / Milestones がそのまま使われる)。

## skill 配置(ワンショット)

`gh tasks install-skills` で gh-tasks の skill bundle(`task-add` / `task-plan` / `task-triage` / `task-review` / `task-standup` / `task-link-pr`)を consumer リポへ 1 ステップで配置する。`gh extension install ozzy-labs/gh-tasks` の完了後、リポルートで実行する。

```bash
cd /path/to/your-repo
gh tasks install-skills            # リポ内のエージェント痕跡から auto-detect
```

auto-detect は filesystem traces(`.claude/` / `AGENTS.md` / `.gemini/` / `.github/copilot-instructions.md`)を確認し、各 adapter の所定パスにファイルを書き出す。再実行は冪等で、adapter ごとの manifest が gh-tasks 所有のパスを記録するため次回以降は差分のみ更新される。

主なバリエーション:

- `gh tasks install-skills --agent claude-code,codex-cli` — agent を明示指定
- `gh tasks install-skills --namespace gh-tasks` — prefix で衝突回避(`/task-add` → `/gh-tasks-add`)
- `gh tasks install-skills --force` — 非管理の既存 skill を上書き(原本は `<path>.bak` に退避)
- `gh tasks install-skills --dry-run` — 実行予定のアクションのみ表示
- `gh tasks install-skills --uninstall` — manifest 記載のファイルを削除。共有集約ファイルは adapter 間で reference count される

Renovate auto-sync 経路(`configs/skills-sync/<adapter>` preset)も併存可能 — 詳細は [`configs/skills-sync/README.md`](../../../../configs/skills-sync/README.md)。両経路は同じ on-disk layout と marker tag を target にしているため相互運用可能。

エージェント別 recipe: [recipes/claude-code.md](../recipes/claude-code.md) / [recipes/codex-cli.md](../recipes/codex-cli.md) / [recipes/gemini-cli.md](../recipes/gemini-cli.md) / [recipes/copilot.md](../recipes/copilot.md)。

## 動作確認

```bash
gh tasks --version
gh tasks add 'first task' --scope=repo --repo=<owner>/<name>
```

## 関連

- [scope-detection.md](../reference/scope-detection.md)
- [troubleshooting.md](./troubleshooting.md)
- [reference/cli.md#gh-tasks-install-skills](../reference/cli.md) — `install-skills` 全フラグの reference
