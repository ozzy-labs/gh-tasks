import { resolveLocale, t } from '../i18n/index.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import {
  GET_VIEWER_LOGIN,
  type GetViewerLoginResponse,
  LIST_CLOSED_ISSUES,
  LIST_MERGED_PRS,
  LIST_REPO_ISSUES,
  type ListClosedIssuesResponse,
  type ListMergedPRsResponse,
  type ListRepoIssuesResponse,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope } from '../lib/scope.ts';

export interface StandupCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  now?: () => Date;
}

const FETCH_LIMIT = 100;

export async function standup(
  argv: readonly string[],
  deps: StandupCommandDeps = {}
): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv);

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote });
  if (scope !== 'repo') {
    stderr.write(`${t(locale, 'error.scope.notImplemented')}: --scope ${scope}\n`);
    return 2;
  }

  const since = parseSince(argv, deps.now ? deps.now() : new Date());
  const mine = argv.includes('--mine');

  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const client = deps.client ?? createClient(resolveToken());

  const viewerLogin = mine
    ? (await client.request<GetViewerLoginResponse>(GET_VIEWER_LOGIN, {})).viewer.login
    : null;

  const [issuesData, prsData, openIssuesData] = await Promise.all([
    client.request<ListClosedIssuesResponse>(LIST_CLOSED_ISSUES, {
      owner: repo.owner,
      name: repo.name,
      first: FETCH_LIMIT,
    }),
    client.request<ListMergedPRsResponse>(LIST_MERGED_PRS, {
      owner: repo.owner,
      name: repo.name,
      first: FETCH_LIMIT,
    }),
    client.request<ListRepoIssuesResponse>(LIST_REPO_ISSUES, {
      owner: repo.owner,
      name: repo.name,
      first: FETCH_LIMIT,
    }),
  ]);

  const closedIssues =
    issuesData.repository?.issues.nodes.filter((n) => new Date(n.closedAt).getTime() >= since) ??
    [];
  const mergedPRs =
    prsData.repository?.pullRequests.nodes.filter((n) => new Date(n.mergedAt).getTime() >= since) ??
    [];
  const openIssues =
    openIssuesData.repository?.issues.nodes.filter(
      (n) => new Date(n.updatedAt).getTime() >= since
    ) ?? [];

  const lines: string[] = [];
  lines.push(`# ${t(locale, 'standup.heading')}${mine && viewerLogin ? ` (@${viewerLogin})` : ''}`);
  lines.push(`since ${new Date(since).toISOString()}`);
  lines.push('');
  lines.push(`## ${t(locale, 'standup.yesterday')}`);
  if (closedIssues.length === 0 && mergedPRs.length === 0) {
    lines.push(`- ${t(locale, 'standup.none')}`);
  } else {
    for (const i of closedIssues) lines.push(`- closed: #${i.number} ${i.title} (${i.url})`);
    for (const p of mergedPRs) lines.push(`- merged: #${p.number} ${p.title} (${p.url})`);
  }
  lines.push('');
  lines.push(`## ${t(locale, 'standup.today')}`);
  if (openIssues.length === 0) {
    lines.push(`- ${t(locale, 'standup.none')}`);
  } else {
    for (const i of openIssues) lines.push(`- in-progress: #${i.number} ${i.title} (${i.url})`);
  }
  lines.push('');
  lines.push(`## ${t(locale, 'standup.blockers')}`);
  lines.push(`- ${t(locale, 'standup.blockersHint')}`);
  lines.push('');
  if (mine) {
    lines.push(t(locale, 'standup.mineNote'));
    lines.push('');
  }
  stdout.write(lines.join('\n'));
  return 0;
}

function parseSince(argv: readonly string[], now: Date): number {
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;
    if (arg.startsWith('--since=')) {
      const ms = Date.parse(arg.slice('--since='.length));
      if (Number.isFinite(ms)) return ms;
    }
    if (arg === '--since') {
      const next = argv[i + 1];
      if (next !== undefined) {
        const ms = Date.parse(next);
        if (Number.isFinite(ms)) return ms;
      }
    }
  }
  return now.getTime() - 24 * 60 * 60 * 1000;
}
