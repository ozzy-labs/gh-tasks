import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import { ProjectError, type ProjectRef, resolveProjectRef } from '../lib/project.ts';
import { resolveProjectNodeId } from '../lib/projectItem.ts';
import {
  ADD_PROJECT_V2_ITEM_BY_ID,
  type AddProjectV2ItemByIdResponse,
  GET_ISSUE_BY_NUMBER,
  GET_PULL_REQUEST_BY_NUMBER,
  type GetIssueByNumberResponse,
  type GetPullRequestByNumberResponse,
  UPDATE_PULL_REQUEST,
  type UpdatePullRequestResponse,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope, type Scope } from '../lib/scope.ts';

export interface LinkCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  config?: AppConfig;
}

export async function link(argv: readonly string[], deps: LinkCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const positionals = parsePositionalNumbers(argv);
  if (positionals.length < 2) {
    stderr.write(`${t(locale, 'error.link.argsRequired')}\n`);
    return 2;
  }
  const [pr, task] = positionals as [number, number, ...number[]];

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote, config: deps.config });

  if (scope === 'repo') {
    return await linkRepoPr({ argv, deps, locale, stdout, stderr, pr, task });
  }

  return await linkProjectItems({ argv, deps, locale, scope, stdout, stderr, pr, task });
}

interface LinkRepoContext {
  argv: readonly string[];
  deps: LinkCommandDeps;
  locale: 'ja' | 'en';
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  pr: number;
  task: number;
}

async function linkRepoPr(ctx: LinkRepoContext): Promise<number> {
  const { argv, deps, locale, stdout, stderr, pr, task } = ctx;

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

interface LinkProjectContext {
  argv: readonly string[];
  deps: LinkCommandDeps;
  locale: 'ja' | 'en';
  scope: Exclude<Scope, 'repo'>;
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  pr: number;
  task: number;
}

/**
 * org / user scope: surface both the PR and the task Issue on the same
 * Projects v2 board. Project v2 has no dedicated "linkedPullRequests"
 * mutation — GitHub derives Issue ↔ PR relations from the repo-level
 * `Closes #N` keyword. Adding both content nodes to the same project means
 * (a) the project view shows them together, and (b) the existing
 * cross-reference relation is the linkage.
 *
 * `addProjectV2ItemById` is idempotent: if the content is already an item on
 * the project the mutation returns that item without erroring, so it is safe
 * to call twice unconditionally.
 *
 * The `<pr>` / `<task>` positionals are repo-scoped numbers (same shape as
 * repo scope). The owning repo is resolved from `--repo` or `git remote
 * origin` so we can look up the content node ids.
 */
async function linkProjectItems(ctx: LinkProjectContext): Promise<number> {
  const { argv, deps, locale, scope, stdout, stderr, pr, task } = ctx;

  let projectRef: ProjectRef;
  try {
    projectRef = resolveProjectRef({ scope, argv, config: deps.config });
  } catch (err) {
    if (err instanceof ProjectError) {
      stderr.write(`${t(locale, err.i18nKey, err.i18nArgs)}\n`);
      return 2;
    }
    throw err;
  }

  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const client = deps.client ?? createClient(resolveToken());

  const projectId = await resolveProjectNodeId({ client, scope, projectRef });
  if (!projectId) {
    stderr.write(
      `project not found: ${projectRef.owner}/${projectRef.number} (--scope ${scope})\n`
    );
    return 1;
  }

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

  const issueData = await client.request<GetIssueByNumberResponse>(GET_ISSUE_BY_NUMBER, {
    owner: repo.owner,
    name: repo.name,
    number: task,
  });
  const issueNode = issueData.repository?.issue;
  if (!issueNode) {
    stderr.write(`Issue not found: ${repo.owner}/${repo.name}#${task}\n`);
    return 1;
  }

  await client.request<AddProjectV2ItemByIdResponse>(ADD_PROJECT_V2_ITEM_BY_ID, {
    input: { projectId, contentId: prNode.id },
  });
  await client.request<AddProjectV2ItemByIdResponse>(ADD_PROJECT_V2_ITEM_BY_ID, {
    input: { projectId, contentId: issueNode.id },
  });

  stdout.write(`${t(locale, 'link.added.project')}: ${prNode.url} ↔ ${issueNode.url}\n`);
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
      if (
        !arg.includes('=') &&
        (arg === '--scope' || arg === '--repo' || arg === '--lang' || arg === '--project')
      ) {
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
