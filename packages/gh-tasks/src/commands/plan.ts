import { resolveLocale, t } from '../i18n/index.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import { type Period, parsePeriodFlag, rangeOf, suggestMilestoneTitle } from '../lib/period.ts';
import {
  LIST_REPO_ISSUES,
  type ListRepoIssuesResponse,
  type RepoIssueNode,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope } from '../lib/scope.ts';

export interface PlanCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  /** Override `now` for deterministic testing. */
  now?: () => Date;
}

const FETCH_LIMIT = 100;
const DEFAULT_PERIOD: Period = 'weekly';

/**
 * v0.1.0 implementation: dry-run only. Lists open issues whose `updatedAt`
 * falls within the period and prints a suggested Milestone title. Actual
 * Milestone creation requires REST API plumbing (GraphQL has no
 * createMilestone mutation), tracked for v0.2.0.
 */
export async function plan(argv: readonly string[], deps: PlanCommandDeps = {}): Promise<number> {
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

  const data = await client.request<ListRepoIssuesResponse>(LIST_REPO_ISSUES, {
    owner: repo.owner,
    name: repo.name,
    first: FETCH_LIMIT,
  });
  if (!data.repository) {
    stderr.write(`repository not found: ${repo.owner}/${repo.name}\n`);
    return 1;
  }

  const now = deps.now ? deps.now() : new Date();
  const range = rangeOf(period, now);
  const inRange = data.repository.issues.nodes.filter((i: RepoIssueNode) => {
    const updated = new Date(i.updatedAt).getTime();
    return updated >= range.start.getTime() && updated < range.end.getTime();
  });

  const title = suggestMilestoneTitle(period, now);
  stdout.write(`${t(locale, 'plan.proposed')}: ${title}\n`);
  stdout.write(
    `  ${range.start.toISOString().slice(0, 10)} → ${range.end.toISOString().slice(0, 10)}\n\n`
  );
  if (inRange.length === 0) {
    stdout.write(`${t(locale, 'plan.empty')}\n`);
  } else {
    stdout.write(`${t(locale, 'plan.candidates')} (${inRange.length})\n`);
    for (const issue of inRange) {
      stdout.write(`  #${issue.number}  ${issue.title}\n`);
    }
  }
  stdout.write(`\n${t(locale, 'plan.dryRunNote')}\n`);
  return 0;
}
