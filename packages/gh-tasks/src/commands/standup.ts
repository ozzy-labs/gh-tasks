import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import { ProjectError, type ProjectRef, resolveProjectRef } from '../lib/project.ts';
import {
  GET_ORG_PROJECT_V2,
  GET_USER_PROJECT_V2,
  GET_VIEWER_LOGIN,
  type GetOrgProjectV2Response,
  type GetUserProjectV2Response,
  type GetViewerLoginResponse,
  LIST_CLOSED_ISSUES,
  LIST_MERGED_PRS,
  LIST_PROJECT_V2_ITEMS,
  LIST_REPO_ISSUES,
  type ListClosedIssuesResponse,
  type ListMergedPRsResponse,
  type ListProjectV2ItemsResponse,
  type ListRepoIssuesResponse,
  type ProjectV2FieldValue,
  type ProjectV2ItemNode,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope, type Scope } from '../lib/scope.ts';

export interface StandupCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  now?: () => Date;
  config?: AppConfig;
}

const FETCH_LIMIT = 100;

/**
 * Standup activity summary.
 *
 * - `--scope repo` (default in a Git repo): Issues / PRs closed-or-merged
 *   since `--since` (default: 24h ago) under "Yesterday", recently-updated
 *   open Issues under "Today", and a placeholder hint under "Blockers".
 * - `--scope org|user`: Projects v2 items whose `updatedAt` is at-or-after
 *   `--since`. Items are split by Status: "Done" → Yesterday, anything
 *   else (incl. unset Status) → Today. Blockers reuses the placeholder
 *   from repo scope — Projects v2 has no "blocked" semantics we can
 *   detect generically.
 *
 * `--mine` filters to items where the viewer is either the content's
 * `author` or one of its `assignees`. DraftIssues have no author/assignee
 * fields, so under `--mine` they are excluded entirely.
 */
export async function standup(
  argv: readonly string[],
  deps: StandupCommandDeps = {}
): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote, config: deps.config });
  const since = parseSince(argv, deps.now ? deps.now() : new Date());
  const mine = argv.includes('--mine');

  if (scope === 'repo') {
    return await standupRepo({ argv, deps, locale, stdout, since, mine });
  }

  return await standupProject({ argv, deps, locale, scope, stdout, stderr, since, mine });
}

interface StandupRepoContext {
  argv: readonly string[];
  deps: StandupCommandDeps;
  locale: 'ja' | 'en';
  stdout: NodeJS.WritableStream;
  since: number;
  mine: boolean;
}

async function standupRepo(ctx: StandupRepoContext): Promise<number> {
  const { argv, deps, locale, stdout, since, mine } = ctx;
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

  const matchesViewer = (n: {
    author?: { login: string } | null;
    assignees?: { nodes: Array<{ login: string }> };
  }): boolean => {
    if (!viewerLogin) return true;
    if (n.author?.login === viewerLogin) return true;
    return n.assignees?.nodes.some((a) => a.login === viewerLogin) ?? false;
  };

  const closedIssues =
    issuesData.repository?.issues.nodes
      .filter((n) => new Date(n.closedAt).getTime() >= since)
      .filter(matchesViewer) ?? [];
  const mergedPRs =
    prsData.repository?.pullRequests.nodes
      .filter((n) => new Date(n.mergedAt).getTime() >= since)
      .filter(matchesViewer) ?? [];
  const openIssues =
    openIssuesData.repository?.issues.nodes
      .filter((n) => new Date(n.updatedAt).getTime() >= since)
      .filter(matchesViewer) ?? [];

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
  stdout.write(lines.join('\n'));
  return 0;
}

interface StandupProjectContext {
  argv: readonly string[];
  deps: StandupCommandDeps;
  locale: 'ja' | 'en';
  scope: Exclude<Scope, 'repo'>;
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  since: number;
  mine: boolean;
}

async function standupProject(ctx: StandupProjectContext): Promise<number> {
  const { argv, deps, locale, scope, stdout, stderr, since, mine } = ctx;

  let projectRef: ProjectRef;
  try {
    projectRef = resolveProjectRef({ scope, argv, config: deps.config });
  } catch (err) {
    if (err instanceof ProjectError) {
      stderr.write(`${err.message}\n`);
      return 2;
    }
    throw err;
  }

  const client = deps.client ?? createClient(resolveToken());

  const viewerLogin = mine
    ? (await client.request<GetViewerLoginResponse>(GET_VIEWER_LOGIN, {})).viewer.login
    : null;

  const projectId = await resolveProjectNodeId({ client, scope, projectRef });
  if (!projectId) {
    stderr.write(
      `project not found: ${projectRef.owner}/${projectRef.number} (--scope ${scope})\n`
    );
    return 1;
  }

  const data = await client.request<ListProjectV2ItemsResponse>(LIST_PROJECT_V2_ITEMS, {
    projectId,
    first: FETCH_LIMIT,
  });
  if (!data.node) {
    stderr.write(
      `project not found: ${projectRef.owner}/${projectRef.number} (--scope ${scope})\n`
    );
    return 1;
  }

  const inRange = data.node.items.nodes.filter((item) => {
    const ms = new Date(item.updatedAt).getTime();
    return Number.isFinite(ms) && ms >= since;
  });
  const visible = viewerLogin
    ? inRange.filter((item) => matchesViewerOnItem(item, viewerLogin))
    : inRange;

  const yesterday: ProjectV2ItemNode[] = [];
  const today: ProjectV2ItemNode[] = [];
  for (const item of visible) {
    if (isDone(item)) yesterday.push(item);
    else today.push(item);
  }

  const lines: string[] = [];
  lines.push(`# ${t(locale, 'standup.heading')}${mine && viewerLogin ? ` (@${viewerLogin})` : ''}`);
  lines.push(`since ${new Date(since).toISOString()}`);
  lines.push('');
  lines.push(`## ${t(locale, 'standup.yesterday')}`);
  if (yesterday.length === 0) {
    lines.push(`- ${t(locale, 'standup.empty.project')}`);
  } else {
    for (const item of yesterday) lines.push(`- done: ${formatItemLine(item)}`);
  }
  lines.push('');
  lines.push(`## ${t(locale, 'standup.today')}`);
  if (today.length === 0) {
    lines.push(`- ${t(locale, 'standup.empty.project')}`);
  } else {
    for (const item of today) lines.push(`- in-progress: ${formatItemLine(item)}`);
  }
  lines.push('');
  lines.push(`## ${t(locale, 'standup.blockers')}`);
  lines.push(`- ${t(locale, 'standup.blockersHint')}`);
  lines.push('');
  stdout.write(lines.join('\n'));
  return 0;
}

/**
 * Match an item against the viewer login. DraftIssues have no author /
 * assignee fields, so under `--mine` they cannot match — this returns
 * false for them by construction.
 */
function matchesViewerOnItem(item: ProjectV2ItemNode, viewerLogin: string): boolean {
  const c = item.content;
  if (!c) return false;
  if (c.__typename === 'DraftIssue') return false;
  if (c.author?.login === viewerLogin) return true;
  return c.assignees?.nodes.some((a) => a.login === viewerLogin) ?? false;
}

/** Status is "Done" (case-insensitive). Status field is conventionally named "Status". */
function isDone(item: ProjectV2ItemNode): boolean {
  const status = findStatus(item.fieldValues.nodes);
  if (status === null) return false;
  return status.toLowerCase() === 'done';
}

function findStatus(values: readonly ProjectV2FieldValue[]): string | null {
  for (const v of values) {
    if (
      v.__typename === 'ProjectV2ItemFieldSingleSelectValue' &&
      v.field.name.toLowerCase() === 'status'
    ) {
      return v.name;
    }
  }
  return null;
}

function formatItemLine(item: ProjectV2ItemNode): string {
  const c = item.content;
  if (!c) return '(no content)';
  if (c.__typename === 'Issue' || c.__typename === 'PullRequest') {
    const prefix = c.__typename === 'PullRequest' ? 'PR' : '';
    return `${prefix}#${c.number} ${c.title} (${c.url})`;
  }
  return `(draft) ${c.title}`;
}

interface ResolveProjectNodeIdOptions {
  client: GraphQLClient;
  scope: Exclude<Scope, 'repo'>;
  projectRef: ProjectRef;
}

async function resolveProjectNodeId(opts: ResolveProjectNodeIdOptions): Promise<string | null> {
  const { client, scope, projectRef } = opts;
  if (scope === 'org') {
    const data = await client.request<GetOrgProjectV2Response>(GET_ORG_PROJECT_V2, {
      login: projectRef.owner,
      number: projectRef.number,
    });
    return data.organization?.projectV2?.id ?? null;
  }
  const data = await client.request<GetUserProjectV2Response>(GET_USER_PROJECT_V2, {
    login: projectRef.owner,
    number: projectRef.number,
  });
  return data.user?.projectV2?.id ?? null;
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
