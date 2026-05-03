import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { done } from './done.ts';

interface RecordedRequest {
  query: string;
  vars: Record<string, unknown>;
}

function makeMockClient(
  recorded: RecordedRequest[],
  options: { state?: 'OPEN' | 'CLOSED'; missing?: boolean } = {}
): GraphQLClient {
  return {
    async request<T>(query: string, vars: Record<string, unknown> = {}): Promise<T> {
      recorded.push({ query, vars });
      if (query.includes('GetIssueByNumber')) {
        if (options.missing) {
          return { repository: { issue: null } } as T;
        }
        return {
          repository: {
            issue: {
              id: 'I_42',
              number: 42,
              url: 'https://github.com/ozzy-labs/gh-tasks/issues/42',
              state: options.state ?? 'OPEN',
            },
          },
        } as T;
      }
      if (query.includes('closeIssue')) {
        return {
          closeIssue: {
            issue: {
              id: 'I_42',
              number: 42,
              url: 'https://github.com/ozzy-labs/gh-tasks/issues/42',
              state: 'CLOSED',
            },
          },
        } as T;
      }
      throw new Error(`unexpected query: ${query}`);
    },
  };
}

interface ProjectMockOptions {
  orgProjectId?: string | null;
  userProjectId?: string | null;
  /** Items returned by ListProjectV2Items. Defaults to a single in-progress item. */
  items?: Array<unknown>;
  /** Field nodes returned by ListProjectV2Fields. Defaults to a Status SINGLE_SELECT with Todo/In Progress/Done. */
  fields?: Array<unknown>;
}

function defaultStatusFieldNodes(): Array<unknown> {
  return [
    {
      id: 'F_status',
      name: 'Status',
      dataType: 'SINGLE_SELECT',
      options: [
        { id: 'opt-todo', name: 'Todo' },
        { id: 'opt-in-progress', name: 'In Progress' },
        { id: 'opt-done', name: 'Done' },
      ],
    },
  ];
}

function makeProjectMockClient(
  recorded: RecordedRequest[],
  opts: ProjectMockOptions = {}
): GraphQLClient {
  const orgProjectId = opts.orgProjectId === undefined ? 'PVT_ORG' : opts.orgProjectId;
  const userProjectId = opts.userProjectId === undefined ? 'PVT_USER' : opts.userProjectId;
  const items = opts.items ?? [];
  const fields = opts.fields ?? defaultStatusFieldNodes();

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
      if (query.includes('ListProjectV2Fields')) {
        return { node: { fields: { nodes: fields } } } as T;
      }
      if (query.includes('ListProjectV2Items')) {
        return { node: { items: { nodes: items } } } as T;
      }
      if (query.includes('updateProjectV2ItemFieldValue')) {
        return { updateProjectV2ItemFieldValue: { projectV2Item: { id: 'PVTI_1' } } } as T;
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

function inProgressItem(id = 'PVTI_1'): unknown {
  return {
    id,
    updatedAt: '2026-05-03T09:00:00Z',
    content: {
      __typename: 'Issue',
      id: 'I1',
      number: 11,
      title: 'in progress',
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
          optionId: 'opt-in-progress',
          name: 'In Progress',
          field: { id: 'F_status', name: 'Status' },
        },
      ],
    },
  };
}

function alreadyDoneItem(id = 'PVTI_1'): unknown {
  return {
    id,
    updatedAt: '2026-05-03T09:00:00Z',
    content: {
      __typename: 'Issue',
      id: 'I1',
      number: 11,
      title: 'already done',
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
          optionId: 'opt-done',
          name: 'Done',
          field: { id: 'F_status', name: 'Status' },
        },
      ],
    },
  };
}

describe('done command', () => {
  it('closes an open Issue and reports the URL', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const code = await done(['42', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'], {
      client: makeMockClient(recorded),
      stdout,
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(stdout.written).toContain('https://github.com/ozzy-labs/gh-tasks/issues/42');
    expect(recorded[0]?.vars).toEqual({ owner: 'ozzy-labs', name: 'gh-tasks', number: 42 });
    expect(recorded[1]?.vars).toEqual({ input: { issueId: 'I_42' } });
  });

  it('accepts # prefix on the id (#42 → 42)', async () => {
    const recorded: RecordedRequest[] = [];
    const code = await done(['#42', '--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient(recorded),
      stdout: makeStream(),
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(recorded[0]?.vars).toEqual({ owner: 'ozzy-labs', name: 'gh-tasks', number: 42 });
  });

  it('returns 2 when id is missing', async () => {
    const stderr = makeStream();
    const code = await done(['--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient([]),
      stdout: makeStream(),
      stderr,
    });
    expect(code).toBe(2);
    expect(stderr.written).toMatch(/(id|ID)/);
  });

  it('returns 1 when the Issue is not found', async () => {
    const stderr = makeStream();
    const code = await done(['999', '--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient([], { missing: true }),
      stdout: makeStream(),
      stderr,
    });
    expect(code).toBe(1);
    expect(stderr.written).toContain('not found');
  });

  it('skips the close mutation when the Issue is already CLOSED', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const code = await done(['42', '--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient(recorded, { state: 'CLOSED' }),
      stdout,
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(recorded).toHaveLength(1);
    expect(stdout.written).toMatch(/(already|既に|クローズ|closed)/i);
  });

  it('updates Status to Done in org scope', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await done(['PVTI_1', '--scope=org', '--project=ozzy-labs/7', '--lang=en'], {
      client: makeProjectMockClient(recorded, { items: [inProgressItem('PVTI_1')] }),
      hasGitRemote: () => false,
      stdout,
      stderr,
    });

    expect(code).toBe(0);
    expect(stderr.written).toBe('');

    // Order: GetOrgProjectV2 → ListProjectV2Fields → ListProjectV2Items → updateProjectV2ItemFieldValue
    expect(recorded).toHaveLength(4);
    expect(recorded[0]?.query).toContain('GetOrgProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy-labs', number: 7 });
    expect(recorded[1]?.query).toContain('ListProjectV2Fields');
    expect(recorded[2]?.query).toContain('ListProjectV2Items');
    expect(recorded[3]?.query).toContain('updateProjectV2ItemFieldValue');
    expect(recorded[3]?.vars).toEqual({
      input: {
        projectId: 'PVT_ORG',
        itemId: 'PVTI_1',
        fieldId: 'F_status',
        value: { singleSelectOptionId: 'opt-done' },
      },
    });

    expect(stdout.written.toLowerCase()).toContain('done');
    expect(stdout.written).toContain('PVTI_1');
  });

  it('skips the update mutation when the project item is already Done', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();

    const code = await done(['PVTI_1', '--scope=org', '--project=ozzy-labs/7', '--lang=en'], {
      client: makeProjectMockClient(recorded, { items: [alreadyDoneItem('PVTI_1')] }),
      hasGitRemote: () => false,
      stdout,
      stderr: makeStream(),
    });

    expect(code).toBe(0);
    // Should NOT reach the update mutation. Allowed: project lookup +
    // fields list + items list (3 reads, no mutation).
    expect(recorded).toHaveLength(3);
    for (const r of recorded) {
      expect(r.query).not.toContain('updateProjectV2ItemFieldValue');
    }
    expect(stdout.written.toLowerCase()).toMatch(/already|既に/);
  });

  it('returns 2 when --scope org is given without a project ref', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await done(['PVTI_1', '--scope=org'], {
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
});
