// Gemini CLI adapter.
//
// Gemini CLI reads `.gemini/settings.json` and follows `context.fileName`
// to the consumer's AGENTS.md, where the skill list lives. This adapter
// emits the same `AGENTS.md.snippet` body the Codex CLI adapter produces
// (both adapters share scripts/lib/agents-md-snippet.mjs). The
// `.gemini/settings.json` itself is upstream-managed by `@ozzylabs/skills`,
// so this adapter does not emit it.
//
// Reference: https://github.com/google-gemini/gemini-cli

import { AdapterBase } from '../lib/adapter-base.mjs';
import { renderAgentsMdSnippet } from '../lib/agents-md-snippet.mjs';

/**
 * @typedef {import('../lib/types.mjs').Skill} Skill
 * @typedef {import('../lib/types.mjs').OutputFile} OutputFile
 */

export class GeminiCliAdapter extends AdapterBase {
  static id = 'gemini-cli';

  /**
   * @param {Skill[]} skills
   * @returns {OutputFile[]}
   */
  generate(skills) {
    const sorted = [...skills].sort((a, b) => a.name.localeCompare(b.name));
    return [
      {
        relativePath: 'AGENTS.md.snippet',
        content: renderAgentsMdSnippet(sorted, 'ja'),
      },
    ];
  }
}
