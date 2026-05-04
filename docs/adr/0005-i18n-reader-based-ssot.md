# 0005. i18n SSOT を読み手ベースに再設計し docs/ を再構成する

- Status: Accepted
- Date: 2026-05-04
- Deciders: ozzy
- Tags: i18n, docs, structure

## Context

ADR-0002 で「主メンテナの起稿言語が ja だから全領域を ja SSOT に揃える」原則を採用したが、運用で複数の不整合が表面化した。

1. **前提条件の精緻化**: 開発者は個人 1 名(ja ネイティブ)、ユーザー向けプライマリは en、ja もサポート。「起稿言語 = SSOT」より「読み手 = SSOT」の方が筋が良い場面が多い
2. **CLI 出力 ja SSOT との構造的不整合**: ユーザー向けプライマリが en なら、ユーザーが読む CLI 出力 / エラーメッセージも en SSOT が自然(`error.*` 等の key 自体が英語であることとも整合)
3. **ハードコード ja 文字列の混入**: `packages/gh-tasks/src/lib/project.ts:88,98` に i18n key を経由しない ja リテラルが存在。「ja SSOT」感覚が ja 直書きを許容しやすい
4. **`docs/` の軸混在**: `docs/{en,ja}/`(言語軸)と `docs/adr/`(カテゴリ軸)が同階層に並び、構造が不整合。さらに `docs/{en,ja}/` 内にユーザーマニュアルと仕様メモが混在
5. **LLM 翻訳の成熟**: 起稿言語と SSOT 言語を分離するコストが現実的に小さくなった
6. **gh extension としての配布**: 本家 `gh` は `cli.github.com/manual/` で "Manual" を採用、Unix CLI 系の伝統と整合する命名が望ましい

## Decision

i18n SSOT 原則と `docs/` 構造を以下のとおり再定義する。

### 原則: 読み手ベース SSOT

| 主な読み手 | SSOT | 翻訳方針 |
| --- | --- | --- |
| ユーザー(エンドユーザー) | **en** | ja mirror、LLM 翻訳 + セルフレビュー |
| 開発者本人(メンテナ) | **ja 単一** | 翻訳しない |

起稿言語と SSOT 言語は分離する(LLM 翻訳前提)。「翻訳遅延が SSOT 鮮度を直撃しない」を SSOT 言語選択の根拠にしない。

### 対象別マトリクス

| 対象 | 形式 | SSOT | ADR-0002 比 |
| --- | --- | --- | --- |
| `README.md` / `README.ja.md` | en + ja | **en** | 変更なし |
| `docs/manual/{en,ja}/` | en + ja mirror | **en** | 🔁 ja → en |
| `docs/adr/` | ja 単一 | ja | 変更なし |
| `docs/design/` | ja 単一 | ja | 新設 |
| `AGENTS.md` / `CLAUDE.md` | ja 単一 | ja | 変更なし |
| `src/skills/*/SKILL.md` | ja SSOT + `SKILL.en.md` mirror | ja | 変更なし |
| `packages/gh-tasks/src/i18n/*.json` | en + ja | **en** | 🔁 ja → en |
| ハードコード文字列 | 全廃、i18n key 経由必須 | — | 🆕 |

### `docs/` ディレクトリ構造

```text
docs/
├── manual/                  # ユーザーマニュアル(en SSOT + ja mirror)
│   ├── en/                  # SSOT
│   │   ├── README.md
│   │   ├── concepts.md      # Explanation
│   │   ├── guides/          # How-to: installation / projects-v2-setup / troubleshooting
│   │   ├── reference/       # Reference: cli / scope-detection / locale-detection
│   │   └── recipes/         # Tutorial: claude-code / codex-cli / copilot / gemini-cli
│   └── ja/                  # mirror(同構造)
├── adr/                     # 不変な意思決定記録(ja 単一)
└── design/                  # living な設計ドキュメント(ja 単一)
```

設計原則:

- **最上位はカテゴリ軸で統一**(`manual/` / `adr/` / `design/`)。言語軸は `manual/` 配下に閉じ込める
- **`docs/` を「プロジェクトのドキュメント全般を集約する場所」と再定義**。OSS 透明性原則に従い、`adr/` `design/` も公開対象とする(「非公開」ではなく「メンテナ向け」と位置づける)
- **`manual/` 命名根拠**: 本家 `gh` の `cli.github.com/manual/` 慣例、Unix CLI 系の伝統(GNU, PostgreSQL, Docker manuals)、gh extension としての整合
- **Diátaxis 簡易版を `manual/{en,ja}/` 内で適用**: `guides/`(How-to)/ `reference/`(Reference) / `recipes/`(Tutorial) + ルート `concepts.md`(Explanation)
- **`adr/` と `design/` の使い分け**:
  - **`adr/`**: 1 決定 = 1 ファイル、**不変**、連番(`NNNN-*.md`)
  - **`design/`**: テーマ単位、**living**、名前ベース(`architecture.md`, `data-flow.md` 等)

### CLI locale 解決順(ADR-0002 から継承、変更なし)

1. `--lang` フラグ
2. `~/.config/ozzylabs/gh-tasks.toml` の `lang`
3. `LC_ALL` 環境変数(POSIX どおり LANG より優先)
4. `LANG` 環境変数
5. デフォルト `en`

### 強制機構

ハードコード文字列の検知のため、以下を CI / lint で強制する:

- 非 ASCII 文字(主に ja)を含む文字列リテラルを `packages/gh-tasks/src/` 配下で検知し、エラー扱い
- 例外は `src/i18n/ja.json` 等の翻訳定義ファイルのみ

### 実装フェーズ

- **Phase 0**(本 ADR + 構造変更): ADR-0005 受理、ADR-0002 を Superseded、`docs/` 再編(ファイル移動 + 内部リンク更新)、`docs/design/README.md` 新設
- **Phase 1**(CLI i18n 反転): `src/i18n/{en,ja}.json` の en SSOT 反転、ハードコード文字列の i18n 化、非 ASCII 検知 lint 追加
- **Phase 2**(docs i18n 反転): `docs/manual/{en,ja}/` の en SSOT 反転(LLM 翻訳 + セルフレビュー)

## Consequences

### Positive

- ユーザー向けプライマリ en と整合、英語ユーザー体験の鮮度をプライマリ言語で保てる
- 軸混在の解消: 最上位カテゴリ軸で統一、言語軸は `manual/` 内に閉じ込められる
- ja ハードコードを lint で機械的に検知可能(en SSOT に倒すと非 ASCII 文字が異常値として浮く)
- 本家 `gh` の "Manual" 慣例に揃い、将来 `cli.github.com` 風の docs サイト化が自然
- ADR / AGENTS / CLAUDE / SKILL の ja 単一は維持、開発者の起稿コストを最小化
- `docs/{en,ja}/` の中身が Diátaxis 簡易構造で整理され、文書追加時の置き場判断が機械的に
- 「ADR ほど重くない設計メモ」の置き場として `design/` が確保される

### Negative / Trade-offs

- 既存 ja docs を en SSOT に反転する一時コスト(LLM 翻訳 + セルフレビュー、Phase 2)
- ja ネイティブ開発者が「最初に en で書く」認知負荷(LLM 補助で緩和可能)
- ADR-0002 の判断を 5 日で覆す。ただし前提条件(ユーザー向けプライマリの明確化、LLM 翻訳成熟の認識)の精緻化が根拠
- `docs/adr/` `docs/design/` も GitHub Pages 化時に exclude が必要(`_config.yml` 1 行追加)
- 既存 docs / コード内のリンク更新作業(機械的に grep / sed で対応可能)

## Alternatives considered

- **ADR-0002 維持 + ハードコード解消のみ** — 最小変更だが、「ユーザー向けプライマリ en」と「CLI 出力 ja SSOT」の構造的矛盾が解消されず、長期的に技術的負債を残す。不採用
- **全領域 en SSOT** — 起稿言語との乖離が大きく、ADR / AGENTS / CLAUDE の翻訳遅延が SSOT 鮮度を直撃。メンテナ 1 人の起稿コスト上昇。不採用
- **ja 単一(en mirror 全廃)** — gh extension としての国際配布で英語ユーザーの入口が失われる。不採用
- **`internal-docs/` 等で物理分離** — 公開 / 非公開を物理ディレクトリで分離する案。OSS 透明性慣例(Kubernetes / React / Rust 等は内部ドキュメントも公開)から外れ、得られるメリット(GitHub Pages exclude 不要)は `_config.yml` 1 行で代替可能。不採用
- **`user-guide/` ラップ** — `manual/` の代替命名候補。エンタープライズ系慣例(AWS, Terraform)だが、gh 本家 "Manual" 慣例と乖離、ハイフン入りで長い。不採用
- **ラップなし(`docs/{en,ja}/` + `docs/adr/` + `docs/design/`)** — 軸混在(言語軸とカテゴリ軸の同階層配置)を許容する案。`docs/README.md` で説明することで運用上の混乱は回避可能だが、`en/` `ja/` ディレクトリ名から内容カテゴリが推測できず、論理純度が低い。不採用
- **Diátaxis 厳格 4 分類** — `tutorials/` `how-to/` `reference/` `explanation/` を厳格に分離。文書数に対してディレクトリ過多(現状 `concepts.md` のみが Explanation)。簡易版で十分。不採用

## References

- Related repo ADR: [ADR-0002](./0002-i18n-japanese-ssot.md)(本 ADR で Superseded)
- gh CLI Manual: <https://cli.github.com/manual/>
- Diátaxis Framework: <https://diataxis.fr/>
