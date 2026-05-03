import { execFileSync } from 'node:child_process';

export type Scope = 'repo' | 'org' | 'user';

const VALID: readonly Scope[] = ['repo', 'org', 'user'];

export class ScopeError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ScopeError';
  }
}

export interface DetectScopeOptions {
  argv: readonly string[];
  hasGitRemote?: () => boolean;
  config?: { defaultScope?: Scope };
}

/**
 * Resolve the working scope.
 *
 * Order:
 *   1. `--scope repo|org|user` flag
 *   2. git remote `origin` exists → `repo`
 *   3. `~/.config/ozzylabs/gh-tasks.toml` `default_scope` (passed via `config`)
 *   4. fallback → `user`
 */
export function detectScope(opts: DetectScopeOptions): Scope {
  const fromFlag = parseScopeFlag(opts.argv);
  if (fromFlag) return fromFlag;

  const detector = opts.hasGitRemote ?? defaultHasGitRemote;
  if (detector()) return 'repo';

  if (opts.config?.defaultScope) return opts.config.defaultScope;

  return 'user';
}

export function parseScopeFlag(argv: readonly string[]): Scope | null {
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;

    if (arg.startsWith('--scope=')) {
      const value = arg.slice('--scope='.length);
      return assertScope(value);
    }
    if (arg === '--scope') {
      const next = argv[i + 1];
      if (next === undefined) {
        throw new ScopeError('--scope フラグに値が指定されていません');
      }
      return assertScope(next);
    }
  }
  return null;
}

function assertScope(value: string): Scope {
  if ((VALID as readonly string[]).includes(value)) {
    return value as Scope;
  }
  throw new ScopeError(`不正な --scope 値: '${value}' (有効値: ${VALID.join(' | ')})`);
}

function defaultHasGitRemote(): boolean {
  try {
    execFileSync('git', ['remote', 'get-url', 'origin'], { stdio: 'pipe' });
    return true;
  } catch {
    return false;
  }
}
