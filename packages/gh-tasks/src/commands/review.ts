import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import { type DateRange, type Period, parsePeriodFlag, rangeOf } from '../lib/period.ts';
import { ProjectError, type ProjectRef, resolveProjectRef } from '../lib/project.ts';
import {
  type ClosedIssueNode,
  GET_ORG_PROJECT_V2,
  GET_USER_PROJECT_V2,
  type GetOrgProjectV2Response,
  type GetUserProjectV2Response,
  LIST_CLOSED_ISSUES,
  LIST_MERGED_PRS,
  LIST_PROJECT_V2_ITEMS,
  type ListClosedIssuesResponse,
  type ListMergedPRsResponse,
  type ListProjectV2ItemsResponse,
  type MergedPRNode,
  type ProjectV2FieldValue,
  type ProjectV2ItemNode,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope, type Scope } from '../lib/scope.ts';

export interface ReviewCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  now?: () => Date;
  config?: AppConfig;
}

const FETCH_LIMIT = 100;
const DEFAULT_PERIOD: Period = 'weekly';

/**
 * Retrospective summary for the chosen `--period`.
 *
 * - `--scope repo` (default in a Git repo): aggregates Issues `closedAt` and
 *   PRs `mergedAt` that fall in the period window.
 * - `--scope org|user`: aggregates Projects v2 items whose Status is "Done"
 *   (case-insensitive) and whose `updatedAt` falls in the period window.
 *   `updatedAt` is the proxy for "moved to Done" — Projects v2 doesn't expose
 *   per-field-change timestamps via GraphQL.
 */
export async function review(
  argv: readonly string[],
  deps: ReviewCommandDeps = {}
): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const period = parsePeriodFlag(argv) ?? DEFAULT_PERIOD;
  const now = deps.now ? deps.now() : new Date();

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote, config: deps.config });

  if (scope === 'repo') {
    return await reviewRepo({ argv, deps, locale, stdout, stderr, period, now });
  }

  return await reviewProject({ argv, deps, locale, scope, stdout, stderr, period, now });
}

interface ReviewRepoContext {
  argv: readonly string[];
  deps: ReviewCommandDeps;
  locale: 'ja' | 'en';
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  period: Period;
  now: Date;
}

async function reviewRepo(ctx: ReviewRepoContext): Promise<number> {
  const { argv, deps, locale, stdout, period, now } = ctx;

  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const client = deps.client ?? createClient(resolveToken());
  const range = rangeOf(period, now);

  const [issuesData, prsData] = await Promise.all([
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
  ]);

  const closedIssues =
    issuesData.repository?.issues.nodes.filter((n) => withinRange(n.closedAt, range)) ?? [];
  const mergedPRs =
    prsData.repository?.pullRequests.nodes.filter((n) => withinRange(n.mergedAt, range)) ?? [];

  stdout.write(renderRepoMarkdown({ period, range, closedIssues, mergedPRs, locale }));
  return 0;
}

interface ReviewProjectContext {
  argv: readonly string[];
  deps: ReviewCommandDeps;
  locale: 'ja' | 'en';
  scope: Exclude<Scope, 'repo'>;
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  period: Period;
  now: Date;
}

async function reviewProject(ctx: ReviewProjectContext): Promise<number> {
  const { argv, deps, locale, scope, stdout, stderr, period, now } = ctx;

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

  const range = rangeOf(period, now);
  const completed = data.node.items.nodes.filter(
    (item) => withinRange(item.updatedAt, range) && isDone(item)
  );

  stdout.write(renderProjectMarkdown({ period, range, completed, locale }));
  return 0;
}

function withinRange(iso: string | null | undefined, range: DateRange): boolean {
  if (!iso) return false;
  const ms = new Date(iso).getTime();
  if (!Number.isFinite(ms)) return false;
  return ms >= range.start.getTime() && ms < range.end.getTime();
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

interface RenderRepoInput {
  period: Period;
  range: DateRange;
  closedIssues: ClosedIssueNode[];
  mergedPRs: MergedPRNode[];
  locale: 'ja' | 'en';
}

function renderRepoMarkdown(input: RenderRepoInput): string {
  const { period, range, closedIssues, mergedPRs, locale } = input;
  const startStr = range.start.toISOString().slice(0, 10);
  const endStr = range.end.toISOString().slice(0, 10);
  const lines: string[] = [];
  lines.push(`# ${t(locale, 'review.heading')} (${period})`);
  lines.push(`${startStr} → ${endStr}`);
  lines.push('');
  lines.push(`## ${t(locale, 'review.closedIssues')} (${closedIssues.length})`);
  if (closedIssues.length === 0) {
    lines.push(`- ${t(locale, 'review.none')}`);
  } else {
    for (const i of closedIssues) {
      lines.push(`- #${i.number} ${i.title} (${i.url})`);
    }
  }
  lines.push('');
  lines.push(`## ${t(locale, 'review.mergedPRs')} (${mergedPRs.length})`);
  if (mergedPRs.length === 0) {
    lines.push(`- ${t(locale, 'review.none')}`);
  } else {
    for (const p of mergedPRs) {
      lines.push(`- #${p.number} ${p.title} (${p.url})`);
    }
  }
  lines.push('');
  return lines.join('\n');
}

interface RenderProjectInput {
  period: Period;
  range: DateRange;
  completed: ProjectV2ItemNode[];
  locale: 'ja' | 'en';
}

function renderProjectMarkdown(input: RenderProjectInput): string {
  const { period, range, completed, locale } = input;
  const startStr = range.start.toISOString().slice(0, 10);
  const endStr = range.end.toISOString().slice(0, 10);
  const lines: string[] = [];
  lines.push(`# ${t(locale, 'review.heading')} (${period})`);
  lines.push(`${startStr} → ${endStr}`);
  lines.push('');
  lines.push(`## ${t(locale, 'review.completedProjectItems')} (${completed.length})`);
  if (completed.length === 0) {
    lines.push(`- ${t(locale, 'review.empty.project')}`);
  } else {
    for (const item of completed) {
      lines.push(`- ${formatItemLine(item)}`);
    }
  }
  lines.push('');
  return lines.join('\n');
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
