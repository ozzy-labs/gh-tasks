import { describe, expect, it } from 'vitest';
import { detectScope, parseScopeFlag, ScopeError } from './scope.ts';

describe('parseScopeFlag', () => {
  it('returns null when no --scope flag is present', () => {
    expect(parseScopeFlag([])).toBeNull();
    expect(parseScopeFlag(['add', 'foo'])).toBeNull();
  });

  it('parses --scope=value form', () => {
    expect(parseScopeFlag(['--scope=repo'])).toBe('repo');
    expect(parseScopeFlag(['--scope=org'])).toBe('org');
    expect(parseScopeFlag(['--scope=user'])).toBe('user');
  });

  it('parses --scope value form (next argv)', () => {
    expect(parseScopeFlag(['--scope', 'repo'])).toBe('repo');
    expect(parseScopeFlag(['add', 'title', '--scope', 'org'])).toBe('org');
  });

  it('throws on unknown scope value', () => {
    expect(() => parseScopeFlag(['--scope=bogus'])).toThrow(ScopeError);
    expect(() => parseScopeFlag(['--scope', 'global'])).toThrow(ScopeError);
  });

  it('throws when --scope has no value', () => {
    expect(() => parseScopeFlag(['--scope'])).toThrow(ScopeError);
  });
});

describe('detectScope', () => {
  it('honors --scope flag over git remote detection', () => {
    expect(detectScope({ argv: ['--scope=user'], hasGitRemote: () => true })).toBe('user');
  });

  it('falls back to repo when git remote exists', () => {
    expect(detectScope({ argv: [], hasGitRemote: () => true })).toBe('repo');
  });

  it('falls back to user when no git remote', () => {
    expect(detectScope({ argv: [], hasGitRemote: () => false })).toBe('user');
  });

  it('honors config.defaultScope when no flag and no git remote', () => {
    expect(
      detectScope({ argv: [], hasGitRemote: () => false, config: { defaultScope: 'org' } })
    ).toBe('org');
  });

  it('flag outranks config.defaultScope', () => {
    expect(
      detectScope({
        argv: ['--scope=user'],
        hasGitRemote: () => false,
        config: { defaultScope: 'org' },
      })
    ).toBe('user');
  });

  it('git remote outranks config.defaultScope', () => {
    expect(
      detectScope({ argv: [], hasGitRemote: () => true, config: { defaultScope: 'user' } })
    ).toBe('repo');
  });
});
