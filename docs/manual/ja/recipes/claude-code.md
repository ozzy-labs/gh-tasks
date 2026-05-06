# Claude Code Recipes

Claude Code から `gh-tasks` の CLI と skill を併用するためのレシピ集。

## 前提

- Claude Code がインストール済み(`claude` コマンドが PATH 上)
- `gh extension install ozzy-labs/gh-tasks` 完了済み
- `gh auth login` 完了済み
- リポジトリ初期セットアップは [installation.md](../guides/installation.md) を参照

## skill の取り込み

Claude Code は `.claude/skills/{name}/SKILL.md` を skill 定義として読み込む。最短手順はワンショット install:

```bash
cd /path/to/your-repo
gh tasks install-skills            # `.claude/` または CLAUDE.md から claude-code を auto-detect
```

`task-add` / `task-plan` / `task-triage` / `task-review` / `task-standup` / `task-link-pr` を `.claude/skills/{name}/SKILL.md` に書き出し、`.claude/skills/.gh-tasks-manifest.json` で provenance を追跡するので再実行は冪等。主なバリエーション:

- `gh tasks install-skills --agent claude-code` — auto-detect が効かない場合の明示指定
- `gh tasks install-skills --namespace gh-tasks` — 衝突回避用の rename install(`/task-add` → `/gh-tasks-add`)
- `gh tasks install-skills --force` — 非管理の既存 skill を上書き(原本は `<path>.bak` に退避)
- `gh tasks install-skills --dry-run` — 実行予定のアクションのみ表示
- `gh tasks install-skills --uninstall` — manifest 記載のファイルを削除

skill の更新を Renovate 経路で取り込みたい場合は [`configs/skills-sync/README.md`](../../../../configs/skills-sync/README.md) を参照。両経路は配置先と marker tag を共有するため相互運用可能で、片方から他方へ切り替えても spurious な差分は出ない。

skill の SSOT は `skills/{name}/SKILL.md`。frontmatter の `name` / `description` / `allowed-tools` を Claude Code が認識し、必要に応じて自律的に起動する(auto-trigger)。手動で呼び出す場合は `/task-add` のようにスラッシュコマンド形式を使う。

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

- `.claude/skills/{name}/SKILL.md` が存在するか確認。なければ repo ルートで `gh tasks install-skills` を実行
- frontmatter の `name` フィールドが skill ディレクトリ名と一致しているか確認(install コマンドはこれを保証する。手編集すると乖離する可能性がある)
- Claude Code を再起動(skill 一覧はセッション開始時にロードされる)

### `--scope` 自動判定が失敗する

- git remote `origin` が無い、または `gh` が認証されていない可能性
- `--scope user` を明示するか、[scope-detection.md](../reference/scope-detection.md) の優先順を確認

### `gh tasks` が見つからない

- `gh extension install ozzy-labs/gh-tasks` が未実行
- `gh extension list` で確認できる

### Projects v2 のフィールドが見つからない

- `org` / `user` scope の初回利用時は [projects-v2-setup.md](../guides/projects-v2-setup.md) のフィールド定義が必要
- `Status` / `Iteration` の最低 2 フィールドがプロジェクトに存在することを確認

## 関連

- [cli.md](../reference/cli.md): 全コマンド / フラグ
- [concepts.md](../concepts.md): scope / item / iteration の用語
- [skills/](../../../../skills/): skill SSOT
