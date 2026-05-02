// Shared `AGENTS.md` snippet generator.
//
// Codex CLI and Gemini CLI both read `AGENTS.md`. Both adapters emit the
// same skill-list snippet, so the rendering lives here.
//
// Output is wrapped with `<!-- begin/end: @ozzylabs/gh-tasks -->` markers
// (repo-local — see scripts/lib/snippet.mjs) so consumer-side sync scripts
// replace just the gh-tasks managed region of an otherwise hand-edited
// AGENTS.md, leaving the upstream `@ozzylabs/skills` block untouched.

import { wrapSnippet } from './snippet.mjs';

/**
 * @typedef {import('./types.mjs').Skill} Skill
 */

/**
 * Render the `AGENTS.md` skill-list snippet for a given locale.
 *
 * @param {Skill[]} skills  Skills are sorted by name internally for determinism.
 * @param {'ja' | 'en'} locale
 * @returns {string}        Snippet body wrapped with begin/end markers.
 */
export function renderAgentsMdSnippet(skills, locale) {
  const sorted = [...skills].sort((a, b) => a.name.localeCompare(b.name));
  const lines = ['## gh-tasks Skills', ''];
  for (const skill of sorted) {
    const desc = locale === 'en' ? skill.descriptionEn : skill.description;
    lines.push(`- \`${skill.name}\` — ${desc}`);
  }
  return wrapSnippet(lines.join('\n'));
}
