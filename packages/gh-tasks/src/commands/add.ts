import { resolveLocale, t } from '../i18n/index.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import {
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
}

interface ParsedArgs {
  title: string | null;
  body: string | undefined;
}

export async function add(argv: readonly string[], deps: AddCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv);

  const { title, body } = parseArgs(argv);
  if (!title) {
    stderr.write(`${t(locale, 'error.add.titleRequired')}\n`);
    return 2;
  }

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote });
  if (scope !== 'repo') {
    stderr.write(`${t(locale, 'error.scope.notImplemented')}: --scope ${scope}\n`);
    return 2;
  }

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
const VALUE_FLAGS = new Set(['--scope', '--repo', '--lang', '--body']);

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
