import type { AppConfig } from './config.ts';
import type { I18nArgs } from './github.ts';
import type { Scope } from './scope.ts';

export class ProjectError extends Error {
  readonly i18nKey: string;
  readonly i18nArgs: I18nArgs;
  constructor(i18nKey: string, args: I18nArgs = {}) {
    super(i18nKey);
    this.name = 'ProjectError';
    this.i18nKey = i18nKey;
    this.i18nArgs = args;
  }
}

/**
 * A Projects v2 reference: owner login + project number.
 *
 * The `<owner>/<number>` form mirrors GitHub's URL convention
 * (`/users/<owner>/projects/<number>` and `/orgs/<owner>/projects/<number>`).
 */
export interface ProjectRef {
  owner: string;
  number: number;
}

/**
 * Parse a project identifier in `<owner>/<number>` form. Returns null on
 * any malformed input — callers decide whether that's an error (flag
 * present but invalid) or a fall-through (config absent).
 */
export function parseProjectIdentifier(value: string | undefined): ProjectRef | null {
  if (typeof value !== 'string') return null;
  const trimmed = value.trim();
  if (trimmed.length === 0) return null;
  const slash = trimmed.indexOf('/');
  if (slash <= 0 || slash === trimmed.length - 1) return null;
  const owner = trimmed.slice(0, slash);
  const numberStr = trimmed.slice(slash + 1);
  const number = Number.parseInt(numberStr, 10);
  if (!Number.isFinite(number) || number <= 0 || String(number) !== numberStr) return null;
  return { owner, number };
}

/**
 * Pull `--project=<owner>/<number>` (or `--project <owner>/<number>`) out
 * of argv. Throws `ProjectError` when the flag is present but malformed —
 * silently falling through would surprise the user.
 */
export function parseProjectFlag(argv: readonly string[]): ProjectRef | null {
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;
    let value: string | undefined;
    if (arg.startsWith('--project=')) {
      value = arg.slice('--project='.length);
    } else if (arg === '--project') {
      value = argv[i + 1];
    } else {
      continue;
    }
    if (value === undefined) {
      throw new ProjectError('error.project.flagMissingValue');
    }
    const ref = parseProjectIdentifier(value);
    if (!ref) {
      throw new ProjectError('error.project.invalidIdentifier', { value });
    }
    return ref;
  }
  return null;
}

export interface ResolveProjectRefOptions {
  scope: Scope;
  argv: readonly string[];
  config?: AppConfig;
}

/**
 * Resolve the Projects v2 reference for the given scope.
 *
 * Order:
 *   1. `--project` flag
 *   2. `~/.config/ozzylabs/gh-tasks.toml` `org_project` / `user_project`
 *   3. throw `ProjectError` (callers should report the missing setting)
 *
 * Throws when called with `scope: 'repo'` — repo scope does not use
 * Projects v2.
 */
export function resolveProjectRef(opts: ResolveProjectRefOptions): ProjectRef {
  if (opts.scope === 'repo') {
    throw new ProjectError('error.project.repoScope');
  }
  const fromFlag = parseProjectFlag(opts.argv);
  if (fromFlag) return fromFlag;

  const fromConfig = opts.scope === 'org' ? opts.config?.orgProject : opts.config?.userProject;
  if (fromConfig) return fromConfig;

  const configKey = opts.scope === 'org' ? 'org_project' : 'user_project';
  throw new ProjectError('error.project.notSpecified', { scope: opts.scope, configKey });
}
