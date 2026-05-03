import { describe, expect, it } from 'vitest';
import {
  ProjectError,
  parseProjectFlag,
  parseProjectIdentifier,
  resolveProjectRef,
} from './project.ts';

describe('parseProjectIdentifier', () => {
  it('parses owner/number', () => {
    expect(parseProjectIdentifier('ozzy-labs/5')).toEqual({ owner: 'ozzy-labs', number: 5 });
  });

  it('returns null for malformed inputs', () => {
    expect(parseProjectIdentifier(undefined)).toBeNull();
    expect(parseProjectIdentifier('')).toBeNull();
    expect(parseProjectIdentifier('ozzy-labs')).toBeNull();
    expect(parseProjectIdentifier('/5')).toBeNull();
    expect(parseProjectIdentifier('ozzy-labs/')).toBeNull();
    expect(parseProjectIdentifier('ozzy-labs/abc')).toBeNull();
    expect(parseProjectIdentifier('ozzy-labs/0')).toBeNull();
    expect(parseProjectIdentifier('ozzy-labs/-1')).toBeNull();
    expect(parseProjectIdentifier('ozzy-labs/3.5')).toBeNull();
  });

  it('trims surrounding whitespace', () => {
    expect(parseProjectIdentifier('  ozzy-labs/5  ')).toEqual({ owner: 'ozzy-labs', number: 5 });
  });
});

describe('parseProjectFlag', () => {
  it('returns null when no --project flag is present', () => {
    expect(parseProjectFlag([])).toBeNull();
    expect(parseProjectFlag(['add', 'foo'])).toBeNull();
  });

  it('parses --project=<owner>/<number>', () => {
    expect(parseProjectFlag(['--project=ozzy-labs/5'])).toEqual({
      owner: 'ozzy-labs',
      number: 5,
    });
  });

  it('parses --project <owner>/<number> (separate arg)', () => {
    expect(parseProjectFlag(['--project', 'ozzy-labs/5'])).toEqual({
      owner: 'ozzy-labs',
      number: 5,
    });
  });

  it('throws on missing value', () => {
    expect(() => parseProjectFlag(['--project'])).toThrow(ProjectError);
  });

  it('throws on malformed value', () => {
    expect(() => parseProjectFlag(['--project=bogus'])).toThrow(ProjectError);
    expect(() => parseProjectFlag(['--project=ozzy-labs/0'])).toThrow(ProjectError);
  });
});

describe('resolveProjectRef', () => {
  it('returns the --project flag value when present', () => {
    expect(resolveProjectRef({ scope: 'org', argv: ['--project=ozzy-labs/5'] })).toEqual({
      owner: 'ozzy-labs',
      number: 5,
    });
  });

  it('falls back to config.orgProject for org scope', () => {
    expect(
      resolveProjectRef({
        scope: 'org',
        argv: [],
        config: { orgProject: { owner: 'ozzy-labs', number: 7 } },
      })
    ).toEqual({ owner: 'ozzy-labs', number: 7 });
  });

  it('falls back to config.userProject for user scope', () => {
    expect(
      resolveProjectRef({
        scope: 'user',
        argv: [],
        config: { userProject: { owner: 'ozzy-3', number: 2 } },
      })
    ).toEqual({ owner: 'ozzy-3', number: 2 });
  });

  it('flag outranks config', () => {
    expect(
      resolveProjectRef({
        scope: 'org',
        argv: ['--project=ozzy-labs/5'],
        config: { orgProject: { owner: 'other', number: 99 } },
      })
    ).toEqual({ owner: 'ozzy-labs', number: 5 });
  });

  it('does not cross-pollinate org / user config keys', () => {
    expect(() =>
      resolveProjectRef({
        scope: 'user',
        argv: [],
        config: { orgProject: { owner: 'ozzy-labs', number: 5 } },
      })
    ).toThrow(ProjectError);
    expect(() =>
      resolveProjectRef({
        scope: 'org',
        argv: [],
        config: { userProject: { owner: 'ozzy-3', number: 2 } },
      })
    ).toThrow(ProjectError);
  });

  it('throws when neither flag nor config is set', () => {
    expect(() => resolveProjectRef({ scope: 'org', argv: [] })).toThrow(ProjectError);
    expect(() => resolveProjectRef({ scope: 'user', argv: [] })).toThrow(ProjectError);
  });

  it('throws when called with repo scope', () => {
    expect(() => resolveProjectRef({ scope: 'repo', argv: ['--project=ozzy-labs/5'] })).toThrow(
      ProjectError
    );
  });
});
