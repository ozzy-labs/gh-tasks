# GitHub Copilot Recipes

GitHub Copilot から `gh-tasks` の CLI と skill を併用するためのレシピ集。

## 前提

- GitHub Copilot Chat / Coding Agent が利用可能
- `gh extension install ozzy-labs/gh-tasks` 完了済み
- `gh auth login` 完了済み
- リポジトリ初期セットアップは [installation.md](../guides/installation.md) を参照

## skill の取り込み

GitHub Copilot は `.github/copilot-instructions.md` を最上位の指示書として読み込む。`gh-tasks` の adapter は `.github/copilot-instructions.md.snippet` を配信し、それを `copilot-instructions.md` の marker block に挿入する。Copilot は SKILL.md を直接ロードしないため、skill は名前 + 説明文の一覧として与えられる。

```bash
# 1. gh-tasks 側で adapter 出力を生成
gh tasks build-skills    # dist/copilot/.github/copilot-instructions.md.snippet を生成

# 2. consumer リポのルートで commons の sync-skills.sh を MARKER_TAG 上書きで実行
MARKER_TAG=@ozzylabs/gh-tasks bash /path/to/commons/sync-skills.sh -y \
  /path/to/gh-tasks/dist \
  .
```

snippet が consumer の `.github/copilot-instructions.md` の marker block に挿入される。詳細は [`skills-sync/README.md`](../../../../skills-sync/README.md) を参照。

`copilot-instructions.md` には `AGENTS.md` の内容を取り込むよう指示するか、AGENTS.md と並行して skill 一覧 marker block を入れる。snippet 例:

```markdown
<!-- begin: @ozzylabs/gh-tasks -->

## gh-tasks Skills

- `task-add` — ...
- `task-plan` — ...

<!-- end: @ozzylabs/gh-tasks -->
```

Copilot はこれを読み、ユーザーが skill 名で指示したときに対応する `gh tasks` コマンドを推測して提案する。

## 利用シーン

### 1. 朝の週次計画

Copilot Chat に:

```text
@workspace task-plan で weekly / user scope を実行して。
```

Copilot が `gh tasks plan --period weekly --scope user` を提案する。

### 2. inbox triage

```text
@workspace task-triage を org scope, limit 10 で。
```

`gh tasks triage --scope org --limit 10` の実行を提案する。triage 後の判定(label / scope 振り分け)は Copilot が文脈から提案する。

### 3. 会話中のタスク化

実装中の会話で:

```text
@workspace 今気付いた「scope-detection の cache を LRU に置き換える」を repo scope のタスクで起票して。
```

`gh tasks add` を提案する。

### 4. PR 作成時の紐付け

PR テンプレや PR コメントから:

```text
@workspace この PR (#123) を Issue #456 と link。
```

`gh tasks link 123 456` を提案。repo scope では PR body に `Closes #456` が冪等に追記される。

### 5. 一日の終わりの振り返り

```text
@workspace task-review を daily / user scope で実行。
```

Markdown サマリが返るので、Issue コメントや wiki に貼り付けできる。

### 6. チーム共有のスタンドアップ

```text
@workspace standup --mine で org scope の直近 24h を要約。
```

`gh tasks standup --mine --scope org` を提案する。

## CLI と skill の使い分け

GitHub Copilot は SKILL.md 本体を読み込まないため、skill 機構は名前 + 説明文の一覧に留まる。

- **CLI を直接叩く**: ターミナルから自分で実行 / スクリプト化したいとき
- **Copilot 経由の提案**: 自然言語からコマンドを組み立てさせたいとき、PR / Issue コメントから呼び出したいとき

Copilot Coding Agent からは `gh` CLI 経由で実行できるので、PR comment trigger と組み合わせると `task-link-pr` の自動化に向く。

## Trouble shooting

### skill 名で呼んでも反応しない

- `.github/copilot-instructions.md` の marker block が存在するか確認
- リポを開き直して Copilot が instructions を再読込するか確認
- `MARKER_TAG=@ozzylabs/gh-tasks bash /path/to/commons/sync-skills.sh -y /path/to/gh-tasks/dist .` を再実行 (idempotent)

### `--scope` 自動判定が失敗する

- git remote `origin` が無い、または `gh` 未認証
- `--scope user` を明示するか、[scope-detection.md](../reference/scope-detection.md) を参照

### `gh tasks` が見つからない

- `gh extension install ozzy-labs/gh-tasks` 未実行
- Copilot Coding Agent 環境では事前に extension をインストールするセットアップステップが必要
- `gh extension list` で確認

### `copilot-instructions.md` の snippet が壊れた

- `MARKER_TAG=@ozzylabs/gh-tasks bash /path/to/commons/sync-skills.sh -y /path/to/gh-tasks/dist .` を再実行 (idempotent)
- snippet は idempotent なので複数回実行しても安全

### Projects v2 のフィールド不足

- `org` / `user` scope の初回利用時は [projects-v2-setup.md](../guides/projects-v2-setup.md) のフィールド定義が必要

## 関連

- [cli.md](../reference/cli.md): 全コマンド / フラグ
- [concepts.md](../concepts.md): scope / item / iteration の用語
- [skills/](../../../../skills/): skill SSOT
