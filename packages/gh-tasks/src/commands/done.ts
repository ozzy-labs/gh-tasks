import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import {
  CLOSE_ISSUE,
  type CloseIssueResponse,
  GET_ISSUE_BY_NUMBER,
  type GetIssueByNumberResponse,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope } from '../lib/scope.ts';

export interface DoneCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  config?: AppConfig;
}

export async function done(argv: readonly string[], deps: DoneCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const id = parseIssueId(argv);
  if (id === null) {
    stderr.write(`${t(locale, 'error.done.idRequired')}\n`);
    return 2;
  }

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote, config: deps.config });
  if (scope !== 'repo') {
    stderr.write(`${t(locale, 'error.scope.notImplemented')}: --scope ${scope}\n`);
    return 2;
  }

  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const client = deps.client ?? createClient(resolveToken());

  const issueData = await client.request<GetIssueByNumberResponse>(GET_ISSUE_BY_NUMBER, {
    owner: repo.owner,
    name: repo.name,
    number: id,
  });
  const issue = issueData.repository?.issue;
  if (!issue) {
    stderr.write(`Issue not found: ${repo.owner}/${repo.name}#${id}\n`);
    return 1;
  }
  if (issue.state === 'CLOSED') {
    stdout.write(`${t(locale, 'done.alreadyClosed')}: ${issue.url}\n`);
    return 0;
  }

  const closed = await client.request<CloseIssueResponse>(CLOSE_ISSUE, {
    input: { issueId: issue.id },
  });
  stdout.write(`${t(locale, 'done.closed')}: ${closed.closeIssue.issue.url}\n`);
  return 0;
}

function parseIssueId(argv: readonly string[]): number | null {
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;
    if (arg.startsWith('--')) {
      // skip value-flags' values
      if (!arg.includes('=') && (arg === '--scope' || arg === '--repo' || arg === '--lang')) {
        i++;
      }
      continue;
    }
    const stripped = arg.startsWith('#') ? arg.slice(1) : arg;
    const n = Number.parseInt(stripped, 10);
    if (Number.isFinite(n) && n > 0) return n;
  }
  return null;
}
