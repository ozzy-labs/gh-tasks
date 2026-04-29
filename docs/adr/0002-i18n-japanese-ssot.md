# 0002. i18n は Japanese SSOT + English mirror で運用する

- Status: Accepted
- Date: 2026-04-30
- Deciders: ozzy
- Tags: i18n, docs, skills

## Context

[handbook ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md) の Decision 4 で「i18n 対応(en/ja)」が必須要件として確定。具体的な SSOT は [handbook reviews/2026-04-30-gh-tasks-design.md](https://github.com/ozzy-labs/handbook/blob/main/reviews/2026-04-30-gh-tasks-design.md) 9 で「Japanese SSOT + English mirror」と決定した。本 ADR で repo 内の運用ルールを確定する。

OzzyLabs 全体の状況:

- 既存リポは README で `README.md`(en)+ `README.ja.md`(ja)パターン(skills、road、commons 等)
- AGENTS.md / CLAUDE.md は ja のみ(社内エージェント向け)
- 設計ドキュメント / SKILL.md / CLI 出力 / エラーメッセージ は本リポで初の i18n 対応となる

## Decision

i18n の SSOT 言語と配置は以下のとおりに固定する。

| 対象 | 形式 | SSOT | 翻訳方針 |
| --- | --- | --- | --- |
| README | `README.md`(en)+ `README.ja.md`(ja) | en(既存規約) | 手動同期 |
| 設計 docs | `docs/ja/` + `docs/en/` 並列 | **ja** | 手動同期(初期)、将来 LLM 半自動化 |
| ADR(本ディレクトリ) | `docs/adr/{NNNN}-*.md`(ja 単一) | ja | **翻訳しない**(社内意思決定文書) |
| SKILL.md | `SKILL.md`(ja)+ `SKILL.en.md`(en) | **ja** | adapter で `--locale en` 出力 |
| CLI 出力 | `src/i18n/ja.json` + `en.json` | **ja** | キーベース |
| エラーメッセージ | i18n 対象 | **ja** | 同上 |

例外:

- README のみ en SSOT(既存 OzzyLabs 規約に従う)
- ADR は ja 単一で翻訳しない(意思決定文脈は社内のみで読まれる)

CLI locale 解決順:

1. `--lang` フラグ
2. `~/.config/ozzylabs/gh-tasks.toml` の `lang`
3. `LANG` / `LC_ALL` 環境変数(`ja*` で ja、それ以外で en)
4. デフォルト `en`(海外ユーザー考慮)

## Consequences

### Positive

- ozzy(主たるメンテナ)が ja で書くため、SSOT が常に最新(英訳の翻訳遅延が SSOT 鮮度に影響しない)
- 設計 docs / SKILL.md / CLI 出力で SSOT 言語を統一(ja)し、「どっちが正?」の判断コストを排除
- 既存 README 規約を温存し、海外ユーザーに対して最低限の英語入口を確保
- ADR を翻訳しないことで、意思決定の速度を維持

### Negative / Trade-offs

- 設計 docs / SKILL.en.md の英訳遅延が発生しうる(初期は手動同期)
- 海外コントリビュータが ADR を読めない(本リポは個人プロジェクト方針につき許容、CONTRIBUTING.md 参照)
- adapter での `--locale en` 出力実装が必要(skill 側、handbook ADR-0018 の追加要件)

## Alternatives considered

- **English SSOT 統一** — README 規約には合うが、ozzy(主メンテナ)の起稿言語が ja のため翻訳遅延が SSOT 鮮度を直撃。不採用
- **ja 単一(en 廃止)** — gh extension は GitHub 経由で全世界に配布されうるため、最低限の英語入口は必要。不採用
- **DeepL / LLM 自動翻訳のみで運用** — レビュー無しの自動翻訳は意味取り違えリスクが高い、初期は手動同期、将来 LLM 半自動化を検討

## References

- Related handbook ADR: [ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md)(Decision 4 i18n 必須)、[ADR-0018](https://github.com/ozzy-labs/handbook/blob/main/adr/0018-agent-adapter-architecture.md)(SKILL.md 4 エージェント adapter)
- Related design review: [reviews/2026-04-30-gh-tasks-design.md](https://github.com/ozzy-labs/handbook/blob/main/reviews/2026-04-30-gh-tasks-design.md) 9
- External: なし
