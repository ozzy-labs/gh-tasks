# Claude Code Recipes

Claude Code から `gh-tasks` の CLI と skill を併用するためのレシピ集。

## 前提

- Claude Code がインストール済み(`claude` コマンドが PATH 上)
- `gh extension install ozzy-labs/gh-tasks` 完了済み
- `gh auth login` 完了済み
- リポジトリ初期セットアップは [installation.md](../installation.md) を参照

## skill の取り込み

Claude Code は `.claude/skills/{name}/SKILL.md` を skill 定義として読み込む。`gh-tasks` の adapter は同じパスに `task-add` / `task-plan` / `task-triage` / `task-review` / `task-standup` / `task-link-pr` を配置する。

> **Note**: v0.1.0 時点では consumer リポへの自動配信パイプラインは整備中([Issue #16](https://github.com/ozzy-labs/gh-tasks/issues/16))。確定するまでは下記の手元ビルドを使い、`dist/claude-code/.claude/skills/` を consumer の `.claude/skills/` 配下に手動コピーする。

```bash
pnpm run build:skills    # dist/claude-code/.claude/skills/{name}/SKILL.md を生成
```

skill の SSOT は `src/skills/{name}/SKILL.md`。frontmatter の `name` / `description` / `allowed-tools` を Claude Code が認識し、必要に応じて自律的に起動する(auto-trigger)。手動で呼び出す場合は `/task-add` のようにスラッシュコマンド形式を使う。

## 利用シーン

### 1. 朝の週次計画

週初めに `weekly` の計画を立てる:

```text
/task-plan --period weekly --scope user
```

skill が `gh tasks plan` を `--dry-run` で試行 → 候補を提示 → 確定した時点で本実行する流れ。`--scope` を省略すると git remote から推定される。

### 2. inbox triage

未トリアージの Issue / draft item を一気に整理する:

```text
/task-triage --scope org --limit 10
```

skill は label 付け / scope 振り分け / close 判断をユーザーに確認しながら進める。

### 3. 会話中のタスク化

実装中に「この件は別タスクに切り出すべき」と気付いたら、その場で:

```text
/task-add 'Refactor scope-detection cache to use LRU' --scope repo
```

skill が会話文脈から本文 / 受け入れ条件 / 関連 PR を抽出して `gh tasks add` を呼び出す。戻り値の Issue URL を確認すれば完了。

### 4. PR 作成時の紐付け

PR を出した直後に対応する Issue / Project item と紐付け:

```text
/task-link-pr 123 456
```

repo scope では PR body に `Closes #456` が冪等に追記される。org / user scope では同じ Project ボードに両者が並ぶ。

### 5. 一日の終わりの振り返り

```text
/task-review --period daily --scope user
```

完了 / 進行中 / ブロッカーの 3 セクションで Markdown 出力される。学びと持ち越し課題を skill が確認してくる。

### 6. チーム共有のスタンドアップ

朝会の前に直近 24h を要約:

```text
/task-standup --mine --scope org
```

「昨日 / 今日 / ブロッカー」の Markdown が出るので、Slack や議事録にそのまま貼れる。

## CLI と skill の使い分け

- **CLI (`gh tasks ...`) を直接叩く**: 自動化スクリプト・cron・CI から呼ぶとき、または引数を完全に制御したいとき
- **skill (`/task-*`) を使う**: 会話文脈を含めたいとき(本文を文脈から抽出する `task-add` 等)、対話的に確認しながら進めたいとき、複数ステップの判断を skill に任せたいとき

skill は内部で CLI を呼ぶだけなので、副作用は同じ。失敗した場合は CLI を直接実行してエラーメッセージを確認すると早い。

## Trouble shooting

### skill が認識されない

- `.claude/skills/{name}/SKILL.md` が存在するか確認
- frontmatter の `name` フィールドが skill ディレクトリ名と一致しているか確認
- Claude Code を再起動(skill 一覧はセッション開始時にロードされる)

### `--scope` 自動判定が失敗する

- git remote `origin` が無い、または `gh` が認証されていない可能性
- `--scope user` を明示するか、[scope-detection.md](../scope-detection.md) の優先順を確認

### `gh tasks` が見つからない

- `gh extension install ozzy-labs/gh-tasks` が未実行
- `gh extension list` で確認できる

### Projects v2 のフィールドが見つからない

- `org` / `user` scope の初回利用時は [projects-v2-setup.md](../projects-v2-setup.md) のフィールド定義が必要
- `Status` / `Iteration` の最低 2 フィールドがプロジェクトに存在することを確認

## 関連

- [cli-reference.md](../cli-reference.md): 全コマンド / フラグ
- [concepts.md](../concepts.md): scope / item / iteration の用語
- [src/skills/](../../../src/skills/): skill SSOT
