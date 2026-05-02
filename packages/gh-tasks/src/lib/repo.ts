import { execFileSync } from 'node:child_process';

export interface RepoIdent {
  owner: string;
  name: string;
}

export class RepoError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'RepoError';
  }
}

export interface ResolveRepoOptions {
  argv: readonly string[];
  getRemoteUrl?: () => string | null;
}

/**
 * Resolve `<owner>/<name>` from `--repo` flag or git remote `origin`.
 *
 * Order:
 *   1. `--repo <owner>/<name>` / `--repo=<owner>/<name>` flag
 *   2. `git remote get-url origin` (SSH or HTTPS)
 *
 * Throws `RepoError` when neither yields a valid identifier.
 */
export function resolveRepo(opts: ResolveRepoOptions): RepoIdent {
  const flag = parseRepoFlag(opts.argv);
  if (flag) return parseOwnerName(flag);

  const getter = opts.getRemoteUrl ?? defaultGetRemoteUrl;
  const remote = getter();
  if (!remote) {
    throw new RepoError('--repo フラグも git remote origin も解決できません');
  }
  return parseOwnerName(extractFromRemote(remote));
}

export function parseRepoFlag(argv: readonly string[]): string | null {
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;
    if (arg.startsWith('--repo=')) {
      return arg.slice('--repo='.length);
    }
    if (arg === '--repo') {
      const next = argv[i + 1];
      if (next === undefined) {
        throw new RepoError('--repo フラグに値が指定されていません');
      }
      return next;
    }
  }
  return null;
}

export function parseOwnerName(value: string): RepoIdent {
  const match = value.match(/^([^/]+)\/([^/]+)$/);
  if (!match) {
    throw new RepoError(`不正なリポジトリ識別子: '${value}' (期待: '<owner>/<name>')`);
  }
  return { owner: match[1] as string, name: match[2] as string };
}

/**
 * Extract `<owner>/<name>` from a git remote URL.
 *
 * Supported forms:
 *   - SSH:   `git@github.com:owner/name.git`
 *   - HTTPS: `https://github.com/owner/name.git`
 *   - Trailing `.git` is optional.
 */
export function extractFromRemote(url: string): string {
  const match = url.trim().match(/[:/]([^/:]+)\/([^/]+?)(?:\.git)?\/?$/);
  if (!match) {
    throw new RepoError(`git remote URL から owner/name を抽出できません: '${url}'`);
  }
  return `${match[1]}/${match[2]}`;
}

function defaultGetRemoteUrl(): string | null {
  try {
    return execFileSync('git', ['remote', 'get-url', 'origin'], { stdio: 'pipe' })
      .toString()
      .trim();
  } catch {
    return null;
  }
}
