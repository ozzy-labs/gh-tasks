#!/usr/bin/env node
// Detect hardcoded non-ASCII string literals in CLI source code.
//
// Per repo-internal ADR-0005, all user-facing messages MUST live in
// packages/gh-tasks/src/i18n/{en,ja}.json and be retrieved via `t()`.
// Non-ASCII characters in string literals (and template literals) almost
// always indicate a forgotten translation key. Comments are allowed to
// contain non-ASCII content.
//
// Usage:
//   node scripts/check-no-hardcoded-i18n.mjs
//
// Exits non-zero on first violation. Run from the repo root.

import { glob, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const SRC_GLOB = 'packages/gh-tasks/src/**/*.ts';
const EXCLUDES = [/\.test\.ts$/, /\/i18n\//];

/** @returns {Promise<string[]>} */
async function findFiles() {
  const out = [];
  for await (const entry of glob(SRC_GLOB, { cwd: ROOT })) {
    if (EXCLUDES.some((rx) => rx.test(entry))) continue;
    out.push(path.join(ROOT, entry));
  }
  return out.sort();
}

// Whitelist of decorative / structural Unicode characters that are not
// translatable text and may safely appear in source-code string literals
// (arrows, em-dashes, ellipses, math operators, bullets). Anything outside
// this whitelist that is also non-ASCII is flagged as a likely-untranslated
// message.
const DECORATIVE_NON_ASCII = /[‐-‧←-⇿∀-⋿─-╿■-◿☀-⛿]/g;

/** @param {string} text */
function hasNonAscii(text) {
  const stripped = text.replace(DECORATIVE_NON_ASCII, '');
  // Match anything outside the ASCII range (U+0000–U+007F) using a
  // codepoint comparison; avoids embedding control characters in the regex
  // literal (Biome's noControlCharactersInRegex).
  for (const ch of stripped) {
    if ((ch.codePointAt(0) ?? 0) > 0x7f) return true;
  }
  return false;
}

/**
 * @param {ts.SourceFile} sf
 * @returns {{ line: number; column: number; text: string }[]}
 */
function findViolations(sf) {
  const hits = [];
  /** @param {ts.Node} node */
  function visit(node) {
    if (
      ts.isStringLiteral(node) ||
      ts.isNoSubstitutionTemplateLiteral(node) ||
      ts.isTemplateHead(node) ||
      ts.isTemplateMiddle(node) ||
      ts.isTemplateTail(node)
    ) {
      const text = node.text;
      if (hasNonAscii(text)) {
        const start = node.getStart(sf);
        const { line, character } = sf.getLineAndCharacterOfPosition(start);
        hits.push({ line: line + 1, column: character + 1, text });
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sf);
  return hits;
}

async function main() {
  const files = await findFiles();
  let total = 0;
  for (const file of files) {
    const source = await readFile(file, 'utf8');
    const sf = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true);
    const hits = findViolations(sf);
    for (const hit of hits) {
      const rel = path.relative(ROOT, file);
      const preview = hit.text.length > 80 ? `${hit.text.slice(0, 77)}...` : hit.text;
      console.error(
        `${rel}:${hit.line}:${hit.column}  hardcoded non-ASCII literal  ${JSON.stringify(preview)}`
      );
    }
    total += hits.length;
  }
  if (total > 0) {
    console.error('');
    console.error(`${total} hardcoded non-ASCII literal(s) detected.`);
    console.error(
      'Move these strings to packages/gh-tasks/src/i18n/{en,ja}.json and retrieve via t() (repo-internal ADR-0005).'
    );
    process.exit(1);
  }
  console.log(`OK: scanned ${files.length} files, no hardcoded non-ASCII literals`);
}

await main();
