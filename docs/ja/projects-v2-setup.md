# Projects v2 フィールドのセットアップ

`gh-tasks` の `org` / `user` scope は GitHub Projects v2 を backing storage として使う。最低限必要なフィールド定義をここに記載する。

> v0.1.0 では `org` / `user` scope は未実装。本ドキュメントは設計仕様の先取り記述。

## v0.1.0 ターゲット(個人 / `user` scope)

個人 Project v2 に以下のフィールドを定義:

| Field | Type | 用途 |
| --- | --- | --- |
| Title | (built-in) | item の 1 行サマリ |
| Status | Single select | `Todo` / `In Progress` / `Done` |
| Iteration | Iteration | 週次 / sprint 計画(`gh tasks plan` の対象) |

## v0.1.0 ターゲット(チーム / `org` scope)

OzzyLabs Platform Project v2 に上記 + 以下を追加:

| Field | Type | 用途 |
| --- | --- | --- |
| Repository | Repository | プロジェクト横断調整時の所属 repo |
| Project | Single select | 横断 project 識別子(個別 repo を超える単位) |

## セットアップ手順

GitHub UI で Project を作成し、Settings → Custom fields で上記フィールドを追加する。`gh project` CLI でも作成可能だが、Iteration の設定は UI のほうが容易。

### テンプレート YAML

フィールド定義の SSOT は `packages/templates/projects-v2/` に同梱:

| ファイル | 用途 |
| --- | --- |
| [`packages/templates/projects-v2/user.yaml`](../../packages/templates/projects-v2/user.yaml) | 個人 / `user` scope(Status / Iteration) |
| [`packages/templates/projects-v2/org.yaml`](../../packages/templates/projects-v2/org.yaml) | チーム / `org` scope(user セット + Repository / Project) |

`gh` CLI は現時点で `gh project create --from-yaml` を実装していないため、適用は `gh project field-create` の連続呼び出しで行う。具体的なコマンド例は [`packages/templates/README.md`](../../packages/templates/README.md) を参照。

## scope 別の対応

| scope | Project | フィールド要求 |
| --- | --- | --- |
| `repo` | (未使用、Milestones を使う) | — |
| `org` | OzzyLabs Platform Project | 上記の team セット |
| `user` | 個人 Project | 上記の personal セット |

## 関連

- [handbook ADR-0022](https://github.com/ozzy-labs/handbook/blob/main/adr/0022-create-gh-tasks-repo.md): Projects v2 採用の意思決定
- [concepts.md](./concepts.md): scope / iteration の用語
- [cli-reference.md](./cli-reference.md): `gh tasks plan` 等の挙動
