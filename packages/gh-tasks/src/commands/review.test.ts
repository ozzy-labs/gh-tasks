import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { review } from './review.ts';

interface IssueFixture {
  number: number;
  title: string;
  closedAt: string;
}
interface PRFixture {
  number: number;
  title: string;
  mergedAt: string;
}

function makeMockClient(opts: { issues?: IssueFixture[]; prs?: PRFixture[] }): GraphQLClient {
  return {
    async request<T>(query: string): Promise<T> {
      if (query.includes('ListClosedIssues')) {
        return {
          repository: {
            issues: {
              nodes: (opts.issues ?? []).map((i, idx) => ({
                id: `I${idx}`,
                number: i.number,
                title: i.title,
                url: `https://github.com/o/n/issues/${i.number}`,
                closedAt: i.closedAt,
              })),
            },
          },
        } as T;
      }
      if (query.includes('ListMergedPRs')) {
        return {
          repository: {
            pullRequests: {
              nodes: (opts.prs ?? []).map((p, idx) => ({
                id: `P${idx}`,
                number: p.number,
                title: p.title,
                url: `https://github.com/o/n/pull/${p.number}`,
                mergedAt: p.mergedAt,
              })),
            },
          },
        } as T;
      }
      throw new Error(`unexpected query: ${query}`);
    },
  };
}

interface RecordedRequest {
  query: string;
  vars: Record<string, unknown>;
}

interface ProjectMockOptions {
  orgProjectId?: string | null;
  userProjectId?: string | null;
  items?: Array<unknown>;
  nodeMissing?: boolean;
}

/**
 * Build a Projects v2 item fixture covering the field-value shapes that
 * `review` cares about: a Status single-select value (case-variants of "Done"
 * or non-Done statuses) and an `updatedAt` to drive the date-range filter.
 */
function projectIssueItem(args: {
  id: string;
  number: number;
  title: string;
  updatedAt: string;
  status?: string | null;
  contentType?: 'Issue' | 'PullRequest';
}): unknown {
  const fieldValues = args.status
    ? [
        {
          __typename: 'ProjectV2ItemFieldSingleSelectValue',
          optionId: `opt-${args.status.toLowerCase()}`,
          name: args.status,
          field: { id: 'F_status', name: 'Status' },
        },
      ]
    : [];
  const typename = args.contentType ?? 'Issue';
  const baseContent = {
    __typename: typename,
    id: `I_${args.number}`,
    number: args.number,
    title: args.title,
    url:
      typename === 'PullRequest'
        ? `https://github.com/o/n/pull/${args.number}`
        : `https://github.com/o/n/issues/${args.number}`,
    state: typename === 'PullRequest' ? 'MERGED' : 'CLOSED',
    updatedAt: args.updatedAt,
    author: { login: 'o' },
    assignees: { nodes: [] },
  };
  const content =
    typename === 'PullRequest'
      ? { ...baseContent, mergedAt: args.updatedAt }
      : { ...baseContent, closedAt: args.updatedAt };
  return {
    id: args.id,
    updatedAt: args.updatedAt,
    content,
    fieldValues: { nodes: fieldValues },
  };
}

function makeProjectMockClient(
  recorded: RecordedRequest[],
  opts: ProjectMockOptions = {}
): GraphQLClient {
  const orgProjectId = opts.orgProjectId === undefined ? 'PVT_ORG' : opts.orgProjectId;
  const userProjectId = opts.userProjectId === undefined ? 'PVT_USER' : opts.userProjectId;
  const items = opts.items ?? [];

  return {
    async request<T>(query: string, vars: Record<string, unknown> = {}): Promise<T> {
      recorded.push({ query, vars });
      if (query.includes('GetOrgProjectV2')) {
        return {
          organization: orgProjectId
            ? { projectV2: { id: orgProjectId, number: 7, title: 'Org' } }
            : { projectV2: null },
        } as T;
      }
      if (query.includes('GetUserProjectV2')) {
        return {
          user: userProjectId
            ? { projectV2: { id: userProjectId, number: 3, title: 'User' } }
            : { projectV2: null },
        } as T;
      }
      if (query.includes('ListProjectV2Items')) {
        if (opts.nodeMissing) {
          return { node: null } as T;
        }
        return { node: { items: { nodes: items } } } as T;
      }
      throw new Error(`unexpected query: ${query}`);
    },
  };
}

function makeStream(): NodeJS.WritableStream & { written: string } {
  let written = '';
  const stream = {
    write(chunk: string | Buffer): boolean {
      written += chunk.toString();
      return true;
    },
  } as NodeJS.WritableStream & { written: string };
  Object.defineProperty(stream, 'written', { get: () => written });
  return stream;
}

describe('review command', () => {
  const NOW = new Date('2026-05-03T12:00:00Z'); // Sunday

  it('renders Markdown with closed issues and merged PRs in range', async () => {
    const stdout = makeStream();
    const code = await review(
      ['--period=weekly', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient({
          issues: [
            { number: 1, title: 'closed-this-week', closedAt: '2026-04-29T08:00:00Z' },
            { number: 2, title: 'closed-last-week', closedAt: '2026-04-20T08:00:00Z' },
          ],
          prs: [
            { number: 7, title: 'merged-this-week', mergedAt: '2026-04-30T08:00:00Z' },
            { number: 8, title: 'merged-last-week', mergedAt: '2026-04-22T08:00:00Z' },
          ],
        }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('# ');
    expect(stdout.written).toContain('(weekly)');
    expect(stdout.written).toContain('2026-04-27 → 2026-05-04');
    expect(stdout.written).toContain('#1 closed-this-week');
    expect(stdout.written).not.toContain('#2 closed-last-week');
    expect(stdout.written).toContain('#7 merged-this-week');
    expect(stdout.written).not.toContain('#8 merged-last-week');
  });

  it('shows "none" placeholder when both lists are empty', async () => {
    const stdout = makeStream();
    const code = await review(
      ['--period=daily', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient({}),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('(0)');
  });

  it('defaults to weekly when --period is omitted', async () => {
    const stdout = makeStream();
    const code = await review(['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'], {
      client: makeMockClient({}),
      stdout,
      stderr: makeStream(),
      now: () => NOW,
    });
    expect(code).toBe(0);
    expect(stdout.written).toContain('(weekly)');
  });
});

describe('review command (org / user scope)', () => {
  // 2026-05-03 is Sunday. The weekly window is 2026-04-27 .. 2026-05-04 (UTC).
  const NOW = new Date('2026-05-03T12:00:00Z');

  it('aggregates Done items updated within the period (org scope)', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await review(
      ['--period=weekly', '--scope=org', '--project=ozzy-labs/7', '--lang=en'],
      {
        client: makeProjectMockClient(recorded, {
          items: [
            projectIssueItem({
              id: 'PVTI_1',
              number: 11,
              title: 'done-in-range',
              updatedAt: '2026-04-30T08:00:00Z',
              status: 'Done',
            }),
            projectIssueItem({
              id: 'PVTI_2',
              number: 12,
              title: 'done-lowercase',
              updatedAt: '2026-05-02T08:00:00Z',
              status: 'done',
            }),
          ],
        }),
        hasGitRemote: () => false,
        stdout,
        stderr,
        now: () => NOW,
      }
    );

    expect(code).toBe(0);
    expect(stderr.written).toBe('');
    expect(recorded[0]?.query).toContain('GetOrgProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy-labs', number: 7 });
    expect(recorded[1]?.query).toContain('ListProjectV2Items');
    expect(recorded[1]?.vars).toEqual({ projectId: 'PVT_ORG', first: 100 });
    expect(stdout.written).toContain('(weekly)');
    expect(stdout.written).toContain('2026-04-27 → 2026-05-04');
    expect(stdout.written).toContain('(2)');
    expect(stdout.written).toContain('#11 done-in-range');
    expect(stdout.written).toContain('#12 done-lowercase');
  });

  it('excludes items outside the period or with non-Done Status', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const code = await review(
      ['--period=weekly', '--scope=user', '--project=ozzy3/3', '--lang=en'],
      {
        client: makeProjectMockClient(recorded, {
          items: [
            // In-range and Done → kept.
            projectIssueItem({
              id: 'PVTI_keep',
              number: 21,
              title: 'kept',
              updatedAt: '2026-04-30T08:00:00Z',
              status: 'Done',
            }),
            // Out-of-range (last week) but Done → excluded.
            projectIssueItem({
              id: 'PVTI_old',
              number: 22,
              title: 'old-done',
              updatedAt: '2026-04-20T08:00:00Z',
              status: 'Done',
            }),
            // In-range but Status=In Progress → excluded.
            projectIssueItem({
              id: 'PVTI_inprog',
              number: 23,
              title: 'in-progress',
              updatedAt: '2026-05-01T08:00:00Z',
              status: 'In Progress',
            }),
            // In-range but Status unset → excluded.
            projectIssueItem({
              id: 'PVTI_unset',
              number: 24,
              title: 'unset',
              updatedAt: '2026-05-01T08:00:00Z',
              status: null,
            }),
          ],
        }),
        hasGitRemote: () => false,
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );

    expect(code).toBe(0);
    expect(stdout.written).toContain('(1)');
    expect(stdout.written).toContain('#21 kept');
    expect(stdout.written).not.toContain('#22');
    expect(stdout.written).not.toContain('old-done');
    expect(stdout.written).not.toContain('#23');
    expect(stdout.written).not.toContain('in-progress');
    expect(stdout.written).not.toContain('#24');
  });

  it('shows empty-project message when no Done items fall in range', async () => {
    const stdout = makeStream();
    const code = await review(
      ['--period=daily', '--scope=org', '--project=ozzy-labs/7', '--lang=en'],
      {
        client: makeProjectMockClient([], {
          items: [
            projectIssueItem({
              id: 'PVTI_old_done',
              number: 31,
              title: 'old-done',
              updatedAt: '2026-04-01T08:00:00Z',
              status: 'Done',
            }),
          ],
        }),
        hasGitRemote: () => false,
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );

    expect(code).toBe(0);
    expect(stdout.written.toLowerCase()).toContain('no project items');
  });

  it('returns 2 when --scope org is given without a project ref', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await review(['--period=weekly', '--scope=org'], {
      client: makeProjectMockClient(recorded),
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
      now: () => NOW,
    });

    expect(code).toBe(2);
    expect(stderr.written.toLowerCase()).toContain('project');
    // Project ref unresolvable → must not call GraphQL at all.
    expect(recorded).toHaveLength(0);
  });

  it('returns 1 when the org project cannot be resolved', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await review(
      ['--period=weekly', '--scope=org', '--project=ozzy-labs/999', '--lang=en'],
      {
        client: makeProjectMockClient(recorded, { orgProjectId: null }),
        hasGitRemote: () => false,
        stdout: makeStream(),
        stderr,
        now: () => NOW,
      }
    );

    expect(code).toBe(1);
    expect(stderr.written).toContain('project not found');
    expect(recorded).toHaveLength(1);
  });
});
