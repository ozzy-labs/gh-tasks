import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import { ProjectError, type ProjectRef, resolveProjectRef } from '../lib/project.ts';
import { resolveProjectNodeId } from '../lib/projectItem.ts';
import {
  ADD_PROJECT_V2_DRAFT_ISSUE,
  type AddProjectV2DraftIssueResponse,
  CREATE_ISSUE,
  type CreateIssueResponse,
  GET_REPOSITORY_ID,
  type GetRepositoryIdResponse,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope } from '../lib/scope.ts';

export interface AddCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  config?: AppConfig;
}

interface ParsedArgs {
  title: string | null;
  body: string | undefined;
}

export async function add(argv: readonly string[], deps: AddCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const { title, body } = parseArgs(argv);
  if (!title) {
    stderr.write(`${t(locale, 'error.add.titleRequired')}\n`);
    return 2;
  }

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote, config: deps.config });

  if (scope === 'repo') {
    return await addRepoIssue({ argv, deps, locale, stdout, stderr, title, body });
  }

  // org / user scope: add a Projects v2 draft item.
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

  const draftData = await client.request<AddProjectV2DraftIssueResponse>(
    ADD_PROJECT_V2_DRAFT_ISSUE,
    {
      input: {
        projectId,
        title,
        ...(body !== undefined ? { body } : {}),
      },
    }
  );

  stdout.write(
    `${t(locale, 'add.created.project')}: ${draftData.addProjectV2DraftIssue.projectItem.id}\n`
  );
  return 0;
}

interface AddRepoIssueContext {
  argv: readonly string[];
  deps: AddCommandDeps;
  locale: 'ja' | 'en';
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  title: string;
  body: string | undefined;
}

async function addRepoIssue(ctx: AddRepoIssueContext): Promise<number> {
  const { argv, deps, locale, stdout, stderr, title, body } = ctx;
  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const client = deps.client ?? createClient(resolveToken());

  const repoData = await client.request<GetRepositoryIdResponse>(GET_REPOSITORY_ID, {
    owner: repo.owner,
    name: repo.name,
  });
  if (!repoData.repository) {
    stderr.write(`repository not found: ${repo.owner}/${repo.name}\n`);
    return 1;
  }

  const issueData = await client.request<CreateIssueResponse>(CREATE_ISSUE, {
    input: {
      repositoryId: repoData.repository.id,
      title,
      ...(body !== undefined ? { body } : {}),
    },
  });

  stdout.write(`${t(locale, 'add.created.repo')}: ${issueData.createIssue.issue.url}\n`);
  return 0;
}

// Flags that take a separate-arg value (`--flag value` form). Required so
// parseArgs does not consume the value as a positional title.
const VALUE_FLAGS = new Set(['--scope', '--repo', '--lang', '--body', '--project']);

function parseArgs(argv: readonly string[]): ParsedArgs {
  let title: string | null = null;
  let body: string | undefined;
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;

    if (arg.startsWith('--body=')) {
      body = arg.slice('--body='.length);
      continue;
    }
    if (arg === '--body') {
      body = argv[i + 1];
      i++;
      continue;
    }
    if (arg.startsWith('--')) {
      if (!arg.includes('=') && VALUE_FLAGS.has(arg)) {
        // --flag value form: skip the value so it is not parsed as positional
        i++;
      }
      continue;
    }
    if (title === null) {
      title = arg;
    }
  }
  return { title, body };
}
