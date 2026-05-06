# Release Process

`gh-tasks` のリリースパイプライン全体を記述する。`docs/design/architecture.md` で「配布モデル: CLI バイナリ」と書いた経路の **オペレーション視点での説明**。

## ハイレベルフロー

```text
main への commit(feat/fix/perf 等)
  │
  ▼
release.yaml(workflow_run on push)
  │
  ├─ release-please job
  │    └─ release-please-action が:
  │         - Conventional Commits を集約
  │         - "release PR" を作成 / 更新(現在は PR #2)
  │         - PR は CHANGELOG.md update + version bump を含む
  │
  ▼ (ユーザーが release PR を merge → release_created == true)
  │
  ├─ build-binaries job(`cli/gh-extension-precompile@v2`)
  │    └─ 公式 Action が一括で:
  │         - go build を全 OS / arch で実行
  │         - 命名規則 <goos>-<goarch>[.exe] を自動生成
  │         - manifest.yml(プラットフォーム解決メタデータ)を発行
  │         - generate_attestations: true で SLSA provenance を発行
  │         - すべて gh release upload <tag> ...
  │
  ├─ checksums job(build-binaries 完了後)
  │    └─ `gh release download --pattern '<family>-*'` で全 platform family を回収
  │       → `*-*` glob で SHA256 集約 → checksums.txt → gh release upload
  │
  ▼
GitHub Release v0.X.Y 完成
  │
  ▼
ユーザーは `gh extension install ozzy-labs/gh-tasks` で取得可能
```

## 構成要素

### `.github/workflows/release.yaml`

3 job 構成、`push: branches: [main]` で trigger。top-level `permissions: contents: read`(read-only default)+ 各 job が必要分だけ追加(least-privilege per job、GitHub hardening 推奨パターン)。

| Job | needs | permissions | 役割 |
| --- | --- | --- | --- |
| `release-please` | — | `contents: write`、`pull-requests: write` | release-please-action 実行 |
| `build-binaries` | `release-please` | `contents: write`、`id-token: write`、`attestations: write` | `cli/gh-extension-precompile@v2` で全 OS/arch を一括生成 + manifest.yml + provenance + upload |
| `checksums` | `release-please`、`build-binaries` | `contents: write` | aggregated SHA256SUMS upload |

両方の post-release job は `if: needs.release-please.outputs.release_created == 'true'` でガードしているため、release PR が無く release-please が新規 PR を作るだけのケース(daily な commit 累積期)では起動しない。

### `release-please-config.json`

```json
{
  "packages": {
    ".": {
      "release-type": "go",
      "release-as": "0.1.0",
      "changelog-path": "CHANGELOG.md",
      "include-component-in-tag": false,
      "bump-minor-pre-major": true,
      "bump-patch-for-minor-pre-major": true,
      "extra-files": ["cmd/root.go"],
      "changelog-sections": [
        { "type": "feat", "section": "Features" },
        { "type": "fix", "section": "Bug Fixes" },
        { "type": "perf", "section": "Performance" }
      ]
    }
  }
}
```

ポイント:

- パッケージパスは `.`(リポルート、Go 移行に伴いモノレポ風 `packages/gh-tasks/` は廃止)
- `release-type: go` → Go モジュールとして認識(GitHub Releases のみ、npm publish しない)
- `include-component-in-tag: false` → タグは `vX.Y.Z` 形式(コンポーネント prefix なし)
- `bump-minor-pre-major: true` + `bump-patch-for-minor-pre-major: true` → v1.0.0 未満の段階で `feat:` も minor として扱う
- `release-as: "0.1.0"` → 初回リリース pin。release-please が初回 PR を `0.1.0` で発行するために置く(Go 移行カットオーバー時に検討された `2.0.0-rc.1` pin は 2026-05-05 に撤回、0.x 系で conventional commits 自動 bump、メジャー bump はユーザー判断)
- `extra-files: ["cmd/root.go"]` → release-please が `cmd/root.go` の `Version = "0.0.0-dev" // x-release-please-version` 行をタグ作成時に書き換える(`gh tasks --version` の表示と Release tag を同期)
- `changelog-sections` → `feat` / `fix` / `perf` のみ CHANGELOG に載る。`docs:` / `ci:` / `chore:` 等は除外

### `.release-please-manifest.json`

```json
{ ".": "0.1.0" }
```

現在の version(release-please が更新)。タグ作成時に release-please が自動 bump する。`release-as` の pin が解除されるまで提案 version は固定。

### Asset 命名規約

`cli/gh-extension-precompile@v2` は `<goos>-<goarch>[.exe]` 形式の asset 名で binary を発行する(v1 までの `gh-<extension>_<version>_<os>-<arch>[.exe]` 形式から変更)。具体的には:

```text
darwin-amd64
darwin-arm64
linux-amd64
linux-arm64
windows-amd64.exe
windows-arm64.exe
manifest.yml         ← gh extension が読むプラットフォーム解決メタデータ
checksums.txt        ← 後続 job が aggregated SHA256SUMS を upload
```

各 binary に対して GitHub が SLSA build provenance attestation を発行(`gh attestation verify <binary> --owner ozzy-labs` で検証可能)。

`checksums` job は `gh release download --pattern 'darwin-*' --pattern 'linux-*' --pattern 'freebsd-*' --pattern 'windows-*'` で全 platform family を回収し、`shopt -s nullglob` を有効化したうえで `sha256sum -- *-*` を実行する(`*-*` glob で `manifest.yml` / `checksums.txt` 自身は対象外)。

## ユーザー側の動作

リリース後、ユーザーは以下のフローでインストールできる:

```bash
# 1. インストール(latest Release から自動的に platform 適合 binary を取得)
gh extension install ozzy-labs/gh-tasks

# 2. (任意)checksum 検証
sha256sum -c checksums.txt --ignore-missing

# 3. (任意)attestation 検証
gh attestation verify darwin-arm64 --owner ozzy-labs

# 4. 認証(初回)
gh auth login

# 5. 利用
gh tasks --help
```

詳細は `docs/manual/{en,ja}/guides/installation.md` を参照。

## release-as の経緯

| 日付 | 値 | 経緯 |
| --- | --- | --- |
| 2026-05-04 (Go 移行 v1) | `2.0.0-rc.1` | TS 時代の v0.x との世代分離を狙って pin |
| 2026-05-05 | `0.1.0` | rc.1 pin を撤回し、初回リリース版として `0.1.0` に変更。0.x 系で conventional commits の自動 bump を継続する方針に切替 |

**初回リリース後の解除**: `release-as: "0.1.0"` は初回 PR が `0.1.0` で発行されるための pin。初回リリース完了後は **削除する必要がある**(放置すると release-please が永遠に同 version 提案を続け、`fix:` / `feat:` による bump が反映されない)。メジャー bump (1.0.0 / 2.0.0) はユーザー判断で別途 `release-as` または手動タグ。

## 運用上の注意

### Workflows の一時無効化(過去の運用)

過去にリポの GitHub Actions usage を節約するため、5 workflow すべて(`ci`、`pr-check`、`release`、`release-smoke`(リリース後の `gh extension install` smoke test)、`sync-commons`)を `gh workflow disable` で `disabled_manually` 状態にしていた(2026-05-04 に実施、Issue #85 で再有効化トラッキング)。

無効化中の挙動:

- main への push で release.yaml が起動しない → release PR の作成 / 更新が止まる
- release PR を merge してもタグ / Release / バイナリ生成が起こらない
- リリース実行時は事前に **release workflow だけでも `gh workflow enable release` で再有効化** する必要あり

### release PR の挙動

release-please-action は実行のたびに以下を判定して PR を再生成 / 更新する:

1. main の最新 commit が前回 release タグ以降に Conventional Commits を含むか
2. 含む場合、新しい version を提案する PR を作成 / 既存 PR を update
3. PR を merge すると release-please-action が次回実行時にタグを切り Release を作成

そのため release PR を merge した直後の workflow run で:

- release-please が tag + Release + manifest 更新を実施
- build-binaries / checksums が release_created == true で起動

### リリース前検証 (dev mode smoke)

release PR を merge する **前** に、ローカル `gh extension install .` (dev mode) による smoke test を実施する。**v0.1.0 を含むすべてのリリース前に毎回実行する** (2026-05-06 ユーザー判断、スキップしない方針)。

#### なぜ必要か

リリース後の `release-smoke.yaml` は本物 artifact (precompile binary + manifest.yml + attestation + checksums) を全 platform で smoke するが、**tag 作成後にしか走らない**。リリース前に `gh tasks <cmd>` の起動経路 (gh の subcommand discovery + sub-process exec + 引数 pass-through) を確認できる手段は、ローカル dev mode のみ。

#### 標準手順

```bash
# 1. リポルートで extension 名と一致するバイナリ名で build (gh-tasks 必須)
go build -o gh-tasks .

# 2. dev mode install (~/.local/share/gh/extensions/gh-tasks/ に symlink)
gh extension install .
gh extension list                  # gh-tasks が dev mode で list される

# 3. 主要 read-only コマンドで smoke
gh tasks --version                 # 0.0.0-dev (release-please 未走行のため)
gh tasks --help
gh tasks list
gh tasks today
gh tasks standup --mine
gh tasks build-skills --check-diff # 副作用なし (cmd/build_skills.go: runCheckDiff)

# 4. 片付け
gh extension remove gh-tasks
rm ./gh-tasks
```

ソース変更後は `go build -o gh-tasks .` を再実行するだけで反映される (symlink 経由)。

#### dev mode で確認できること / できないこと

| 観点 | dev mode | release-smoke (post-tag, 自動) |
| --- | --- | --- |
| `gh tasks <cmd>` の subcommand 解決経路 | ✅ | ✅ |
| 実 GitHub API レスポンスに対する動作 | ✅ | △ (--help のみ) |
| adapter pipeline 出力 (`build-skills --check-diff`) | ✅ | ❌ |
| 本物 precompile binary の起動 | ❌ (ローカル `go build`) | ✅ |
| Cross-compile (darwin/linux/windows × amd64/arm64) の完全性 | ❌ | ✅ |
| SLSA build provenance attestation | ❌ | ✅ (`gh attestation verify`) |
| checksums.txt 整合性 | ❌ | ✅ (`sha256sum -c`) |
| 6 platform tier-1 binary の attach 完全性 | ❌ | ✅ |

両者は重複ではなく**役割分担**。dev mode = 経路検証、release-smoke = 配布物検証。

#### スキップしない方針

通常 (`feat:` / `fix:` のみ) のリリースでも省略しない。理由:

- 一貫した手順で検証することの安心感
- dev mode 経路自体が壊れた場合の早期検知 (次回以降の手元再現にも同じ手順を使うため定期的に通したい)
- 毎回手元で触ることによる UX 確認
- コスト約 5 分は許容範囲

> 「変更パターン別ゲートでスキップ可」のような最小コスト案は、ユーザーが明示的に申し出るまで採用しない。

## Trade-offs と代替案

| 観点 | 採用 | 代替 |
| --- | --- | --- |
| version 管理 | `release-please` | 手動 `gh release create`(放棄、Conventional Commits との連動を活用) |
| Cross-compile | `cli/gh-extension-precompile@v2`(公式 Action、Go 中心) | matrix の手書き(`bun --compile` × 5、ADR-0001 旧採用案、ADR-0006 で Superseded) |
| Provenance | precompile-action の `generate_attestations: true` | 自前 GPG 署名(運用負担、見送り) |
| Checksum | aggregated `checksums.txt` | per-binary `<name>.sha256`(verbose、見送り) |

## 関連 ADR / docs

- [ADR-0006](../adr/0006-go-and-cobra-migration.md): Go + cobra + `cli/gh-extension-precompile@v2` 採用(ADR-0001 を Superseded)
- [docs/design/architecture.md](./architecture.md): 配布モデル全体
- [docs/manual/en/guides/installation.md](../manual/en/guides/installation.md): ユーザー向けインストール手順
- gh extension precompiled extension 仕様: <https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions>
- `cli/gh-extension-precompile`: <https://github.com/cli/gh-extension-precompile>
- SLSA build provenance: <https://slsa.dev/provenance/v1>
