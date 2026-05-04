# 0004. SKILL.md frontmatter 最小スキーマを確定する

- Status: Accepted
- Date: 2026-04-30
- Deciders: ozzy
- Tags: skills, i18n, frontmatter

## Context

本リポは「正準 SKILL.md → 4 エージェント adapter で transform」機構を採用する。skill frontmatter は次の要件を満たす必要がある:

1. **i18n**(repo-internal ADR-0002): SKILL.md 本文は ja SSOT、`SKILL.en.md` は en mirror。adapter で en 出力も生成する
2. **CLI 連携**: skill は `gh tasks <subcmd>` を呼ぶ薄いラッパ。`allowed-tools` に `Bash(gh:*)` の許可が必要
3. **description の英語版**: skill discovery(各エージェントの skill listing)で英語ユーザー向けに英訳 description が欲しい

skill frontmatter は `name` / `description` / `allowed-tools` を中核として、本 ADR では本リポ固有の追加フィールドを最小限で確定する。

## Decision

`src/skills/{name}/SKILL.md` の frontmatter は以下のスキーマに従う。

```yaml
---
name: task-add
description: 会話文脈からタスクを追加する。GitHub Issue / Project draft / repo Milestone を自動判定。
description_en: Add a task from conversation context. Auto-detects GitHub Issue / Project draft / repo Milestone.
allowed-tools: Bash(gh:*)
locale: ja
---
```

| フィールド | 必須 | 用途 |
| --- | --- | --- |
| `name` | ✅ | skill 識別子(kebab-case、`task-` prefix) |
| `description` | ✅ | 本文言語(SSOT = ja)での 1 行説明 |
| `description_en` | ✅ | 英語 description(en mirror に取り込む) |
| `allowed-tools` | ✅ | `Bash(gh:*)`(本リポの skill は `gh tasks <subcmd>` を呼ぶため) |
| `locale` | ✅ | SSOT 言語。本リポでは固定で `ja` |

### `SKILL.en.md`(en mirror)のスキーマ

`SKILL.en.md` は SSOT に併設する英訳 reference で、現状は build adapter の対象外(`scripts/build-skills.mjs` は ja SSOT のみを dist へ配信)。手動メンテナンスのため、frontmatter は SSOT を踏襲しつつ次の差分を持つ:

```yaml
---
name: task-add
description: Add a task from conversation context. Auto-detects GitHub Issue / Project draft / repo Milestone.
allowed-tools: Bash(gh:*)
locale: en
---
```

| フィールド | en mirror での扱い |
| --- | --- |
| `name` | SSOT と同一(kebab-case、`task-` prefix) |
| `description` | SSOT の `description_en` の値を入れる(en の 1 行説明) |
| `description_en` | en mirror では省略(`description` 自体が en) |
| `allowed-tools` | SSOT と同一(`Bash(gh:*)`) |
| `locale` | `en` 固定(SSOT は `ja`) |

将来 locale adapter を実装したら SSOT から自動生成に切り替える(現状は Issue で追跡)。それまでは ja を更新したら手動で en mirror も更新する運用とする。

## Consequences

### Positive

- adapter 機構に最小限の差分(`description_en` / `locale` 追加)で乗る
- 4 エージェントの skill listing で英語ユーザーが意味を把握できる(`description_en` を `--locale en` で出力)
- `allowed-tools: Bash(gh:*)` 固定で skill 側の認可ポリシーが明示化、不要な権限拡大を防ぐ
- frontmatter スキーマが `src/skills/` に閉じるため、本リポ固有要件を局所化できる

### Negative / Trade-offs

- `description_en` の手動メンテが必要(ja を更新したら en も追従)— lint で missing/empty を検出する CI チェックを追加余地
- `locale` は現状 `ja` 固定だが、フィールドとして持っておくことで将来 en SSOT skill が混在しても破綻しない(冗長性として許容)

## Alternatives considered

- **`description` のみ(英訳なし)** — 英語ユーザーは skill 名から推測するしかない。不採用
- **`description.ja` / `description.en` のネスト構造** — 既存 SKILL.md スキーマ慣例から乖離、adapter 機構の改造が必要。不採用
- **adapter が i18n を一切扱わず、`SKILL.en.md` を完全手書きとする** — frontmatter 不一致(`name` の typo 等)が発生しうる。不採用

## References

- Related repo ADR: [ADR-0002](./0002-i18n-japanese-ssot.md)(i18n SSOT)
