import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { triage } from './triage.ts';

interface IssueFixture {
  number: number;
  title: string;
  labels: string[];
}

function makeMockClient(issues: IssueFixture[]): GraphQLClient {
  return {
    async request<T>(): Promise<T> {
      return {
        repository: {
          issues: {
            nodes: issues.map((i, idx) => ({
              id: `I${idx}`,
              number: i.number,
              title: i.title,
              url: `https://github.com/o/n/issues/${i.number}`,
              updatedAt: '2026-05-01T00:00:00Z',
              labels: { nodes: i.labels.map((name) => ({ name })) },
            })),
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

/**
 * Default fixture: a representative mix of items with `Status` unset, `Triage`,
 * `In Progress`, and `Done` so each test can pick what it needs.
 */
function defaultProjectItems(): Array<unknown> {
  return [
    {
      id: 'PVTI_unset',
      updatedAt: '2026-05-01T00:00:00Z',
      content: {
        __typename: 'Issue',
        id: 'I_unset',
        number: 101,
        title: 'status-unset',
        url: 'https://github.com/o/n/issues/101',
        state: 'OPEN',
        updatedAt: '2026-05-01T00:00:00Z',
        closedAt: null,
        author: { login: 'o' },
        assignees: { nodes: [] },
      },
      // No Status field value at all.
      fieldValues: { nodes: [] },
    },
    {
      id: 'PVTI_triage',
      updatedAt: '2026-05-01T00:00:00Z',
      content: {
        __typename: 'Issue',
        id: 'I_triage',
        number: 102,
        title: 'status-triage',
        url: 'https://github.com/o/n/issues/102',
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
            optionId: 'opt-triage',
            name: 'Triage',
            field: { id: 'F_status', name: 'Status' },
          },
        ],
      },
    },
    {
      id: 'PVTI_inprogress',
      updatedAt: '2026-05-01T00:00:00Z',
      content: {
        __typename: 'Issue',
        id: 'I_inprogress',
        number: 103,
        title: 'status-in-progress',
        url: 'https://github.com/o/n/issues/103',
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
            optionId: 'opt-doing',
            name: 'In Progress',
            field: { id: 'F_status', name: 'Status' },
          },
        ],
      },
    },
    {
      id: 'PVTI_done',
      updatedAt: '2026-05-01T00:00:00Z',
      content: {
        __typename: 'Issue',
        id: 'I_done',
        number: 104,
        title: 'status-done',
        url: 'https://github.com/o/n/issues/104',
        state: 'CLOSED',
        updatedAt: '2026-05-01T00:00:00Z',
        closedAt: '2026-05-01T00:00:00Z',
        author: { login: 'o' },
        assignees: { nodes: [] },
      },
      fieldValues: {
        nodes: [
          {
            __typename: 'ProjectV2ItemFieldSingleSelectValue',
            optionId: 'opt-done',
            name: 'Done',
            field: { id: 'F_status', name: 'Status' },
          },
        ],
      },
    },
  ];
}

function makeProjectMockClient(
  recorded: RecordedRequest[],
  opts: ProjectMockOptions = {}
): GraphQLClient {
  const orgProjectId = opts.orgProjectId === undefined ? 'PVT_ORG' : opts.orgProjectId;
  const userProjectId = opts.userProjectId === undefined ? 'PVT_USER' : opts.userProjectId;
  const items = opts.items ?? defaultProjectItems();

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

describe('triage command', () => {
  it('lists Issues that have no labels', async () => {
    const stdout = makeStream();
    const code = await triage(['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'], {
      client: makeMockClient([
        { number: 1, title: 'untriaged-a', labels: [] },
        { number: 2, title: 'has-label', labels: ['feat'] },
        { number: 3, title: 'untriaged-b', labels: [] },
      ]),
      stdout,
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(stdout.written).toContain('#1');
    expect(stdout.written).toContain('untriaged-a');
    expect(stdout.written).toContain('#3');
    expect(stdout.written).not.toContain('#2');
    expect(stdout.written).not.toContain('has-label');
  });

  it('honors --limit on the filtered list', async () => {
    const stdout = makeStream();
    const code = await triage(
      ['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--limit=1', '--lang=en'],
      {
        client: makeMockClient([
          { number: 1, title: 'a', labels: [] },
          { number: 2, title: 'b', labels: [] },
          { number: 3, title: 'c', labels: [] },
        ]),
        stdout,
        stderr: makeStream(),
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('#1');
    expect(stdout.written).not.toContain('#2');
  });

  it('shows empty message when everything is triaged', async () => {
    const stdout = makeStream();
    const code = await triage(['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'], {
      client: makeMockClient([{ number: 1, title: 'has-label', labels: ['feat'] }]),
      stdout,
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(stdout.written).toMatch(/(no untriaged|未トリアージ)/i);
  });

  it('extracts items whose Status field is unset (org scope)', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();
    const code = await triage(['--scope=org', '--project=ozzy-labs/7', '--lang=en'], {
      client: makeProjectMockClient(recorded),
      hasGitRemote: () => false,
      stdout,
      stderr,
    });

    expect(code).toBe(0);
    expect(stderr.written).toBe('');
    expect(recorded[0]?.query).toContain('GetOrgProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy-labs', number: 7 });
    expect(recorded[1]?.query).toContain('ListProjectV2Items');
    expect(recorded[1]?.vars).toEqual({ projectId: 'PVT_ORG', first: 100 });
    // Status-unset item shows up.
    expect(stdout.written).toContain('#101');
    expect(stdout.written).toContain('status-unset');
  });

  it('extracts items whose Status is "Triage" (case-insensitive)', async () => {
    const stdout = makeStream();
    const code = await triage(['--scope=user', '--project=ozzy3/3', '--lang=en'], {
      client: makeProjectMockClient([], {
        items: [
          {
            id: 'PVTI_lower',
            updatedAt: '2026-05-01T00:00:00Z',
            content: {
              __typename: 'Issue',
              id: 'I1',
              number: 201,
              title: 'lower-triage',
              url: 'https://github.com/o/n/issues/201',
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
                  optionId: 'opt-triage-lower',
                  name: 'triage',
                  field: { id: 'F_status', name: 'Status' },
                },
              ],
            },
          },
          {
            id: 'PVTI_upper',
            updatedAt: '2026-05-01T00:00:00Z',
            content: {
              __typename: 'Issue',
              id: 'I2',
              number: 202,
              title: 'upper-triage',
              url: 'https://github.com/o/n/issues/202',
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
                  optionId: 'opt-triage-upper',
                  name: 'TRIAGE',
                  field: { id: 'F_status', name: 'Status' },
                },
              ],
            },
          },
        ],
      }),
      hasGitRemote: () => false,
      stdout,
      stderr: makeStream(),
    });

    expect(code).toBe(0);
    expect(stdout.written).toContain('#201');
    expect(stdout.written).toContain('lower-triage');
    expect(stdout.written).toContain('#202');
    expect(stdout.written).toContain('upper-triage');
  });

  it('excludes items in "In Progress" / "Done" Status', async () => {
    const stdout = makeStream();
    const code = await triage(['--scope=org', '--project=ozzy-labs/7', '--lang=en'], {
      client: makeProjectMockClient([]),
      hasGitRemote: () => false,
      stdout,
      stderr: makeStream(),
    });

    expect(code).toBe(0);
    // Untriaged ones are present.
    expect(stdout.written).toContain('#101');
    expect(stdout.written).toContain('#102');
    // In-progress / done ones must be filtered out.
    expect(stdout.written).not.toContain('#103');
    expect(stdout.written).not.toContain('status-in-progress');
    expect(stdout.written).not.toContain('#104');
    expect(stdout.written).not.toContain('status-done');
  });

  it('shows empty-project message when no items qualify', async () => {
    const stdout = makeStream();
    const code = await triage(['--scope=org', '--project=ozzy-labs/7', '--lang=en'], {
      client: makeProjectMockClient([], {
        items: [
          {
            id: 'PVTI_only_done',
            updatedAt: '2026-05-01T00:00:00Z',
            content: {
              __typename: 'Issue',
              id: 'I_done',
              number: 301,
              title: 'all-done',
              url: 'https://github.com/o/n/issues/301',
              state: 'CLOSED',
              updatedAt: '2026-05-01T00:00:00Z',
              closedAt: '2026-05-01T00:00:00Z',
              author: { login: 'o' },
              assignees: { nodes: [] },
            },
            fieldValues: {
              nodes: [
                {
                  __typename: 'ProjectV2ItemFieldSingleSelectValue',
                  optionId: 'opt-done',
                  name: 'Done',
                  field: { id: 'F_status', name: 'Status' },
                },
              ],
            },
          },
        ],
      }),
      hasGitRemote: () => false,
      stdout,
      stderr: makeStream(),
    });

    expect(code).toBe(0);
    expect(stdout.written.toLowerCase()).toContain('no untriaged');
  });

  it('returns 2 when --scope org is given without a project ref', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await triage(['--scope=org'], {
      client: makeProjectMockClient(recorded),
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
    });

    expect(code).toBe(2);
    expect(stderr.written.toLowerCase()).toContain('project');
    expect(recorded).toHaveLength(0);
  });

  it('returns 1 when the org project cannot be resolved', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await triage(['--scope=org', '--project=ozzy-labs/999'], {
      client: makeProjectMockClient(recorded, { orgProjectId: null }),
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
    });

    expect(code).toBe(1);
    expect(stderr.written).toContain('project not found');
    expect(recorded).toHaveLength(1);
  });
});
