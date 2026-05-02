import { resolveLocale, t } from '../i18n/index.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import {
  GET_PULL_REQUEST_BY_NUMBER,
  type GetPullRequestByNumberResponse,
  UPDATE_PULL_REQUEST,
  type UpdatePullRequestResponse,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope } from '../lib/scope.ts';

export interface LinkCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
}

export async function link(argv: readonly string[], deps: LinkCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv);

  const positionals = parsePositionalNumbers(argv);
  if (positionals.length < 2) {
    stderr.write(`${t(locale, 'error.link.argsRequired')}\n`);
    return 2;
  }
  const [pr, task] = positionals as [number, number, ...number[]];

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote });
  if (scope !== 'repo') {
    stderr.write(`${t(locale, 'error.scope.notImplemented')}: --scope ${scope}\n`);
    return 2;
  }

  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const client = deps.client ?? createClient(resolveToken());

  const prData = await client.request<GetPullRequestByNumberResponse>(GET_PULL_REQUEST_BY_NUMBER, {
    owner: repo.owner,
    name: repo.name,
    number: pr,
  });
  const prNode = prData.repository?.pullRequest;
  if (!prNode) {
    stderr.write(`PR not found: ${repo.owner}/${repo.name}#${pr}\n`);
    return 1;
  }

  if (containsCloseLink(prNode.body, task)) {
    stdout.write(`${t(locale, 'link.alreadyLinked')}: ${prNode.url}\n`);
    return 0;
  }

  const updatedBody = appendCloseLink(prNode.body, task);
  const updated = await client.request<UpdatePullRequestResponse>(UPDATE_PULL_REQUEST, {
    input: { pullRequestId: prNode.id, body: updatedBody },
  });
  stdout.write(`${t(locale, 'link.added')}: ${updated.updatePullRequest.pullRequest.url}\n`);
  return 0;
}

const CLOSE_KEYWORDS = ['Closes', 'Fixes', 'Resolves'] as const;

export function containsCloseLink(body: string, taskNumber: number): boolean {
  const pattern = new RegExp(`\\b(?:${CLOSE_KEYWORDS.join('|')})\\s+#${taskNumber}\\b`, 'i');
  return pattern.test(body);
}

export function appendCloseLink(body: string, taskNumber: number): string {
  const trimmed = body.replace(/\s+$/, '');
  const sep = trimmed.length > 0 ? '\n\n' : '';
  return `${trimmed}${sep}Closes #${taskNumber}\n`;
}

function parsePositionalNumbers(argv: readonly string[]): number[] {
  const out: number[] = [];
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;
    if (arg.startsWith('--')) {
      if (!arg.includes('=') && (arg === '--scope' || arg === '--repo' || arg === '--lang')) {
        i++;
      }
      continue;
    }
    const stripped = arg.startsWith('#') ? arg.slice(1) : arg;
    const n = Number.parseInt(stripped, 10);
    if (Number.isFinite(n) && n > 0) out.push(n);
  }
  return out;
}
