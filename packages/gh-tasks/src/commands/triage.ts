import { resolveLocale, t } from '../i18n/index.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import {
  LIST_REPO_ISSUES_WITH_LABELS,
  type ListRepoIssuesWithLabelsResponse,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope } from '../lib/scope.ts';

export interface TriageCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
}

const DEFAULT_LIMIT = 20;
const FETCH_LIMIT = 100;

export async function triage(
  argv: readonly string[],
  deps: TriageCommandDeps = {}
): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv);

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote });
  if (scope !== 'repo') {
    stderr.write(`${t(locale, 'error.scope.notImplemented')}: --scope ${scope}\n`);
    return 2;
  }

  const limit = parseLimit(argv) ?? DEFAULT_LIMIT;
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
