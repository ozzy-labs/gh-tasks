import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import { ProjectError, type ProjectRef, resolveProjectRef } from '../lib/project.ts';
import { formatItem, resolveProjectNodeId } from '../lib/projectItem.ts';
import {
  LIST_PROJECT_V2_ITEMS,
  LIST_REPO_ISSUES,
  type ListProjectV2ItemsResponse,
  type ListRepoIssuesResponse,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope, type Scope } from '../lib/scope.ts';

export interface ListCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  config?: AppConfig;
}

const DEFAULT_LIMIT = 30;

export async function list(argv: readonly string[], deps: ListCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote, config: deps.config });
  const limit = parseLimit(argv) ?? DEFAULT_LIMIT;

  if (scope === 'repo') {
    return await listRepoIssues({ argv, deps, locale, stdout, stderr, limit });
  }

  return await listProjectItems({ argv, deps, locale, scope, stdout, stderr, limit });
}

interface ListRepoContext {
  argv: readonly string[];
  deps: ListCommandDeps;
  locale: 'ja' | 'en';
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  limit: number;
}

async function listRepoIssues(ctx: ListRepoContext): Promise<number> {
  const { argv, deps, locale, stdout, stderr, limit } = ctx;
  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const client = deps.client ?? createClient(resolveToken());

  const data = await client.request<ListRepoIssuesResponse>(LIST_REPO_ISSUES, {
    owner: repo.owner,
    name: repo.name,
    first: limit,
  });
  if (!data.repository) {
    stderr.write(`repository not found: ${repo.owner}/${repo.name}\n`);
    return 1;
  }

  const issues = data.repository.issues.nodes;
  if (issues.length === 0) {
    stdout.write(`${t(locale, 'list.empty')}\n`);
    return 0;
  }
  for (const issue of issues) {
    stdout.write(`#${issue.number}  ${issue.title}\n  ${issue.url}\n`);
  }
  return 0;
}

interface ListProjectContext {
  argv: readonly string[];
  deps: ListCommandDeps;
  locale: 'ja' | 'en';
  scope: Exclude<Scope, 'repo'>;
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  limit: number;
}

async function listProjectItems(ctx: ListProjectContext): Promise<number> {
  const { argv, deps, locale, scope, stdout, stderr, limit } = ctx;

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
    first: limit,
  });
  if (!data.node) {
    stderr.write(
      `project not found: ${projectRef.owner}/${projectRef.number} (--scope ${scope})\n`
    );
    return 1;
  }

  const items = data.node.items.nodes;
  if (items.length === 0) {
    stdout.write(`${t(locale, 'list.empty.project')}\n`);
    return 0;
  }

  for (const item of items) {
    stdout.write(formatItem(item));
  }
  return 0;
}

function parseLimit(argv: readonly string[]): number | null {
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;
    if (arg.startsWith('--limit=')) {
      return toLimit(arg.slice('--limit='.length));
    }
    if (arg === '--limit') {
      return toLimit(argv[i + 1]);
    }
  }
  return null;
}

function toLimit(value: string | undefined): number | null {
  if (value === undefined) return null;
  const n = Number.parseInt(value, 10);
  return Number.isFinite(n) && n > 0 ? n : null;
}
