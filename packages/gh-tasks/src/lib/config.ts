import { readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { parse as parseToml, TomlError } from 'smol-toml';

import type { Locale } from '../i18n/index.ts';
import type { I18nArgs } from './github.ts';
import { type ProjectRef, parseProjectIdentifier } from './project.ts';
import type { Scope } from './scope.ts';

export class ConfigError extends Error {
  readonly i18nKey: string;
  readonly i18nArgs: I18nArgs;
  constructor(i18nKey: string, args: I18nArgs = {}) {
    super(i18nKey);
    this.name = 'ConfigError';
    this.i18nKey = i18nKey;
    this.i18nArgs = args;
  }
}

export interface AppConfig {
  lang?: Locale;
  defaultScope?: Scope;
  orgProject?: ProjectRef;
  userProject?: ProjectRef;
}

const VALID_LANG: readonly Locale[] = ['ja', 'en'];
const VALID_SCOPE: readonly Scope[] = ['repo', 'org', 'user'];

export interface LoadConfigOptions {
  env?: NodeJS.ProcessEnv;
  /** Override the resolved file path. Used by tests. */
  path?: string;
  /** Override the file reader. Used by tests. */
  readFile?: (path: string) => string;
}

/**
 * Resolve the config path per XDG Base Directory:
 *   `${XDG_CONFIG_HOME}/ozzylabs/gh-tasks.toml`, falling back to
 *   `${HOME}/.config/ozzylabs/gh-tasks.toml`.
 *
 * The trailing path segment matches the CLI's documented config (see
 * README and `docs/{ja,en}/installation.md`).
 */
export function resolveConfigPath(env: NodeJS.ProcessEnv = process.env): string {
  const xdg = env.XDG_CONFIG_HOME;
  const base = xdg && xdg.length > 0 ? xdg : join(homedir(), '.config');
  return join(base, 'ozzylabs', 'gh-tasks.toml');
}

/**
 * Load the persistent config. Returns an empty object when the file is
 * absent (the file is optional). Throws `ConfigError` only when the file
 * exists but is malformed or holds invalid values, so the user sees a
 * specific message instead of a silent fallback to defaults.
 */
export function loadConfig(options: LoadConfigOptions = {}): AppConfig {
  const env = options.env ?? process.env;
  const path = options.path ?? resolveConfigPath(env);
  const reader = options.readFile ?? defaultReadFile;

  let raw: string | null;
  try {
    raw = reader(path);
  } catch (err) {
    if (isFileNotFound(err)) return {};
    throw new ConfigError('error.config.readFailed', { path, reason: describeError(err) });
  }
  if (raw === null) return {};

  let parsed: Record<string, unknown>;
  try {
    parsed = parseToml(raw) as Record<string, unknown>;
  } catch (err) {
    if (err instanceof TomlError) {
      throw new ConfigError('error.config.tomlParseFailed', { path, reason: err.message });
    }
    throw err;
  }

  const config: AppConfig = {};
  if ('lang' in parsed) {
    const value = parsed.lang;
    if (typeof value !== 'string' || !(VALID_LANG as readonly string[]).includes(value)) {
      throw new ConfigError('error.config.invalidLang', {
        path,
        value: String(value),
        valid: VALID_LANG.join(' | '),
      });
    }
    config.lang = value as Locale;
  }
  if ('default_scope' in parsed) {
    const value = parsed.default_scope;
    if (typeof value !== 'string' || !(VALID_SCOPE as readonly string[]).includes(value)) {
      throw new ConfigError('error.config.invalidDefaultScope', {
        path,
        value: String(value),
        valid: VALID_SCOPE.join(' | '),
      });
    }
    config.defaultScope = value as Scope;
  }
  if ('org_project' in parsed) {
    config.orgProject = parseProjectKey(parsed.org_project, 'org_project', path);
  }
  if ('user_project' in parsed) {
    config.userProject = parseProjectKey(parsed.user_project, 'user_project', path);
  }
  return config;
}

function parseProjectKey(value: unknown, key: string, path: string): ProjectRef {
  if (typeof value !== 'string') {
    throw new ConfigError('error.config.invalidProjectRef', {
      key,
      path,
      value: String(value),
    });
  }
  const ref = parseProjectIdentifier(value);
  if (!ref) {
    throw new ConfigError('error.config.invalidProjectRef', { key, path, value });
  }
  return ref;
}

function defaultReadFile(path: string): string | null {
  try {
    return readFileSync(path, 'utf8');
  } catch (err) {
    if (isFileNotFound(err)) return null;
    throw err;
  }
}

function isFileNotFound(err: unknown): boolean {
  return err instanceof Error && 'code' in err && (err as { code?: string }).code === 'ENOENT';
}

function describeError(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
