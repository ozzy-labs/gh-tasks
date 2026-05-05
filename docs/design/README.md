# Design Documents (repo-internal)

このディレクトリは `gh-tasks` の **living な設計ドキュメント** を置く場所。ADR(`docs/adr/`)では重い、または ADR と性質が違うトピックを記録する。

## ADR との使い分け

| 性質 | 置き場所 |
| --- | --- |
| 1 決定 = 1 ファイル、**不変**(歴史的記録)、連番(`NNNN-*.md`) | `docs/adr/` |
| テーマ単位、**living**(更新前提)、名前ベース | `docs/design/`(本ディレクトリ) |
| 「○○を採用する」「△△は使わない」 | `docs/adr/` |
| 「現状こういう構造になっている」「このフローはこう動く」 | `docs/design/` |
| 図表が中心(mermaid 等) | `docs/design/` |
| 1 つの決定で完結 | `docs/adr/` |
| 複数の決定が織り込まれた俯瞰図 | `docs/design/`(関連 ADR を参照リンク) |

迷ったら ADR を起こす(粒度に拘らず ADR 化)、肥大化したら本ディレクトリに切り出す。

## 構成方針

- 言語は日本語(repo-internal、翻訳しない — repo-internal ADR-0005)
- ファイル名は `{kebab-case-topic}.md`(連番なし)
- ステータス管理なし(古くなったら削除 or 統合)
- 大きな決定が含まれると判断したら、ADR を起こして本ディレクトリ側はその ADR を参照する

## Index

| File | 用途 |
| --- | --- |
| [architecture.md](./architecture.md) | 全体俯瞰・モジュール境界・ディレクトリ構成・配布モデル |
| [adapter-pipeline.md](./adapter-pipeline.md) | skill SSOT → dist/{adapter}/ → consumer の処理連鎖、4 adapter の出力仕様 |
| [release-process.md](./release-process.md) | release.yaml の 3 job 構成、release-please 連携、attestations / checksums、暫定 pin の経緯 |
| [test-structure.md](./test-structure.md) | `cmd/` パッケージのテスト構成 (ファイル分担、mock 戦略、命名規則、新規テスト追加先のフロー) |
