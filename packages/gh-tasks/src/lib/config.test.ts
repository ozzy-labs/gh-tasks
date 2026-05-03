import { describe, expect, it } from 'vitest';
import { ConfigError, loadConfig, resolveConfigPath } from './config.ts';

function makeReader(map: Record<string, string>): (path: string) => string {
  return (path: string) => {
    if (path in map) return map[path] as string;
    const err: NodeJS.ErrnoException = Object.assign(new Error('ENOENT'), { code: 'ENOENT' });
    throw err;
  };
}

describe('resolveConfigPath', () => {
  it('honors XDG_CONFIG_HOME when set', () => {
    expect(resolveConfigPath({ XDG_CONFIG_HOME: '/tmp/xdg' })).toBe(
      '/tmp/xdg/ozzylabs/gh-tasks.toml'
    );
  });

  it('falls back to ~/.config when XDG_CONFIG_HOME is unset', () => {
    const path = resolveConfigPath({});
    expect(path.endsWith('/.config/ozzylabs/gh-tasks.toml')).toBe(true);
  });

  it('treats empty XDG_CONFIG_HOME as unset', () => {
    const path = resolveConfigPath({ XDG_CONFIG_HOME: '' });
    expect(path.endsWith('/.config/ozzylabs/gh-tasks.toml')).toBe(true);
  });
});

describe('loadConfig', () => {
  const PATH = '/tmp/xdg/ozzylabs/gh-tasks.toml';
  const env = { XDG_CONFIG_HOME: '/tmp/xdg' };

  it('returns empty object when file is missing', () => {
    expect(loadConfig({ env, readFile: makeReader({}) })).toEqual({});
  });

  it('parses lang and default_scope', () => {
    const cfg = loadConfig({
      env,
      readFile: makeReader({ [PATH]: 'lang = "ja"\ndefault_scope = "user"\n' }),
    });
    expect(cfg).toEqual({ lang: 'ja', defaultScope: 'user' });
  });

  it('returns partial config when only lang is set', () => {
    const cfg = loadConfig({
      env,
      readFile: makeReader({ [PATH]: 'lang = "en"\n' }),
    });
    expect(cfg).toEqual({ lang: 'en' });
  });

  it('throws ConfigError on invalid lang value', () => {
    expect(() => loadConfig({ env, readFile: makeReader({ [PATH]: 'lang = "xx"\n' }) })).toThrow(
      ConfigError
    );
  });

  it('throws ConfigError on invalid default_scope value', () => {
    expect(() =>
      loadConfig({ env, readFile: makeReader({ [PATH]: 'default_scope = "global"\n' }) })
    ).toThrow(ConfigError);
  });

  it('throws ConfigError on malformed TOML', () => {
    expect(() => loadConfig({ env, readFile: makeReader({ [PATH]: 'lang =\n' }) })).toThrow(
      ConfigError
    );
  });

  it('ignores unknown keys (forward compatible)', () => {
    const cfg = loadConfig({
      env,
      readFile: makeReader({ [PATH]: 'lang = "ja"\nfuture_key = "something"\n' }),
    });
    expect(cfg).toEqual({ lang: 'ja' });
  });

  it('respects an explicit path override', () => {
    const cfg = loadConfig({
      env,
      path: '/custom/gh-tasks.toml',
      readFile: makeReader({ '/custom/gh-tasks.toml': 'lang = "ja"\n' }),
    });
    expect(cfg).toEqual({ lang: 'ja' });
  });

  it('parses org_project and user_project as ProjectRef', () => {
    const cfg = loadConfig({
      env,
      readFile: makeReader({
        [PATH]: 'org_project = "ozzy-labs/5"\nuser_project = "ozzy-3/2"\n',
      }),
    });
    expect(cfg).toEqual({
      orgProject: { owner: 'ozzy-labs', number: 5 },
      userProject: { owner: 'ozzy-3', number: 2 },
    });
  });

  it('throws ConfigError on malformed org_project', () => {
    expect(() =>
      loadConfig({ env, readFile: makeReader({ [PATH]: 'org_project = "ozzy-labs"\n' }) })
    ).toThrow(ConfigError);
  });

  it('throws ConfigError on non-string user_project', () => {
    expect(() =>
      loadConfig({ env, readFile: makeReader({ [PATH]: 'user_project = 5\n' }) })
    ).toThrow(ConfigError);
  });
});
