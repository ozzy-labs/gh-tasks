# Projects v2 フィールドのセットアップ

`gh-tasks` の `org` / `user` scope は GitHub Projects v2 を backing storage として使う。最低限必要なフィールド定義をここに記載する。

## 個人(`user` scope)

個人 Project v2 に以下のフィールドを定義:

| Field | Type | 用途 |
| --- | --- | --- |
| Title | (built-in) | item の 1 行サマリ |
| Status | Single select | `Triage` / `Todo` / `In Progress` / `Done`(`Triage` は `gh tasks triage` の明示マーカー、`Status` 未設定のアイテムも triage 対象) |
| Iteration | Iteration | 週次 / sprint 計画(`gh tasks plan` の対象) |

## チーム(`org` scope)

組織の Project v2 に上記 + 以下を追加:

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

`gh tasks projects init` がこの YAML を直接消費して Project 作成 + field 配置を 1 コマンドで実行する:

```bash
gh tasks projects init --template user --title "gh-tasks personal"
gh tasks projects init --template org --owner <org> --title "team board"
gh tasks projects init packages/templates/projects-v2/user.yaml --title "from path"
```

`--dry-run` で生成予定の field 一覧を確認できる。`gh project field-create` を直接叩くフォールバック手順は [`packages/templates/README.md`](../../packages/templates/README.md) を参照。

## scope 別の対応

| scope | Project | フィールド要求 |
| --- | --- | --- |
| `repo` | (未使用、Milestones を使う) | — |
| `org` | Organization Project | 上記の team セット |
| `user` | 個人 Project | 上記の personal セット |

## 関連

- [concepts.md](./concepts.md): scope / iteration の用語
- [cli-reference.md](./cli-reference.md): `gh tasks plan` 等の挙動
