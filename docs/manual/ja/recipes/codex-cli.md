# Codex CLI Recipes

Codex CLI から `gh-tasks` の CLI と skill を併用するためのレシピ集。

## 前提

- Codex CLI がインストール済み
- `gh extension install ozzy-labs/gh-tasks` 完了済み
- `gh auth login` 完了済み
- リポジトリ初期セットアップは [installation.md](../guides/installation.md) を参照

## skill の取り込み

Codex CLI は `AGENTS.md` を起点として、参照される skill 本体を `.agents/skills/{name}/SKILL.md` から解決する。`gh-tasks` の adapter は両方を配信する:

- `.agents/skills/{name}/SKILL.md` — skill 本体(SSOT そのまま)
- `AGENTS.md.snippet` — `AGENTS.md` 末尾に挿入する skill 一覧 marker block

```bash
# 1. gh-tasks 側で adapter 出力を生成
pnpm run build:skills    # dist/codex-cli/.agents/skills/{name}/SKILL.md と dist/codex-cli/AGENTS.md.snippet を生成

# 2. consumer リポのルートで commons の sync-skills.sh を MARKER_TAG 上書きで実行
MARKER_TAG=@ozzylabs/gh-tasks bash /path/to/commons/sync-skills.sh -y \
  /path/to/gh-tasks/dist \
  .
```

`.agents/skills/{name}/SKILL.md` のコピーと `AGENTS.md` の marker block 挿入が一括で実施される。詳細は [`skills-sync/README.md`](../../../../skills-sync/README.md) を参照。

`AGENTS.md` には以下のような marker block が挿入される:

```markdown
<!-- begin: @ozzylabs/gh-tasks -->

## gh-tasks Skills

- `task-add` — ...
- `task-plan` — ...

<!-- end: @ozzylabs/gh-tasks -->
```

Codex CLI はこの一覧を見て、skill 名を会話中で指示されたときに `.agents/skills/{name}/SKILL.md` を参照する。

## 利用シーン

### 1. 朝の週次計画

```text
task-plan を週次で実行して、user scope の項目を整理してください。
```

Codex CLI が `task-plan` skill を読み、`gh tasks plan --period weekly --scope user` を呼び出す。

### 2. inbox triage

```text
org scope で task-triage、limit 10 で。
```

Codex CLI は `task-triage` skill の手順に従って未トリアージ項目を取得し、判定をユーザーに提示する。

### 3. 会話中のタスク化

実装中の会話で気付いた todo を直接タスク化:

```text
「scope-detection の cache を LRU に置き換える」を repo scope のタスクとして起票して。
```

skill が文脈から本文を整え、`gh tasks add` を呼び出す。

### 4. PR 作成時の紐付け

```text
PR #123 を Issue #456 に link して。
```

`task-link-pr` skill が `gh tasks link 123 456` を実行する。

### 5. 一日の終わりの振り返り

```text
task-review を daily / user scope で。
```

Markdown サマリが返るので、`gh-tasks review` の出力をそのまま Slack や議事録に貼れる。

### 6. チーム共有のスタンドアップ

```text
直近 24h の自分の活動を standup でまとめて、org scope で。
```

`task-standup --mine --scope org` 相当が実行される。

## CLI と skill の使い分け

- **CLI を直接叩く**: スクリプト化したいとき、引数を完全制御したいとき、自動化したいとき
- **skill 経由**: 会話文脈を反映したいとき(`task-add` のタスク本文抽出など)、複数判断を skill に任せたいとき

Codex CLI の skill は markdown ベースの手順書として動作するので、`SKILL.md` の手順をエージェントが順に実行する。CLI と異なり、判断ステップが含まれる点に注意。

## Trouble shooting

### skill が認識されない

- `AGENTS.md` 内の marker block が存在するか確認(`<!-- begin: @ozzylabs/gh-tasks -->`)
- `.agents/skills/{name}/SKILL.md` のファイルが存在するか確認
- `MARKER_TAG=@ozzylabs/gh-tasks bash /path/to/commons/sync-skills.sh -y /path/to/gh-tasks/dist .` を再実行 (idempotent)

### `--scope` 自動判定が失敗する

- git remote `origin` が無い、または `gh` 未認証
- `--scope user` を明示するか、[scope-detection.md](../reference/scope-detection.md) を参照

### `AGENTS.md` の marker block が壊れた

- `MARKER_TAG=@ozzylabs/gh-tasks bash /path/to/commons/sync-skills.sh -y /path/to/gh-tasks/dist .` を再実行すれば marker 間を idempotent に書き換えて回復
- 手動編集する場合は marker の外側だけを編集する

### `gh tasks` が見つからない

- `gh extension install ozzy-labs/gh-tasks` 未実行
- `gh extension list` で確認

### Projects v2 のフィールド不足

- `org` / `user` scope の初回利用時は [projects-v2-setup.md](../guides/projects-v2-setup.md) のフィールド定義が必要

## 関連

- [cli.md](../reference/cli.md): 全コマンド / フラグ
- [concepts.md](../concepts.md): scope / item / iteration の用語
- [src/skills/](../../../../src/skills/): skill SSOT
