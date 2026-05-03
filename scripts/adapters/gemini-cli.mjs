// Gemini CLI adapter.
//
// Gemini CLI reads `.gemini/settings.json` and follows `context.fileName`
// to the consumer's AGENTS.md, where the skill list lives. This adapter
// emits both:
//   - `.gemini/settings.json` — the canonical context.fileName pointer
//   - `AGENTS.md.snippet` — the skill list inserted into the consumer's
//     AGENTS.md (shares scripts/lib/agents-md-snippet.mjs with codex-cli)
//
// The settings.json content is identical to what `@ozzylabs/skills` emits;
// both bundles' sync runs are idempotent against this file.
//
// Reference: https://github.com/google-gemini/gemini-cli

import { AdapterBase } from '../lib/adapter-base.mjs';
import { renderAgentsMdSnippet } from '../lib/agents-md-snippet.mjs';

/**
 * @typedef {import('../lib/types.mjs').Skill} Skill
 * @typedef {import('../lib/types.mjs').OutputFile} OutputFile
 */

const GEMINI_SETTINGS = `${JSON.stringify({ context: { fileName: ['AGENTS.md'] } }, null, 2)}\n`;

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
        relativePath: '.gemini/settings.json',
        content: GEMINI_SETTINGS,
      },
      {
        relativePath: 'AGENTS.md.snippet',
        content: renderAgentsMdSnippet(sorted, 'ja'),
      },
    ];
  }
}
