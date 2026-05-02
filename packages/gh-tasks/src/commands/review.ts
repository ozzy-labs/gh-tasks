import { resolveLocale, t } from '../i18n/index.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import { type DateRange, type Period, parsePeriodFlag, rangeOf } from '../lib/period.ts';
import {
  type ClosedIssueNode,
  LIST_CLOSED_ISSUES,
  LIST_MERGED_PRS,
  type ListClosedIssuesResponse,
  type ListMergedPRsResponse,
  type MergedPRNode,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope } from '../lib/scope.ts';

export interface ReviewCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  now?: () => Date;
}

const FETCH_LIMIT = 100;
const DEFAULT_PERIOD: Period = 'weekly';

export async function review(
  argv: readonly string[],
  deps: ReviewCommandDeps = {}
): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv);

  const period = parsePeriodFlag(argv) ?? DEFAULT_PERIOD;

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote });
  if (scope !== 'repo') {
    stderr.write(`${t(locale, 'error.scope.notImplemented')}: --scope ${scope}\n`);
    return 2;
  }

  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const client = deps.client ?? createClient(resolveToken());
  const now = deps.now ? deps.now() : new Date();
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

  stdout.write(renderMarkdown({ period, range, closedIssues, mergedPRs, locale }));
  return 0;
}

function withinRange(iso: string, range: DateRange): boolean {
  const t = new Date(iso).getTime();
  return t >= range.start.getTime() && t < range.end.getTime();
}

interface RenderInput {
  period: Period;
  range: DateRange;
  closedIssues: ClosedIssueNode[];
  mergedPRs: MergedPRNode[];
  locale: 'ja' | 'en';
}

function renderMarkdown(input: RenderInput): string {
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
