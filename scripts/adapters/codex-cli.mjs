// Codex CLI adapter.
//
// Codex CLI reads `AGENTS.md` and resolves skill references to
// `.agents/skills/{name}/SKILL.md`. This adapter emits both: the canonical
// SKILL.md files under `.agents/skills/` and an `AGENTS.md.snippet` that
// consumer-side sync scripts merge into the repo's hand-edited AGENTS.md.

import { AdapterBase } from '../lib/adapter-base.mjs';
import { renderAgentsMdSnippet } from '../lib/agents-md-snippet.mjs';

/**
 * @typedef {import('../lib/types.mjs').Skill} Skill
 * @typedef {import('../lib/types.mjs').OutputFile} OutputFile
 */

export class CodexCliAdapter extends AdapterBase {
  static id = 'codex-cli';

  /**
   * @param {Skill[]} skills
   * @returns {OutputFile[]}
   */
  generate(skills) {
    const sorted = [...skills].sort((a, b) => a.name.localeCompare(b.name));
    const outputs = sorted.map((skill) => ({
      relativePath: `.agents/skills/${skill.name}/SKILL.md`,
      content: skill.raw,
    }));
    outputs.push({
      relativePath: 'AGENTS.md.snippet',
      content: renderAgentsMdSnippet(sorted, 'ja'),
    });
    return outputs;
  }
}
