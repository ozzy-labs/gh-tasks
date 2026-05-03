import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import {
  createClient,
  createRestClient,
  type GraphQLClient,
  type RestClient,
  resolveToken,
} from '../lib/github.ts';
import { type Period, parsePeriodFlag, rangeOf, suggestMilestoneTitle } from '../lib/period.ts';
import { ProjectError, type ProjectRef, resolveProjectRef } from '../lib/project.ts';
import { resolveProjectNodeId } from '../lib/projectItem.ts';
import {
  createMilestone,
  LIST_MILESTONES,
  LIST_PROJECT_V2_FIELDS,
  LIST_PROJECT_V2_ITEMS,
  LIST_REPO_ISSUES_WITH_MILESTONE,
  type ListMilestonesResponse,
  type ListProjectV2FieldsResponse,
  type ListProjectV2ItemsResponse,
  type ListRepoIssuesWithMilestoneResponse,
  type ProjectV2FieldNode,
  type ProjectV2ItemNode,
  type ProjectV2IterationOption,
  type RepoIssueWithMilestoneNode,
  UPDATE_ISSUE_MILESTONE,
  UPDATE_PROJECT_V2_ITEM_FIELD_VALUE,
  type UpdateIssueMilestoneResponse,
  type UpdateProjectV2ItemFieldValueResponse,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope, type Scope } from '../lib/scope.ts';

export interface PlanCommandDeps {
  client?: GraphQLClient;
  rest?: RestClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  /** Override `now` for deterministic testing. */
  now?: () => Date;
  config?: AppConfig;
}

const FETCH_LIMIT = 100;
const FIELDS_FETCH_LIMIT = 50;
const DEFAULT_PERIOD: Period = 'weekly';

/**
 * Plan a daily / weekly / sprint cycle.
 *
 * - `--scope repo` (default when in a Git repo): finds-or-creates the
 *   Milestone whose title matches the period and binds open Issues whose
 *   `updatedAt` falls in the period.
 * - `--scope org|user`: finds the Iteration on the target Projects v2 board
 *   that matches the period title (or falls back to the iteration containing
 *   the current day) and updates the Iteration field on every project item
 *   updated within the period.
 *
 * `--dry-run` keeps preview output without mutating anything in either mode.
 *
 * Items already bound to a different Milestone (repo) or already on the
 * target Iteration (project) are skipped — we never silently re-route
 * someone else's planning state and we avoid no-op writes.
 */
export async function plan(argv: readonly string[], deps: PlanCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const period = parsePeriodFlag(argv) ?? DEFAULT_PERIOD;
  const dryRun = argv.includes('--dry-run');
  const now = deps.now ? deps.now() : new Date();

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote, config: deps.config });

  if (scope === 'repo') {
    return await planRepo({ argv, deps, locale, stdout, stderr, period, dryRun, now });
  }

  return await planProject({ argv, deps, locale, scope, stdout, stderr, period, dryRun, now });
}

interface PlanRepoContext {
  argv: readonly string[];
  deps: PlanCommandDeps;
  locale: 'ja' | 'en';
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  period: Period;
  dryRun: boolean;
  now: Date;
}

async function planRepo(ctx: PlanRepoContext): Promise<number> {
  const { argv, deps, locale, stdout, stderr, period, dryRun, now } = ctx;

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

interface PlanProjectContext {
  argv: readonly string[];
  deps: PlanCommandDeps;
  locale: 'ja' | 'en';
  scope: Exclude<Scope, 'repo'>;
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  period: Period;
  dryRun: boolean;
  now: Date;
}

async function planProject(ctx: PlanProjectContext): Promise<number> {
  const { argv, deps, locale, scope, stdout, stderr, period, dryRun, now } = ctx;

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

  const fieldsData = await client.request<ListProjectV2FieldsResponse>(LIST_PROJECT_V2_FIELDS, {
    projectId,
    first: FIELDS_FETCH_LIMIT,
  });
  if (!fieldsData.node) {
    stderr.write(
      `project not found: ${projectRef.owner}/${projectRef.number} (--scope ${scope})\n`
    );
    return 1;
  }

  const iterationField = findIterationField(fieldsData.node.fields.nodes);
  if (!iterationField) {
    stderr.write(`${t(locale, 'error.plan.iterationFieldMissing')}\n`);
    return 1;
  }

  const range = rangeOf(period, now);
  const targetTitle = suggestMilestoneTitle(period, now);

  const resolved = resolveTargetIteration(iterationField.iterations, targetTitle, now);
  if (!resolved) {
    stderr.write(`${t(locale, 'error.plan.noIterationsAvailable')}\n`);
    return 1;
  }

  // Header: proposed iteration + period range.
  stdout.write(`${t(locale, 'plan.proposed.project')}: ${resolved.iteration.title}\n`);
  stdout.write(
    `  ${range.start.toISOString().slice(0, 10)} → ${range.end.toISOString().slice(0, 10)}\n`
  );
  if (resolved.matched) {
    stdout.write(`  ${t(locale, 'plan.iterationMatched.project')}: ${targetTitle}\n\n`);
  } else {
    // Surface fallback to stderr so scripts can detect it; also echo to stdout
    // for the human reading the preview.
    stderr.write(
      `${t(locale, 'plan.iterationFallback.project')}: ${targetTitle} → ${resolved.iteration.title}\n`
    );
    stdout.write(`  ${t(locale, 'plan.iterationFallback.project')}\n\n`);
  }

  const itemsData = await client.request<ListProjectV2ItemsResponse>(LIST_PROJECT_V2_ITEMS, {
    projectId,
    first: FETCH_LIMIT,
  });
  const allItems = itemsData.node?.items.nodes ?? [];
  const inRange = allItems.filter((item) => {
    const updated = new Date(item.updatedAt).getTime();
    return updated >= range.start.getTime() && updated < range.end.getTime();
  });

  if (inRange.length === 0) {
    stdout.write(`${t(locale, 'plan.empty.project')}\n`);
    if (dryRun) stdout.write(`\n${t(locale, 'plan.dryRunNote.project')}\n`);
    return 0;
  }

  stdout.write(`${t(locale, 'plan.candidates.project')} (${inRange.length})\n`);
  for (const item of inRange) {
    stdout.write(formatItemLine(item));
  }
  stdout.write('\n');

  if (dryRun) {
    stdout.write(`${t(locale, 'plan.dryRunNote.project')}\n`);
    return 0;
  }

  for (const item of inRange) {
    if (isAlreadyOnIteration(item, iterationField.id, resolved.iteration.id)) {
      stdout.write(`  ${t(locale, 'plan.iterationAlreadySet.project')}: ${describeItem(item)}\n`);
      continue;
    }
    await client.request<UpdateProjectV2ItemFieldValueResponse>(
      UPDATE_PROJECT_V2_ITEM_FIELD_VALUE,
      {
        input: {
          projectId,
          itemId: item.id,
          fieldId: iterationField.id,
          value: { iterationId: resolved.iteration.id },
        },
      }
    );
    stdout.write(`  ${t(locale, 'plan.iterationUpdated.project')}: ${describeItem(item)}\n`);
  }

  return 0;
}

interface IterationFieldShape {
  id: string;
  name: string;
  iterations: ProjectV2IterationOption[];
}

function findIterationField(fields: readonly ProjectV2FieldNode[]): IterationFieldShape | null {
  for (const f of fields) {
    if (f.dataType === 'ITERATION' && f.name.toLowerCase() === 'iteration') {
      return {
        id: f.id,
        name: f.name,
        iterations: [...f.configuration.iterations],
      };
    }
  }
  // Fall back to any ITERATION field if no field is literally named "Iteration".
  for (const f of fields) {
    if (f.dataType === 'ITERATION') {
      return {
        id: f.id,
        name: f.name,
        iterations: [...f.configuration.iterations],
      };
    }
  }
  return null;
}

interface ResolvedIteration {
  iteration: ProjectV2IterationOption;
  matched: boolean;
}

/**
 * Pick the iteration to write. Order:
 *   1. exact-title match (`Daily YYYY-MM-DD` etc.)
 *   2. iteration whose [startDate, startDate + duration) window contains `now`
 *   3. nearest upcoming iteration by startDate
 *   4. last available iteration
 *
 * Returns null only when the field has no iterations at all (the caller
 * reports an actionable error rather than picking arbitrarily).
 *
 * GitHub's Projects v2 GraphQL API exposes iteration creation only through
 * `updateProjectV2IterationField`-style mutations on the *iteration field's
 * configuration*, not as a per-iteration create. Rather than implement that
 * indirect mutation, we keep the v0.1.0 behavior: find-or-fallback. The
 * caller can pre-create iterations in the Project UI to make the matching
 * deterministic.
 */
function resolveTargetIteration(
  iterations: readonly ProjectV2IterationOption[],
  targetTitle: string,
  now: Date
): ResolvedIteration | null {
  if (iterations.length === 0) return null;

  const exact = iterations.find((it) => it.title === targetTitle);
  if (exact) return { iteration: exact, matched: true };

  const nowMs = now.getTime();
  const current = iterations.find((it) => containsDay(it, nowMs));
  if (current) return { iteration: current, matched: false };

  const upcoming = iterations
    .filter((it) => new Date(it.startDate).getTime() >= nowMs)
    .sort((a, b) => new Date(a.startDate).getTime() - new Date(b.startDate).getTime());
  if (upcoming[0]) return { iteration: upcoming[0], matched: false };

  // No current and no upcoming — pick the most recent past one (last entry).
  const last = iterations[iterations.length - 1];
  if (last) return { iteration: last, matched: false };
  return null;
}

function containsDay(iteration: ProjectV2IterationOption, nowMs: number): boolean {
  const start = new Date(iteration.startDate).getTime();
  if (Number.isNaN(start)) return false;
  const end = start + iteration.duration * 24 * 60 * 60 * 1000;
  return nowMs >= start && nowMs < end;
}

function isAlreadyOnIteration(
  item: ProjectV2ItemNode,
  iterationFieldId: string,
  iterationId: string
): boolean {
  for (const v of item.fieldValues.nodes) {
    if (
      v.__typename === 'ProjectV2ItemFieldIterationValue' &&
      v.field.id === iterationFieldId &&
      v.iterationId === iterationId
    ) {
      return true;
    }
  }
  return false;
}

function formatItemLine(item: ProjectV2ItemNode): string {
  const c = item.content;
  if (!c) return `  (no content)\n`;
  if (c.__typename === 'Issue' || c.__typename === 'PullRequest') {
    const prefix = c.__typename === 'PullRequest' ? 'PR' : '';
    return `  ${prefix}#${c.number}  ${c.title}\n`;
  }
  return `  (draft)  ${c.title}\n`;
}

function describeItem(item: ProjectV2ItemNode): string {
  const c = item.content;
  if (!c) return item.id;
  if (c.__typename === 'Issue' || c.__typename === 'PullRequest') {
    const prefix = c.__typename === 'PullRequest' ? 'PR' : '';
    return `${prefix}#${c.number}`;
  }
  return item.id;
}
