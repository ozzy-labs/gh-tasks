#!/usr/bin/env node
// Build the gh-tasks skill distribution.
//
// Reads canonical (ja SSOT) skill files from src/skills/{name}/SKILL.md,
// validates the frontmatter against repo-internal ADR-0004, and runs each
// agent adapter to emit OutputFiles under dist/{adapter-id}/.
//
// Adapters are pure transform functions per handbook ADR-0018; this
// orchestrator is the sole writer to dist/.
//
// The English mirror (SKILL.en.md) is not currently consumed by the build;
// it is kept alongside the canonical SKILL.md as a hand-maintained reference
// for future locale adapters. Promoting en mirror to a build artefact is
// tracked separately.

import { existsSync } from 'node:fs';
import { cp, mkdir, readdir, readFile, rm, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ClaudeCodeAdapter } from './adapters/claude-code.mjs';
import { CodexCliAdapter } from './adapters/codex-cli.mjs';
import { CopilotAdapter } from './adapters/copilot.mjs';
import { GeminiCliAdapter } from './adapters/gemini-cli.mjs';
import { assertRequiredFields, parseSkillDocument } from './lib/frontmatter.mjs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, '..');
const SRC = join(ROOT, 'src', 'skills');
const DIST = join(ROOT, 'dist');

// Local staging targets for dogfooding: this repo's own Claude Code / Codex CLI
// sessions read from .claude/skills/ and .agents/skills/. Commons skills land
// there via sync-commons; task-* skills are produced by this script and must be
// mirrored explicitly so /task-* are usable while developing gh-tasks itself.
const LOCAL_STAGES = [
  { distSubpath: '.claude/skills', localPath: join(ROOT, '.claude', 'skills') },
  { distSubpath: '.agents/skills', localPath: join(ROOT, '.agents', 'skills') },
];

const ADAPTERS = [
  new ClaudeCodeAdapter(),
  new CodexCliAdapter(),
  new GeminiCliAdapter(),
  new CopilotAdapter(),
];

// Repo-internal ADR-0004 frontmatter schema.
const REQUIRED_FIELDS = ['name', 'description', 'description_en', 'allowed-tools', 'locale'];

async function readSkillNames() {
  const entries = await readdir(SRC, { withFileTypes: true });
  return entries
    .filter((e) => e.isDirectory())
    .map((e) => e.name)
    .sort();
}

async function loadSkills() {
  const names = await readSkillNames();
  if (names.length === 0) {
    throw new Error(`No skills found under ${SRC}`);
  }
  const skills = [];
  for (const name of names) {
    const file = join(SRC, name, 'SKILL.md');
    const raw = await readFile(file, 'utf8');
    const label = `src/skills/${name}/SKILL.md`;
    const { frontmatter, body } = parseSkillDocument(raw, label);
    assertRequiredFields(frontmatter, REQUIRED_FIELDS, label);
    if (frontmatter.name !== name) {
      throw new Error(
        `${label}: frontmatter name='${frontmatter.name}' does not match directory name='${name}'`
      );
    }
    if (frontmatter.locale !== 'ja') {
      throw new Error(
        `${label}: frontmatter locale='${frontmatter.locale}' must be 'ja' for the canonical SSOT (ADR-0002, ADR-0004)`
      );
    }
    skills.push({
      name,
      description: frontmatter.description,
      descriptionEn: frontmatter.description_en,
      locale: frontmatter.locale,
      frontmatter,
      body,
      raw,
    });
  }
  return skills;
}

async function writeAdapterOutputs(skills) {
  for (const adapter of ADAPTERS) {
    const id = adapter.constructor.id;
    if (!id) {
      throw new Error(`${adapter.constructor.name} is missing static id`);
    }
    const adapterRoot = join(DIST, id);
    if (existsSync(adapterRoot)) {
      await rm(adapterRoot, { recursive: true, force: true });
    }
    const outputs = adapter.generate(skills);
    for (const out of outputs) {
      const dest = join(adapterRoot, out.relativePath);
      await mkdir(dirname(dest), { recursive: true });
      await writeFile(dest, out.content);
    }
  }
}

async function stageLocalCopies(skills) {
  const skillNames = new Set(skills.map((s) => s.name));
  for (const adapter of ADAPTERS) {
    const adapterRoot = join(DIST, adapter.constructor.id);
    for (const stage of LOCAL_STAGES) {
      const distSkillsDir = join(adapterRoot, stage.distSubpath);
      if (!existsSync(distSkillsDir)) continue;
      await mkdir(stage.localPath, { recursive: true });
      for (const name of skillNames) {
        const src = join(distSkillsDir, name);
        if (!existsSync(src)) continue;
        const dest = join(stage.localPath, name);
        if (existsSync(dest)) {
          await rm(dest, { recursive: true, force: true });
        }
        await cp(src, dest, { recursive: true });
      }
    }
  }
}

async function main() {
  if (!existsSync(SRC)) {
    console.log('[build-skills] no src/skills/ — nothing to build');
    return;
  }
  const skills = await loadSkills();
  await writeAdapterOutputs(skills);
  await stageLocalCopies(skills);

  console.log(`✓ Built ${skills.length} skill(s) for ${ADAPTERS.length} adapters`);
  for (const adapter of ADAPTERS) {
    console.log(`  dist/${adapter.constructor.id}/`);
  }
  console.log('  staged into .claude/skills/, .agents/skills/');
  for (const skill of skills) {
    console.log(`  - ${skill.name}`);
  }
}

main().catch((err) => {
  console.error(`✗ build-skills failed: ${err.message}`);
  process.exit(1);
});
