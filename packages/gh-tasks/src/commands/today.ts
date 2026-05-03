import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import { ProjectError, type ProjectRef, resolveProjectRef } from '../lib/project.ts';
import {
  GET_ORG_PROJECT_V2,
  GET_USER_PROJECT_V2,
  type GetOrgProjectV2Response,
  type GetUserProjectV2Response,
  LIST_PROJECT_V2_ITEMS,
  LIST_REPO_ISSUES,
  type ListProjectV2ItemsResponse,
  type ListRepoIssuesResponse,
  type ProjectV2FieldValue,
  type ProjectV2ItemNode,
  type RepoIssueNode,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope, type Scope } from '../lib/scope.ts';

export interface TodayCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  /** Override of `now` for deterministic testing. */
  now?: () => Date;
  config?: AppConfig;
}

const FETCH_LIMIT = 100;

export async function today(argv: readonly string[], deps: TodayCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote, config: deps.config });
  const now = deps.now ? deps.now() : new Date();

  if (scope === 'repo') {
    return await todayRepo({ argv, deps, locale, stdout, stderr, now });
  }

  return await todayProject({ argv, deps, locale, scope, stdout, stderr, now });
}

interface TodayRepoContext {
  argv: readonly string[];
  deps: TodayCommandDeps;
  locale: 'ja' | 'en';
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  now: Date;
}

async function todayRepo(ctx: TodayRepoContext): Promise<number> {
  const { argv, deps, locale, stdout, stderr, now } = ctx;
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

  const { start, end } = todayRange(now);
  const todays = data.repository.issues.nodes.filter((i: RepoIssueNode) => {
    const updated = new Date(i.updatedAt).getTime();
    return updated >= start && updated < end;
  });

  if (todays.length === 0) {
    stdout.write(`${t(locale, 'today.empty')}\n`);
    return 0;
  }
  for (const issue of todays) {
    stdout.write(`#${issue.number}  ${issue.title}\n  ${issue.url}\n`);
  }
  return 0;
}

interface TodayProjectContext {
  argv: readonly string[];
  deps: TodayCommandDeps;
  locale: 'ja' | 'en';
  scope: Exclude<Scope, 'repo'>;
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  now: Date;
}

async function todayProject(ctx: TodayProjectContext): Promise<number> {
  const { argv, deps, locale, scope, stdout, stderr, now } = ctx;

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

  const { start, end } = todayRange(now);
  const items = data.node.items.nodes.filter((item: ProjectV2ItemNode) => {
    const updated = new Date(item.updatedAt).getTime();
    return updated >= start && updated < end;
  });

  if (items.length === 0) {
    stdout.write(`${t(locale, 'today.empty.project')}\n`);
    return 0;
  }

  for (const item of items) {
    stdout.write(formatItem(item));
  }
  return 0;
}

function formatItem(item: ProjectV2ItemNode): string {
  const status = findStatus(item.fieldValues.nodes);
  const statusSuffix = status ? `  [${status}]` : '';
  const content = item.content;
  if (!content) {
    return `(no content)${statusSuffix}\n`;
  }
  if (content.__typename === 'Issue' || content.__typename === 'PullRequest') {
    const prefix = content.__typename === 'PullRequest' ? 'PR' : '';
    return `${prefix}#${content.number}  ${content.title}${statusSuffix}\n  ${content.url}\n`;
  }
  // DraftIssue: no number/url.
  return `(draft)  ${content.title}${statusSuffix}\n`;
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

/**
 * "Today" is anchored at UTC midnight so the result is identical on a JST
 * dev box and a UTC CI runner. Returns `[start, end)` in epoch milliseconds.
 */
function todayRange(now: Date): { start: number; end: number } {
  const start = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
  const end = start + 24 * 60 * 60 * 1000;
  return { start, end };
}
