# skills-sync (Renovate preset)

`@ozzylabs/gh-tasks` の skill bundle を consumer リポに自動配信する Renovate preset。`@ozzylabs/skills` の `skills-sync` と並走できる構造で、フィールド名は `gh_tasks_commit:`(skills 側は `skills_commit:`)で衝突しない。

## consumer 側のセットアップ

### 1. `renovate.json` で preset を extend

`.claude/skills/` 等に gh-tasks 固有 skill を取り込みたいなら、利用するアダプターの opt-in preset を extend する。複数アダプターを同時に使う場合はすべて列挙する。

```jsonc
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": [
    "github>ozzy-labs/gh-tasks//skills-sync/claude-code",
    "github>ozzy-labs/gh-tasks//skills-sync/codex-cli"
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

Renovate 自身は `.commons/sync.yaml` の SHA を bump するだけで、`dist/` 内容を consumer に展開する作業は別経路で実施する。`@ozzylabs/commons` の `sync-skills.sh` 相当を gh-tasks 専用にも対応させる作業は本 PR のスコープ外で、後続 Issue で tracking する。

## ファイル構成

| file | 役割 |
| --- | --- |
| `default.json` | 主 preset。`gh_tasks_commit:` を git-refs データソースで監視 |
| `claude-code.json` | adapter opt-in。`adapter:claude-code` ラベル付与 |
| `codex-cli.json` | 同上、`adapter:codex-cli` |
| `gemini-cli.json` | 同上、`adapter:gemini-cli` |
| `copilot.json` | 同上、`adapter:copilot` |

## 関連

- [handbook ADR-0018](https://github.com/ozzy-labs/handbook/blob/main/adr/0018-agent-adapter-architecture.md): 4 エージェント adapter 機構
- [handbook ADR-0019](https://github.com/ozzy-labs/handbook/blob/main/adr/0019-adapter-aware-sync-conventions.md): consumer の `extends` パターン
- [@ozzylabs/skills の skills-sync](https://github.com/ozzy-labs/skills/tree/main/skills-sync): 並走する preset
