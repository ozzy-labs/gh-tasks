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

[handbook ADR-0018](https://github.com/ozzy-labs/handbook/blob/main/adr/0018-agent-adapter-architecture.md) の adapter 機構を再利用し、`dist/{claude-code,codex-cli,gemini-cli,copilot}/.agents/skills/{name}/SKILL.md` を生成する。

> adapter 配信パイプライン(`scripts/build-skills.mjs`)は @ozzylabs/skills の lib 抽出と並行整備中。SKILL.md 自体は CLI(`gh tasks ...`)を呼ぶラッパーとして利用可能。
