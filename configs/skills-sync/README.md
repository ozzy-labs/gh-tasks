# skills-sync (Renovate preset)

`@ozzylabs/gh-tasks` の skill bundle を consumer リポに自動配信する Renovate preset。`@ozzylabs/skills` の `skills-sync` と並走できる構造で、フィールド名は `gh_tasks_commit:`(skills 側は `skills_commit:`)で衝突しない。

## consumer 側のセットアップ

### 1. `renovate.json` で preset を extend

`.claude/skills/` 等に gh-tasks 固有 skill を取り込みたいなら、利用するアダプターの opt-in preset を extend する。複数アダプターを同時に使う場合はすべて列挙する。

```jsonc
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": [
    "github>ozzy-labs/gh-tasks//configs/skills-sync/claude-code",
    "github>ozzy-labs/gh-tasks//configs/skills-sync/codex-cli"
  ]
}
```

`default.json` を直接 extend してもよいが、その場合は adapter ラベルが付かない。

### 2. `.commons/sync.yaml` に `gh_tasks_commit:` を追加

```yaml
gh_tasks_commit: <40-char-sha>
gh_tasks_adapters:
  - claude-code
  - codex-cli
```

Renovate がこの行の SHA を更新する PR を発行する。`gh_tasks_adapters:` は consumer 側の手動列挙で、adapter dist の取り込み対象を絞る。

### 3. file materialisation の sync スクリプト

Renovate は `.commons/sync.yaml` の `gh_tasks_commit:` を bump するだけで、`dist/{adapter}/` 内容を consumer に展開する作業は別途実施する。`@ozzylabs/commons` の `sync-skills.sh` を `MARKER_TAG` 環境変数で上書きして再利用する([commons#91](https://github.com/ozzy-labs/commons/pull/91))。

```bash
# consumer リポのルートで実行
MARKER_TAG=@ozzylabs/gh-tasks bash /path/to/commons/sync-skills.sh -y \
  /path/to/gh-tasks-clone/dist \
  .
```

これで `<!-- begin: @ozzylabs/gh-tasks -->...<!-- end: @ozzylabs/gh-tasks -->` 形式の marker block で skill / snippet が consumer リポに展開される(`@ozzylabs/skills` 既存 marker と並存)。

#### consumer 側 workflow への組み込み例

`.github/workflows/sync-gh-tasks.yaml` 等で Renovate PR への自動展開を実装する例:

```yaml
name: sync gh-tasks adapters
on:
  pull_request:
    branches: [main]
    paths:
      - .commons/sync.yaml

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.head_ref }}
      - uses: pnpm/action-setup@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: pnpm
      - name: Resolve gh_tasks_commit and clone gh-tasks
        run: |
          GH_TASKS_SHA=$(yq '.gh_tasks_commit' .commons/sync.yaml)
          git clone https://github.com/ozzy-labs/gh-tasks /tmp/gh-tasks
          (cd /tmp/gh-tasks && git checkout "$GH_TASKS_SHA" && pnpm install --frozen-lockfile && pnpm run build:skills)
      - name: Clone commons (sync-skills.sh)
        run: git clone https://github.com/ozzy-labs/commons /tmp/commons
      - name: Sync gh-tasks adapter outputs
        env:
          MARKER_TAG: '@ozzylabs/gh-tasks'
        run: bash /tmp/commons/sync-skills.sh -y /tmp/gh-tasks/dist .
      - name: Commit sync result
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add -A && git diff --cached --quiet || git commit -m "chore: sync @ozzylabs/gh-tasks adapter outputs"
          git push
```

`MARKER_TAG` 未指定時は default `@ozzylabs/skills` で動作するため、`@ozzylabs/skills` 用と `@ozzylabs/gh-tasks` 用の workflow を並走させても互いに干渉しない。

## ファイル構成

| file | 役割 |
| --- | --- |
| `default.json` | 主 preset。`gh_tasks_commit:` を git-refs データソースで監視 |
| `claude-code.json` | adapter opt-in。`adapter:claude-code` ラベル付与 |
| `codex-cli.json` | 同上、`adapter:codex-cli` |
| `gemini-cli.json` | 同上、`adapter:gemini-cli` |
| `copilot.json` | 同上、`adapter:copilot` |

## 関連

- [@ozzylabs/skills の skills-sync](https://github.com/ozzy-labs/skills/tree/main/skills-sync): 並走する preset
