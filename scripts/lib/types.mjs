// Shared type definitions for the adapter pipeline.
//
// Adapters take a list of canonical Skill objects and return OutputFile[].
// File system writes are performed by the build orchestrator only — adapters
// are pure functions (handbook ADR-0018).

/**
 * A canonical skill loaded from `src/skills/{name}/SKILL.md`.
 *
 * Repo-internal extension over @ozzylabs/skills: per ADR-0004 each frontmatter
 * carries `description_en` (English mirror description) and `locale` (always
 * `ja` for the canonical SSOT) in addition to the upstream fields.
 *
 * @typedef {object} Skill
 * @property {string} name           Skill identifier (matches directory name).
 * @property {string} description    One-line description from frontmatter (ja SSOT).
 * @property {string} descriptionEn  English description (frontmatter `description_en`).
 * @property {string} locale         SSOT locale, always `ja` for this repo.
 * @property {Record<string, string>} frontmatter  Full parsed frontmatter map.
 * @property {string} body           SKILL.md content with frontmatter stripped.
 * @property {string} raw            Full SKILL.md content (frontmatter + body).
 */

/**
 * One file emitted by an adapter, relative to the adapter's dist root.
 *
 * @typedef {object} OutputFile
 * @property {string} relativePath   Path under `dist/{adapter}/`.
 * @property {string} content        File content (UTF-8).
 */

export {};
