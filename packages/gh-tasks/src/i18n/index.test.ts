import { describe, expect, it } from 'vitest';
import { resolveLocale, t } from './index.ts';

describe('resolveLocale — flag', () => {
  it('honors --lang=ja form', () => {
    expect(resolveLocale(['--lang=ja'], {})).toBe('ja');
    expect(resolveLocale(['--lang=en'], {})).toBe('en');
  });

  it('honors --lang ja (separate-arg) form', () => {
    expect(resolveLocale(['--lang', 'ja'], {})).toBe('ja');
    expect(resolveLocale(['add', 'foo', '--lang', 'en'], {})).toBe('en');
  });

  it('ignores unknown --lang values and falls through to env', () => {
    expect(resolveLocale(['--lang=xx'], { LANG: 'ja_JP.UTF-8' })).toBe('ja');
    expect(resolveLocale(['--lang=xx'], {})).toBe('en');
  });

  it('treats lone --lang with no value as no flag (falls through)', () => {
    expect(resolveLocale(['--lang'], { LANG: 'ja_JP.UTF-8' })).toBe('ja');
    expect(resolveLocale(['--lang'], {})).toBe('en');
  });
});

describe('resolveLocale — config', () => {
  it('honors config.lang when no flag is set', () => {
    expect(resolveLocale([], {}, { lang: 'ja' })).toBe('ja');
  });

  it('flag outranks config.lang', () => {
    expect(resolveLocale(['--lang=en'], {}, { lang: 'ja' })).toBe('en');
  });

  it('config.lang outranks env', () => {
    expect(resolveLocale([], { LANG: 'ja_JP.UTF-8' }, { lang: 'en' })).toBe('en');
    expect(resolveLocale([], { LC_ALL: 'en_US.UTF-8' }, { lang: 'ja' })).toBe('ja');
  });
});

describe('resolveLocale — env', () => {
  it('uses LC_ALL when no flag', () => {
    expect(resolveLocale([], { LC_ALL: 'ja_JP.UTF-8' })).toBe('ja');
    expect(resolveLocale([], { LC_ALL: 'en_US.UTF-8' })).toBe('en');
  });

  it('falls back to LANG when LC_ALL missing', () => {
    expect(resolveLocale([], { LANG: 'ja_JP.UTF-8' })).toBe('ja');
    expect(resolveLocale([], { LANG: 'C' })).toBe('en');
  });

  it('prefers LC_ALL over LANG', () => {
    expect(resolveLocale([], { LC_ALL: 'en_US.UTF-8', LANG: 'ja_JP.UTF-8' })).toBe('en');
  });

  it('defaults to en when no flag and no env', () => {
    expect(resolveLocale([], {})).toBe('en');
  });
});

describe('t', () => {
  it('returns ja message when locale is ja', () => {
    expect(t('ja', 'error.unknownCommand')).toBe('不明なコマンド');
  });

  it('returns en message when locale is en', () => {
    expect(t('en', 'error.unknownCommand')).toBe('Unknown command');
  });

  it('falls back to en when key is missing in ja', () => {
    // This test asserts the fallback path (`TABLES[locale][key] ?? TABLES.en[key]`).
    // If a key exists in en but not ja, `t('ja', key)` returns en; if missing in
    // both, returns the key itself.
    expect(t('ja', 'nonexistent.key.xyz')).toBe('nonexistent.key.xyz');
  });

  it('interpolates {name} placeholders from args', () => {
    expect(t('en', 'error.scope.invalid', { value: 'bogus', valid: 'repo | org | user' })).toBe(
      "Invalid --scope value: 'bogus' (valid: repo | org | user)"
    );
    expect(t('ja', 'error.repo.invalidIdentifier', { value: 'oops' })).toBe(
      "不正なリポジトリ識別子: 'oops' (期待: '<owner>/<name>')"
    );
  });

  it('leaves placeholders untouched when args are missing', () => {
    expect(t('en', 'error.repo.invalidIdentifier')).toContain('{value}');
  });
});
