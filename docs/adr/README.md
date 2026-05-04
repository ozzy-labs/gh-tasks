# Architecture Decision Records (repo-internal)

このディレクトリは `gh-tasks` リポ内に閉じる技術判断を記録する。

## 構成方針

- 番号は `0001` から連番、ファイル名は `NNNN-{kebab-case-title}.md`
- 言語は日本語(repo-internal な意思決定記録、翻訳しない — ADR-0002)
- template は [template.md](./template.md) を使用

## Index

| # | Title | Status |
| --- | --- | --- |
| 0001 | [Bun `--compile` をバイナリビルドに採用](./0001-use-bun-compile-for-binary.md) | Accepted |
| 0002 | [i18n は Japanese SSOT + English mirror](./0002-i18n-japanese-ssot.md) | Superseded by [0005](./0005-i18n-reader-based-ssot.md) |
| 0003 | [GraphQL は Octokit 経由、`gh api` shell-out 不採用](./0003-graphql-via-octokit.md) | Accepted |
| 0004 | [SKILL.md frontmatter 最小スキーマ](./0004-skill-frontmatter-schema.md) | Accepted |
| 0005 | [i18n SSOT を読み手ベースに再設計し docs/ を再構成](./0005-i18n-reader-based-ssot.md) | Accepted |
