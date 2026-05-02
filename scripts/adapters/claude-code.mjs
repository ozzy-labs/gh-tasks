// Claude Code adapter.
//
// Claude Code reads `.claude/skills/{name}/SKILL.md` from the consumer repo.
// This adapter emits the canonical (ja SSOT) SKILL.md as-is. The English
// mirror (`SKILL.en.md`) is not currently distributed via this adapter —
// Claude Code consumers running with `LANG=en` see the description_en field
// at skill discovery time but the body remains ja for now. Distribution of
// en mirror as a separate locale subtree is left to a follow-up PR.

import { AdapterBase } from '../lib/adapter-base.mjs';

/**
 * @typedef {import('../lib/types.mjs').Skill} Skill
 * @typedef {import('../lib/types.mjs').OutputFile} OutputFile
 */

export class ClaudeCodeAdapter extends AdapterBase {
  static id = 'claude-code';

  /**
   * @param {Skill[]} skills
   * @returns {OutputFile[]}
   */
  generate(skills) {
    const sorted = [...skills].sort((a, b) => a.name.localeCompare(b.name));
    return sorted.map((skill) => ({
      relativePath: `.claude/skills/${skill.name}/SKILL.md`,
      content: skill.raw,
    }));
  }
}
