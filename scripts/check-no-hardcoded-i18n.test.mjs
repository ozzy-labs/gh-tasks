// Tests for the hardcoded-non-ASCII-string-literal lint
// (scripts/check-no-hardcoded-i18n.mjs).
//
// Uses the built-in node:test runner (no extra dependency). Run with:
//   node --test scripts/check-no-hardcoded-i18n.test.mjs
// or via the package script:
//   pnpm run test:scripts

import { strict as assert } from 'node:assert';
import { test } from 'node:test';
import ts from 'typescript';
import { findViolations, hasNonAscii } from './check-no-hardcoded-i18n.mjs';

// ---------------------------------------------------------------------------
// hasNonAscii
// ---------------------------------------------------------------------------

test('hasNonAscii: ASCII-only text is allowed', () => {
  assert.equal(hasNonAscii(''), false);
  assert.equal(hasNonAscii('hello world'), false);
  assert.equal(hasNonAscii('Error: invalid value'), false);
  assert.equal(hasNonAscii('--scope flag requires a value'), false);
  assert.equal(hasNonAscii('123 + 456 = 579'), false);
});

test('hasNonAscii: CJK text is rejected', () => {
  assert.equal(hasNonAscii('日本語'), true);
  assert.equal(hasNonAscii('hello 世界'), true);
  assert.equal(hasNonAscii('エラー: 不正な値'), true);
  assert.equal(hasNonAscii('한국어'), true);
  assert.equal(hasNonAscii('中文'), true);
});

test('hasNonAscii: accented Latin (Latin-1 Supplement) is rejected', () => {
  assert.equal(hasNonAscii('café'), true);
  assert.equal(hasNonAscii('naïve'), true);
  assert.equal(hasNonAscii('Zürich'), true);
});

test('hasNonAscii: decorative arrows are allowed', () => {
  assert.equal(hasNonAscii('a → b'), false);
  assert.equal(hasNonAscii('a ← b'), false);
  assert.equal(hasNonAscii(' ↔ '), false);
  assert.equal(hasNonAscii('a ⇒ b ⇐ c'), false);
  assert.equal(hasNonAscii('a ⇔ b'), false);
});

test('hasNonAscii: decorative dashes and punctuation are allowed', () => {
  assert.equal(hasNonAscii('— em dash —'), false);
  assert.equal(hasNonAscii('en–dash'), false);
  assert.equal(hasNonAscii('ellipsis…'), false);
});

test('hasNonAscii: math operators are allowed', () => {
  assert.equal(hasNonAscii('∀x ∈ S'), false);
  assert.equal(hasNonAscii('a ≠ b'), false);
});

test('hasNonAscii: box-drawing and geometric shapes are allowed', () => {
  assert.equal(hasNonAscii('├── child'), false);
  assert.equal(hasNonAscii('└── leaf'), false);
  assert.equal(hasNonAscii('▶ arrow'), false);
});

test('hasNonAscii: mixed decorative + ja still triggers', () => {
  assert.equal(hasNonAscii('日本語 → translation'), true);
  assert.equal(hasNonAscii('a → b で日本語'), true);
});

// ---------------------------------------------------------------------------
// findViolations (TypeScript AST walk)
// ---------------------------------------------------------------------------

/** @param {string} code */
function parse(code) {
  return ts.createSourceFile('test.ts', code, ts.ScriptTarget.Latest, true);
}

test('findViolations: detects ja in single-quoted string literal', () => {
  const sf = parse(`const msg = 'こんにちは';`);
  const hits = findViolations(sf);
  assert.equal(hits.length, 1);
  assert.equal(hits[0].text, 'こんにちは');
});

test('findViolations: detects ja in double-quoted string literal', () => {
  const sf = parse(`const msg = "エラー";`);
  const hits = findViolations(sf);
  assert.equal(hits.length, 1);
  assert.equal(hits[0].text, 'エラー');
});

test('findViolations: detects ja in no-substitution template literal', () => {
  const sf = parse('const msg = `日本語`;');
  const hits = findViolations(sf);
  assert.equal(hits.length, 1);
  assert.equal(hits[0].text, '日本語');
});

test('findViolations: detects ja in template literal with substitution (head)', () => {
  const sf = parse('const msg = `日本語 ${value} 末尾`;');
  const hits = findViolations(sf);
  // Both the head ("日本語 ") and tail (" 末尾") contain ja; expect 2 hits.
  assert.equal(hits.length, 2);
});

test('findViolations: ASCII-only string literals are clean', () => {
  const sf = parse(`
    const a = 'hello';
    const b = "world";
    const c = \`template \${x}\`;
  `);
  const hits = findViolations(sf);
  assert.equal(hits.length, 0);
});

test('findViolations: ja in line and block comments is allowed', () => {
  const sf = parse(`
    // 日本語のコメント
    /* ブロックコメント */
    /**
     * JSDoc に書いた日本語
     */
    const x = 1;
  `);
  const hits = findViolations(sf);
  assert.equal(hits.length, 0);
});

test('findViolations: decorative chars in literals are allowed', () => {
  const sf = parse(`
    const arrow = ' → ';
    const sep = ' ↔ ';
    const dash = 'a — b';
  `);
  const hits = findViolations(sf);
  assert.equal(hits.length, 0);
});

test('findViolations: reports correct 1-indexed line and column', () => {
  const sf = parse(`const a = 1;
const b = 'こんにちは';`);
  const hits = findViolations(sf);
  assert.equal(hits.length, 1);
  assert.equal(hits[0].line, 2);
  // Column points at the start of the literal (the opening quote).
  assert.ok(hits[0].column > 0);
});

test('findViolations: detects ja interleaved with English in a literal', () => {
  const sf = parse(`const m = 'Error: 不正な値です';`);
  const hits = findViolations(sf);
  assert.equal(hits.length, 1);
  assert.equal(hits[0].text, 'Error: 不正な値です');
});

test('findViolations: empty file produces no hits', () => {
  const sf = parse('');
  const hits = findViolations(sf);
  assert.equal(hits.length, 0);
});

test('findViolations: object property values are walked', () => {
  const sf = parse(`const obj = { msg: 'こんにちは', code: 1 };`);
  const hits = findViolations(sf);
  assert.equal(hits.length, 1);
  assert.equal(hits[0].text, 'こんにちは');
});

test('findViolations: nested template literal substitution does not double-count', () => {
  const sf = parse('const m = `${`内側`}`;');
  const hits = findViolations(sf);
  assert.equal(hits.length, 1);
  assert.equal(hits[0].text, '内側');
});
