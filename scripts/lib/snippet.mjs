// `.snippet` marker helpers.
//
// Adapters that emit content meant to be merged into a consumer-owned file
// (AGENTS.md, copilot-instructions.md) wrap their payload with begin/end
// markers so consumer-side sync scripts can replace just the managed region.
//
// Vendored from @ozzylabs/skills/scripts/lib/snippet.mjs.
// The marker tag is repo-local (`@ozzylabs/gh-tasks`) so the gh-tasks skill
// list is replaced independently of the upstream `@ozzylabs/skills` block in
// the same consumer file.

const MARKER_TAG = '@ozzylabs/gh-tasks';

export const SNIPPET_BEGIN = `<!-- begin: ${MARKER_TAG} -->`;
export const SNIPPET_END = `<!-- end: ${MARKER_TAG} -->`;

/**
 * Wrap a body string with begin/end markers. One blank line is inserted on
 * each side of the body so the output is idempotent under Prettier's Markdown
 * formatter (which inserts blank lines around HTML-block comments).
 *
 * @param {string} body
 * @returns {string}
 */
export function wrapSnippet(body) {
  const trimmed = body.replace(/\n+$/, '');
  return `${SNIPPET_BEGIN}\n\n${trimmed}\n\n${SNIPPET_END}\n`;
}
