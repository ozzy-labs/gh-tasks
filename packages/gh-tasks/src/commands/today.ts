import { resolveLocale, t } from '../i18n/index.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import {
  LIST_REPO_ISSUES,
  type ListRepoIssuesResponse,
  type RepoIssueNode,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope } from '../lib/scope.ts';

export interface TodayCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  /** Override of `now` for deterministic testing. */
  now?: () => Date;
}

const FETCH_LIMIT = 100;

export async function today(argv: readonly string[], deps: TodayCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv);

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

  const startOfDay = startOfLocalDay(deps.now ? deps.now() : new Date());
  const todays = data.repository.issues.nodes.filter((i: RepoIssueNode) => {
    const updated = new Date(i.updatedAt);
    return updated.getTime() >= startOfDay.getTime();
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

function startOfLocalDay(d: Date): Date {
  const copy = new Date(d);
  copy.setHours(0, 0, 0, 0);
  return copy;
}
