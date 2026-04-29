#!/usr/bin/env node
// Generate dist/{adapter}/.agents/skills/{name}/SKILL.md outputs by transforming
// canonical SKILL.md files in src/skills/ via the ADR-0018 adapter mechanism.
//
// v0.1.0 placeholder: full adapter integration lands once src/skills/ is populated
// and the @ozzylabs/skills lib is extracted (handbook reviews/2026-04-30-gh-tasks-design.md §4.2).
import { existsSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = resolve(fileURLToPath(import.meta.url), '../..');
const SKILLS_SRC = resolve(ROOT, 'src/skills');

if (!existsSync(SKILLS_SRC)) {
  console.log('[build-skills] no src/skills/ yet — nothing to build');
  process.exit(0);
}

const skills = readdirSync(SKILLS_SRC, { withFileTypes: true })
  .filter((e) => e.isDirectory())
  .map((e) => e.name);

if (skills.length === 0) {
  console.log('[build-skills] no skills under src/skills/ — nothing to build');
  process.exit(0);
}

console.log(`[build-skills] found ${skills.length} skill(s): ${skills.join(', ')}`);
console.log('[build-skills] adapter integration is pending — see docs/adr/0001 backlog');
