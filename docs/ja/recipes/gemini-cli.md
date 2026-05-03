# Gemini CLI Recipes

Gemini CLI から `gh-tasks` の CLI と skill を併用するためのレシピ集。

## 前提

- Gemini CLI がインストール済み
- `gh extension install ozzy-labs/gh-tasks` 完了済み
- `gh auth login` 完了済み
- リポジトリ初期セットアップは [installation.md](../installation.md) を参照

## skill の取り込み

Gemini CLI は `.gemini/settings.json` の `context.fileName` で指定したファイル(通常 `AGENTS.md`)を読み込む。`gh-tasks` の adapter は `AGENTS.md.snippet` のみを配信し、それを `AGENTS.md` の marker block に挿入する。Gemini CLI 自体には Claude Code のような `SKILL.md` 自動ロード機構が無いため、skill は AGENTS.md 内の説明文として参照される。

> **Note**: v0.1.0 時点では consumer リポへの自動配信パイプラインは整備中([Issue #16](https://github.com/ozzy-labs/gh-tasks/issues/16))。確定するまでは下記の手元ビルドを使い、`AGENTS.md.snippet` の marker block 挿入を手動で行う。

```bash
pnpm run build:skills    # dist/gemini-cli/AGENTS.md.snippet を生成
```

`.gemini/settings.json` 例:

```jsonc
{
  "context": {
    "fileName": "AGENTS.md"
  }
}
```

`AGENTS.md` の marker block:

```markdown
<!-- begin: @ozzylabs/gh-tasks -->

## gh-tasks Skills

- `task-add` — 会話文脈からタスクを追加する...
- `task-plan` — 日次 / 週次 / イテレーション計画を実行する...

<!-- end: @ozzylabs/gh-tasks -->
```

Gemini CLI はこの一覧を読み、ユーザーが skill 名で指示したとき該当する `gh tasks` コマンドを推測して呼び出す。

## 利用シーン

### 1. 朝の週次計画

```text
task-plan で weekly / user scope を実行して。
```

Gemini CLI が AGENTS.md の skill 説明を参照し、`gh tasks plan --period weekly --scope user` を実行する。

### 2. inbox triage

```text
task-triage を org scope / limit 10 で。
```

Gemini CLI が `gh tasks triage --scope org --limit 10` を呼び、結果に対して整理判断をユーザーに提案する。

### 3. 会話中のタスク化

```text
「scope-detection の cache を LRU に置き換える」を repo scope のタスクで起票して。
```

`gh tasks add 'Refactor scope-detection cache to use LRU' --scope repo` 相当が実行される。

### 4. PR 作成時の紐付け

```text
PR #123 を Issue #456 と link。
```

`gh tasks link 123 456` を実行する。

### 5. 一日の終わりの振り返り

```text
task-review を daily / user scope で。
```

Markdown サマリが返り、Slack や議事録に貼れる。

### 6. チーム共有のスタンドアップ

```text
standup --mine で org scope の活動を直近 24h まとめて。
```

`gh tasks standup --mine --scope org` 相当が実行される。

## CLI と skill の使い分け

Gemini CLI には Claude Code のような `SKILL.md` 自動ロード機構が無いため、skill 機構は薄い。実態としては:

- **CLI を直接叩く**: 厳密に挙動を制御したいとき
- **AGENTS.md 経由の skill 説明**: コマンド名を覚えていなくても skill 名(`task-add` など)で呼べる、文脈解釈をエージェントに任せたいとき

`SKILL.md` 本体は Codex CLI / Claude Code 向けに `.agents/skills/` または `.claude/skills/` に存在しているので、Gemini CLI からも参照は可能。ただしロードは自動ではない。

## Trouble shooting

### skill 名で呼んでも反応しない

- `AGENTS.md` 内の marker block が存在するか確認
- `.gemini/settings.json` の `context.fileName` が `AGENTS.md` を指しているか確認
- Gemini CLI を再起動してコンテキストを再読込

### `--scope` 自動判定が失敗する

- git remote `origin` が無い、または `gh` 未認証
- `--scope user` を明示するか、[scope-detection.md](../scope-detection.md) を参照

### `gh tasks` が見つからない

- `gh extension install ozzy-labs/gh-tasks` 未実行
- `gh extension list` で確認

### `AGENTS.md` の snippet が古い

- `pnpm run build:skills` で再生成した snippet を `AGENTS.md` の marker block に再投入([Issue #16](https://github.com/ozzy-labs/gh-tasks/issues/16) で配信パイプライン整備中)
- snippet は idempotent な更新なので、複数回実行しても安全

### Projects v2 のフィールド不足

- `org` / `user` scope の初回利用時は [projects-v2-setup.md](../projects-v2-setup.md) のフィールド定義が必要

## 関連

- [cli-reference.md](../cli-reference.md): 全コマンド / フラグ
- [concepts.md](../concepts.md): scope / item / iteration の用語
- [src/skills/](../../../src/skills/): skill SSOT
