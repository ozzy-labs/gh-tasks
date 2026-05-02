import { describe, expect, it } from 'vitest';
import {
  extractFromRemote,
  parseOwnerName,
  parseRepoFlag,
  RepoError,
  resolveRepo,
} from './repo.ts';

describe('parseOwnerName', () => {
  it('parses owner/name', () => {
    expect(parseOwnerName('ozzy-labs/gh-tasks')).toEqual({
      owner: 'ozzy-labs',
      name: 'gh-tasks',
    });
  });

  it('rejects malformed values', () => {
    expect(() => parseOwnerName('gh-tasks')).toThrow(RepoError);
    expect(() => parseOwnerName('a/b/c')).toThrow(RepoError);
    expect(() => parseOwnerName('')).toThrow(RepoError);
  });
});

describe('extractFromRemote', () => {
  it('parses SSH form', () => {
    expect(extractFromRemote('git@github.com:ozzy-labs/gh-tasks.git')).toBe('ozzy-labs/gh-tasks');
    expect(extractFromRemote('git@github.com:ozzy-labs/gh-tasks')).toBe('ozzy-labs/gh-tasks');
  });

  it('parses HTTPS form', () => {
    expect(extractFromRemote('https://github.com/ozzy-labs/gh-tasks.git')).toBe(
      'ozzy-labs/gh-tasks'
    );
    expect(extractFromRemote('https://github.com/ozzy-labs/gh-tasks')).toBe('ozzy-labs/gh-tasks');
    expect(extractFromRemote('https://github.com/ozzy-labs/gh-tasks/')).toBe('ozzy-labs/gh-tasks');
  });

  it('rejects unparseable URLs', () => {
    expect(() => extractFromRemote('')).toThrow(RepoError);
    expect(() => extractFromRemote('not-a-url')).toThrow(RepoError);
  });
});

describe('parseRepoFlag', () => {
  it('returns null when --repo is absent', () => {
    expect(parseRepoFlag([])).toBeNull();
    expect(parseRepoFlag(['--scope=repo'])).toBeNull();
  });

  it('parses --repo=value form', () => {
    expect(parseRepoFlag(['--repo=ozzy-labs/gh-tasks'])).toBe('ozzy-labs/gh-tasks');
  });

  it('parses --repo value form', () => {
    expect(parseRepoFlag(['add', 'title', '--repo', 'ozzy-labs/gh-tasks'])).toBe(
      'ozzy-labs/gh-tasks'
    );
  });

  it('throws when --repo has no value', () => {
    expect(() => parseRepoFlag(['--repo'])).toThrow(RepoError);
  });
});

describe('resolveRepo', () => {
  it('uses --repo flag when present', () => {
    expect(resolveRepo({ argv: ['--repo=ozzy-labs/gh-tasks'], getRemoteUrl: () => null })).toEqual({
      owner: 'ozzy-labs',
      name: 'gh-tasks',
    });
  });

  it('falls back to git remote when --repo absent', () => {
    expect(
      resolveRepo({
        argv: [],
        getRemoteUrl: () => 'git@github.com:ozzy-labs/gh-tasks.git',
      })
    ).toEqual({ owner: 'ozzy-labs', name: 'gh-tasks' });
  });

  it('throws when neither --repo nor git remote yields a value', () => {
    expect(() => resolveRepo({ argv: [], getRemoteUrl: () => null })).toThrow(RepoError);
  });
});
