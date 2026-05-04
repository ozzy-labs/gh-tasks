import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import { ProjectError, type ProjectRef, resolveProjectRef } from '../lib/project.ts';
import { findStatus, formatItem, resolveProjectNodeId } from '../lib/projectItem.ts';
import {
  LIST_PROJECT_V2_ITEMS,
  LIST_REPO_ISSUES_WITH_LABELS,
  type ListProjectV2ItemsResponse,
  type ListRepoIssuesWithLabelsResponse,
  type ProjectV2ItemNode,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope, type Scope } from '../lib/scope.ts';

export interface TriageCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  config?: AppConfig;
}

const DEFAULT_LIMIT = 20;
const FETCH_LIMIT = 100;

export async function triage(
  argv: readonly string[],
  deps: TriageCommandDeps = {}
): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote, config: deps.config });
  const limit = parseLimit(argv) ?? DEFAULT_LIMIT;

  if (scope === 'repo') {
    return await triageRepo({ argv, deps, locale, stdout, stderr, limit });
  }

  return await triageProject({ argv, deps, locale, scope, stdout, stderr, limit });
}

interface TriageRepoContext {
  argv: readonly string[];
  deps: TriageCommandDeps;
  locale: 'ja' | 'en';
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  limit: number;
}

async function triageRepo(ctx: TriageRepoContext): Promise<number> {
  const { argv, deps, locale, stdout, stderr, limit } = ctx;
  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const client = deps.client ?? createClient(resolveToken());

  const data = await client.request<ListRepoIssuesWithLabelsResponse>(
    LIST_REPO_ISSUES_WITH_LABELS,
    { owner: repo.owner, name: repo.name, first: FETCH_LIMIT }
  );
  if (!data.repository) {
    stderr.write(`repository not found: ${repo.owner}/${repo.name}\n`);
    return 1;
  }

  const untriaged = data.repository.issues.nodes
    .filter((i) => i.labels.nodes.length === 0)
    .slice(0, limit);

  if (untriaged.length === 0) {
    stdout.write(`${t(locale, 'triage.empty')}\n`);
    return 0;
  }

  stdout.write(`${t(locale, 'triage.found')} (${untriaged.length})\n`);
  for (const issue of untriaged) {
    stdout.write(`#${issue.number}  ${issue.title}\n  ${issue.url}\n`);
  }
  return 0;
}

interface TriageProjectContext {
  argv: readonly string[];
  deps: TriageCommandDeps;
  locale: 'ja' | 'en';
  scope: Exclude<Scope, 'repo'>;
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  limit: number;
}

async function triageProject(ctx: TriageProjectContext): Promise<number> {
  const { argv, deps, locale, scope, stdout, stderr, limit } = ctx;

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

  const untriaged = data.node.items.nodes.filter(isUntriaged).slice(0, limit);

  if (untriaged.length === 0) {
    stdout.write(`${t(locale, 'triage.empty.project')}\n`);
    return 0;
  }

  stdout.write(`${t(locale, 'triage.found.project')} (${untriaged.length})\n`);
  for (const item of untriaged) {
    stdout.write(formatItem(item));
  }
  return 0;
}

/**
 * An item is considered untriaged when its `Status` field is unset, or when the
 * value is the literal `Triage` (case-insensitive).
 */
function isUntriaged(item: ProjectV2ItemNode): boolean {
  const status = findStatus(item.fieldValues.nodes);
  if (status === null) return true;
  return status.toLowerCase() === 'triage';
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
