import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { today } from './today.ts';

interface IssueFixture {
  number: number;
  title: string;
  url: string;
  updatedAt: string;
}

function makeRepoMockClient(issues: IssueFixture[]): GraphQLClient {
  return {
    async request<T>(): Promise<T> {
      return {
        repository: {
          issues: {
            nodes: issues.map((i, idx) => ({ id: `I${idx}`, ...i })),
          },
        },
      } as T;
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

describe('today command', () => {
  // Use UTC-anchored timestamps so the test is TZ-independent across the
  // local dev box (often JST) and the CI runner (UTC).
  const NOW = new Date('2026-05-03T12:00:00Z');

  it('shows issues updated today', async () => {
    const stdout = makeStream();
    const code = await today(['--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeRepoMockClient([
        {
          number: 1,
          title: 'updated today',
          url: 'https://github.com/o/n/issues/1',
          updatedAt: '2026-05-03T11:00:00Z',
        },
        {
          number: 2,
          title: 'old issue',
          url: 'https://github.com/o/n/issues/2',
          updatedAt: '2026-04-30T08:00:00Z',
        },
      ]),
      stdout,
      stderr: makeStream(),
      now: () => NOW,
    });
    expect(code).toBe(0);
    expect(stdout.written).toContain('#1');
    expect(stdout.written).toContain('updated today');
    expect(stdout.written).not.toContain('#2');
    expect(stdout.written).not.toContain('old issue');
  });

  it('shows empty message when nothing was updated today', async () => {
    const stdout = makeStream();
    const code = await today(['--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeRepoMockClient([
        {
          number: 1,
          title: 'old',
          url: 'https://github.com/o/n/issues/1',
          updatedAt: '2026-04-30T08:00:00Z',
        },
      ]),
      stdout,
      stderr: makeStream(),
      now: () => NOW,
    });
    expect(code).toBe(0);
    expect(stdout.written).toMatch(/(該当|今日|today)/i);
  });

  it('shows project items updated today in org scope', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const items = [
      {
        id: 'PVTI_1',
        // updatedAt within today (UTC range starts at 2026-05-03T00:00:00Z)
        updatedAt: '2026-05-03T09:00:00Z',
        content: {
          __typename: 'Issue',
          id: 'I1',
          number: 11,
          title: 'today issue',
          url: 'https://github.com/o/n/issues/11',
          state: 'OPEN',
          updatedAt: '2026-05-03T09:00:00Z',
          closedAt: null,
          author: { login: 'o' },
          assignees: { nodes: [] },
        },
        fieldValues: {
          nodes: [
            {
              __typename: 'ProjectV2ItemFieldSingleSelectValue',
              optionId: 'opt-todo',
              name: 'Todo',
              field: { id: 'F_status', name: 'Status' },
            },
          ],
        },
      },
      {
        id: 'PVTI_2',
        // updatedAt yesterday — must be filtered out
        updatedAt: '2026-05-02T09:00:00Z',
        content: {
          __typename: 'PullRequest',
          id: 'PR1',
          number: 22,
          title: 'old pr',
          url: 'https://github.com/o/n/pull/22',
          state: 'OPEN',
          updatedAt: '2026-05-02T09:00:00Z',
          mergedAt: null,
          author: { login: 'o' },
          assignees: { nodes: [] },
        },
        fieldValues: { nodes: [] },
      },
    ];

    const code = await today(['--scope=org', '--project=ozzy-labs/7', '--lang=en'], {
      client: makeProjectMockClient(recorded, { items }),
      hasGitRemote: () => false,
      stdout,
      stderr,
      now: () => NOW,
    });

    expect(code).toBe(0);
    expect(stderr.written).toBe('');
    expect(recorded).toHaveLength(2);
    expect(recorded[0]?.query).toContain('GetOrgProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy-labs', number: 7 });
    expect(recorded[1]?.query).toContain('ListProjectV2Items');
    expect(recorded[1]?.vars).toEqual({ projectId: 'PVT_ORG', first: 100 });

    expect(stdout.written).toContain('#11');
    expect(stdout.written).toContain('today issue');
    expect(stdout.written).toContain('[Todo]');
    expect(stdout.written).not.toContain('#22');
    expect(stdout.written).not.toContain('old pr');
  });

  it('shows empty message when no project items were updated today (user scope)', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();

    const items = [
      {
        id: 'PVTI_1',
        updatedAt: '2026-04-30T09:00:00Z',
        content: {
          __typename: 'Issue',
          id: 'I1',
          number: 9,
          title: 'old',
          url: 'https://github.com/o/n/issues/9',
          state: 'OPEN',
          updatedAt: '2026-04-30T09:00:00Z',
          closedAt: null,
          author: { login: 'o' },
          assignees: { nodes: [] },
        },
        fieldValues: { nodes: [] },
      },
    ];

    const code = await today(['--scope=user', '--project=ozzy3/3', '--lang=en'], {
      client: makeProjectMockClient(recorded, { items }),
      hasGitRemote: () => false,
      stdout,
      stderr: makeStream(),
      now: () => NOW,
    });

    expect(code).toBe(0);
    expect(recorded[0]?.query).toContain('GetUserProjectV2');
    expect(stdout.written.toLowerCase()).toContain('no project items');
  });

  it('returns 2 when --scope org is given without a project ref', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await today(['--scope=org'], {
      client: makeProjectMockClient(recorded),
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
      now: () => NOW,
    });

    expect(code).toBe(2);
    expect(stderr.written.toLowerCase()).toContain('project');
    // No GraphQL request should have been issued before bailing out.
    expect(recorded).toHaveLength(0);
  });
});
