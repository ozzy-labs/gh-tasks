import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { list } from './list.ts';

interface RecordedRequest {
  query: string;
  vars: Record<string, unknown>;
}

function makeRepoMockClient(
  recorded: RecordedRequest[],
  issues: Array<{ number: number; title: string; url: string }> = [
    { number: 1, title: 'first', url: 'https://github.com/o/n/issues/1' },
    { number: 2, title: 'second', url: 'https://github.com/o/n/issues/2' },
  ]
): GraphQLClient {
  return {
    async request<T>(query: string, vars: Record<string, unknown> = {}): Promise<T> {
      recorded.push({ query, vars });
      return {
        repository: {
          issues: {
            nodes: issues.map((i, idx) => ({
              id: `I${idx}`,
              number: i.number,
              title: i.title,
              url: i.url,
              updatedAt: '2026-05-01T00:00:00Z',
            })),
          },
        },
      } as T;
    },
  };
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
  const items = opts.items ?? [
    {
      id: 'PVTI_1',
      updatedAt: '2026-05-01T00:00:00Z',
      content: {
        __typename: 'Issue',
        id: 'I1',
        number: 11,
        title: 'real issue',
        url: 'https://github.com/o/n/issues/11',
        state: 'OPEN',
        updatedAt: '2026-05-01T00:00:00Z',
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
      updatedAt: '2026-05-02T00:00:00Z',
      content: {
        __typename: 'PullRequest',
        id: 'PR1',
        number: 22,
        title: 'a pr',
        url: 'https://github.com/o/n/pull/22',
        state: 'OPEN',
        updatedAt: '2026-05-02T00:00:00Z',
        mergedAt: null,
        author: { login: 'o' },
        assignees: { nodes: [] },
      },
      fieldValues: { nodes: [] },
    },
    {
      id: 'PVTI_3',
      updatedAt: '2026-05-03T00:00:00Z',
      content: {
        __typename: 'DraftIssue',
        id: 'DI1',
        title: 'a draft',
        body: null,
      },
      fieldValues: {
        nodes: [
          {
            __typename: 'ProjectV2ItemFieldSingleSelectValue',
            optionId: 'opt-doing',
            name: 'In progress',
            field: { id: 'F_status', name: 'Status' },
          },
        ],
      },
    },
  ];

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

describe('list command', () => {
  it('prints open issues for repo scope', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const code = await list(['--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeRepoMockClient(recorded),
      stdout,
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(stdout.written).toContain('#1');
    expect(stdout.written).toContain('first');
    expect(stdout.written).toContain('https://github.com/o/n/issues/2');
    expect(recorded[0]?.vars).toEqual({ owner: 'ozzy-labs', name: 'gh-tasks', first: 30 });
  });

  it('honors --limit', async () => {
    const recorded: RecordedRequest[] = [];
    const code = await list(['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--limit=5'], {
      client: makeRepoMockClient(recorded),
      stdout: makeStream(),
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(recorded[0]?.vars).toEqual({ owner: 'ozzy-labs', name: 'gh-tasks', first: 5 });
  });

  it('shows empty message when there are no issues', async () => {
    const stdout = makeStream();
    const code = await list(['--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeRepoMockClient([], []),
      stdout,
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(stdout.written).toMatch(/(オープン|open)/i);
  });

  it('lists Project v2 items in org scope (Issue / PR / DraftIssue)', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await list(['--scope=org', '--project=ozzy-labs/7', '--lang=en'], {
      client: makeProjectMockClient(recorded),
      hasGitRemote: () => false,
      stdout,
      stderr,
    });

    expect(code).toBe(0);
    expect(stderr.written).toBe('');
    expect(recorded).toHaveLength(2);
    expect(recorded[0]?.query).toContain('GetOrgProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy-labs', number: 7 });
    expect(recorded[1]?.query).toContain('ListProjectV2Items');
    expect(recorded[1]?.vars).toEqual({ projectId: 'PVT_ORG', first: 30 });

    expect(stdout.written).toContain('#11');
    expect(stdout.written).toContain('real issue');
    expect(stdout.written).toContain('[Todo]');
    expect(stdout.written).toContain('PR#22');
    expect(stdout.written).toContain('a pr');
    expect(stdout.written).toContain('(draft)');
    expect(stdout.written).toContain('a draft');
    expect(stdout.written).toContain('[In progress]');
  });

  it('lists Project v2 items in user scope and honors --limit', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();

    const code = await list(['--scope=user', '--project=ozzy3/3', '--limit=10', '--lang=en'], {
      client: makeProjectMockClient(recorded),
      hasGitRemote: () => false,
      stdout,
      stderr: makeStream(),
    });

    expect(code).toBe(0);
    expect(recorded[0]?.query).toContain('GetUserProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy3', number: 3 });
    expect(recorded[1]?.query).toContain('ListProjectV2Items');
    expect(recorded[1]?.vars).toEqual({ projectId: 'PVT_USER', first: 10 });
  });

  it('returns 2 when --scope org is given without a project ref', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await list(['--scope=org'], {
      client: makeProjectMockClient(recorded),
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
    });

    expect(code).toBe(2);
    expect(stderr.written.toLowerCase()).toContain('project');
    // No GraphQL request should have been issued before bailing out.
    expect(recorded).toHaveLength(0);
  });

  it('returns 1 when the org project cannot be resolved', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await list(['--scope=org', '--project=ozzy-labs/999'], {
      client: makeProjectMockClient(recorded, { orgProjectId: null }),
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
    });

    expect(code).toBe(1);
    expect(stderr.written).toContain('project not found');
    expect(recorded).toHaveLength(1);
  });

  it('shows empty-project message when items are empty', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const code = await list(['--scope=org', '--project=ozzy-labs/7', '--lang=en'], {
      client: makeProjectMockClient(recorded, { items: [] }),
      hasGitRemote: () => false,
      stdout,
      stderr: makeStream(),
    });

    expect(code).toBe(0);
    expect(stdout.written.toLowerCase()).toContain('no items');
  });
});
