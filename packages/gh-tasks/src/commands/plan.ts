import { resolveLocale, t } from '../i18n/index.ts';
import {
  createClient,
  createRestClient,
  type GraphQLClient,
  type RestClient,
  resolveToken,
} from '../lib/github.ts';
import { type Period, parsePeriodFlag, rangeOf, suggestMilestoneTitle } from '../lib/period.ts';
import {
  createMilestone,
  LIST_MILESTONES,
  LIST_REPO_ISSUES_WITH_MILESTONE,
  type ListMilestonesResponse,
  type ListRepoIssuesWithMilestoneResponse,
  type RepoIssueWithMilestoneNode,
  UPDATE_ISSUE_MILESTONE,
  type UpdateIssueMilestoneResponse,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope } from '../lib/scope.ts';

export interface PlanCommandDeps {
  client?: GraphQLClient;
  rest?: RestClient;
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
 * Plan a daily / weekly / sprint cycle for the current repo.
 *
 * Default mode (write): finds-or-creates the Milestone whose title matches
 * the period (`Daily YYYY-MM-DD`, `Week of YYYY-MM-DD`, `Sprint YYYY-MM-DD`)
 * and binds open Issues whose `updatedAt` falls in the period.
 *
 * `--dry-run` keeps the original v0.1.0 preview output without mutating
 * anything.
 *
 * Issues already bound to a different Milestone are skipped (we never
 * silently re-route someone else's planning state).
 */
export async function plan(argv: readonly string[], deps: PlanCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv);

  const period = parsePeriodFlag(argv) ?? DEFAULT_PERIOD;
  const dryRun = argv.includes('--dry-run');

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote });
  if (scope !== 'repo') {
    stderr.write(`${t(locale, 'error.scope.notImplemented')}: --scope ${scope}\n`);
    return 2;
  }

  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const needsToken = !deps.client || (!dryRun && !deps.rest);
  const token = needsToken ? resolveToken() : '';
  const client = deps.client ?? createClient(token);
  const rest = deps.rest ?? createRestClient(token);

  const issuesData = await client.request<ListRepoIssuesWithMilestoneResponse>(
    LIST_REPO_ISSUES_WITH_MILESTONE,
    { owner: repo.owner, name: repo.name, first: FETCH_LIMIT }
  );
  if (!issuesData.repository) {
    stderr.write(`repository not found: ${repo.owner}/${repo.name}\n`);
    return 1;
  }

  const now = deps.now ? deps.now() : new Date();
  const range = rangeOf(period, now);
  const inRange = issuesData.repository.issues.nodes.filter((i: RepoIssueWithMilestoneNode) => {
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
    if (dryRun) stdout.write(`\n${t(locale, 'plan.dryRunNote')}\n`);
    return 0;
  }

  stdout.write(`${t(locale, 'plan.candidates')} (${inRange.length})\n`);
  for (const issue of inRange) {
    stdout.write(`  #${issue.number}  ${issue.title}\n`);
  }
  stdout.write('\n');

  if (dryRun) {
    stdout.write(`${t(locale, 'plan.dryRunNote')}\n`);
    return 0;
  }

  // Reuse a same-titled Milestone if present; otherwise create one.
  const milestonesData = await client.request<ListMilestonesResponse>(LIST_MILESTONES, {
    owner: repo.owner,
    name: repo.name,
    first: FETCH_LIMIT,
  });
  const existing = milestonesData.repository?.milestones.nodes.find((m) => m.title === title);

  let milestoneId: string;
  let milestoneNumber: number;
  if (existing) {
    milestoneId = existing.id;
    milestoneNumber = existing.number;
    stdout.write(`${t(locale, 'plan.reused')}: ${title} (#${existing.number})\n`);
  } else {
    const created = await createMilestone(rest, {
      owner: repo.owner,
      name: repo.name,
      title,
    });
    milestoneId = created.node_id;
    milestoneNumber = created.number;
    stdout.write(`${t(locale, 'plan.created')}: ${title} (#${created.number})\n`);
  }

  for (const issue of inRange) {
    if (issue.milestone && issue.milestone.id !== milestoneId) {
      stdout.write(
        `  ${t(locale, 'plan.skippedExisting')}: #${issue.number} → ${issue.milestone.title}\n`
      );
      continue;
    }
    if (issue.milestone && issue.milestone.id === milestoneId) {
      // Already bound to the right milestone — nothing to do.
      continue;
    }
    await client.request<UpdateIssueMilestoneResponse>(UPDATE_ISSUE_MILESTONE, {
      input: { id: issue.id, milestoneId },
    });
    stdout.write(`  ${t(locale, 'plan.linked')}: #${issue.number}\n`);
  }

  // Surface the Milestone number so the user can jump to it without scanning.
  stdout.write(`\nhttps://github.com/${repo.owner}/${repo.name}/milestone/${milestoneNumber}\n`);
  return 0;
}
