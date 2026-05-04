# トラブルシューティング

## 認証エラー: `AuthError: GH_TOKEN / GITHUB_TOKEN が未設定`

**原因**: `gh auth login` が未実行、または GitHub Actions runner で `GITHUB_TOKEN` env が継承されていない。

**解決**:

```bash
gh auth login
# または
export GH_TOKEN=<your-token>
```

## `--repo` 解決失敗: `RepoError: --repo フラグも git remote origin も解決できません`

**原因**: 現在のディレクトリが git リポジトリでない、または remote `origin` が未設定。

**解決**:

```bash
gh tasks add 'title' --repo=<owner>/<name>
# または git remote を設定
git remote add origin git@github.com:<owner>/<name>.git
```

## `--scope org|user` で「Project の指定が必要です」

**原因**: `org` / `user` scope は Projects v2 のターゲットが必要だが、`--project` も config の対応キー(`org_project` / `user_project`)も未設定。

**解決**:

```bash
gh tasks list --scope=user --project=<owner>/<number>
# または ~/.config/ozzylabs/gh-tasks.toml にデフォルトを保存
echo 'user_project = "<owner>/<number>"' >> ~/.config/ozzylabs/gh-tasks.toml
```

config の全スキーマは [installation.md](./installation.md) を参照。

## `gh agent-task` との semantic 衝突

GitHub CLI 公式の `gh agent-task` (preview) と本拡張の `gh tasks` でコマンド名のニアミスがあり得る(監視対象として認識)。顕在時は本リポの ADR で対応方針を決定する。

## API rate limit

**原因**: 短時間に大量の GraphQL リクエスト(GitHub の rate limit は 1 時間あたり 5000 / 認証済)。

**解決**:

- 数分待ってリトライ
- 大量データを扱う場合は `--limit` で結果数を絞る(`gh tasks list` / `gh tasks triage` で対応)
- CI で実行する場合は `GITHUB_TOKEN` env を渡してレート上限を引き上げる

## `gh extension install` がコケる

**原因**: GitHub Releases にプラットフォーム対応バイナリが存在しない。

**解決**: `release.yaml` で配信される 5 ターゲット(darwin x86_64 / arm64、linux x86_64 / arm64、windows x86_64)に該当するか確認([repo-internal ADR-0001](../adr/0001-use-bun-compile-for-binary.md))。該当しない環境では `git clone` + `bun run` で開発実行可能。

## 関連

- [installation.md](./installation.md)
- [scope-detection.md](./scope-detection.md)
