# GitHub Copilot Recipes

GitHub Copilot から `gh-tasks` の CLI と skill を併用するためのレシピ集。

## 前提

- GitHub Copilot Chat / Coding Agent が利用可能
- `gh extension install ozzy-labs/gh-tasks` 完了済み
- `gh auth login` 完了済み
- リポジトリ初期セットアップは [installation.md](../guides/installation.md) を参照

## skill の取り込み

GitHub Copilot は `.github/copilot-instructions.md` を最上位の指示書として読み込む。Copilot は SKILL.md を直接ロードしないため、skill は marker block 内の名前 + 説明文一覧として与えられる。最短手順はワンショット install:

```bash
cd /path/to/your-repo
gh tasks install-skills            # `.github/copilot-instructions.md` から copilot を auto-detect
```

これで `.github/copilot-instructions.md` の marker block に gh-tasks skill 一覧が merge され(ファイル不在なら新規作成)、`.github/.gh-tasks-copilot-manifest.json` で provenance が記録される。marker block は gh-tasks 専用領域で、その外側のコンテンツは byte-for-byte で保護される。

主なバリエーション:

- `gh tasks install-skills --agent copilot` — 明示指定
- `gh tasks install-skills --namespace gh-tasks` — 衝突回避用の rename install
- `gh tasks install-skills --uninstall` — marker block を除去。marker 外のコンテンツはそのまま残る

skill の更新を Renovate 経路で取り込みたい場合は [`configs/skills-sync/README.md`](../../../../configs/skills-sync/README.md) を参照。

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
- repo ルートで `gh tasks install-skills` を再実行(idempotent)

### `--scope` 自動判定が失敗する

- git remote `origin` が無い、または `gh` 未認証
- `--scope user` を明示するか、[scope-detection.md](../reference/scope-detection.md) を参照

### `gh tasks` が見つからない

- `gh extension install ozzy-labs/gh-tasks` 未実行
- Copilot Coding Agent 環境では事前に extension をインストールするセットアップステップが必要
- `gh extension list` で確認

### `copilot-instructions.md` の snippet が壊れた

- `gh tasks install-skills` を再実行すれば marker 間が idempotent に書き直される

### Projects v2 のフィールド不足

- `org` / `user` scope の初回利用時は [projects-v2-setup.md](../guides/projects-v2-setup.md) のフィールド定義が必要

## 関連

- [cli.md](../reference/cli.md): 全コマンド / フラグ
- [concepts.md](../concepts.md): scope / item / iteration の用語
- [skills/](../../../../skills/): skill SSOT
