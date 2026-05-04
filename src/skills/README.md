# Skills SSOT

このディレクトリは `gh-tasks` 固有 skill の SSOT(canonical SKILL.md)を保持する。

## 構成

```text
src/skills/
├── task-add/SKILL.md
├── task-plan/SKILL.md
├── task-triage/SKILL.md
├── task-review/SKILL.md
├── task-standup/SKILL.md
└── task-link-pr/SKILL.md
```

各 SKILL.md の frontmatter スキーマは [docs/adr/0004-skill-frontmatter-schema.md](../../docs/adr/0004-skill-frontmatter-schema.md) を参照。

## ビルド

```bash
pnpm run build:skills
```

adapter 機構経由で `dist/{claude-code,codex-cli,gemini-cli,copilot}/.agents/skills/{name}/SKILL.md` を生成する。

consumer 側の sync 手順は [`skills-sync/README.md`](../../skills-sync/README.md) を参照(Renovate preset + `MARKER_TAG=@ozzylabs/gh-tasks` での `sync-skills.sh`)。
