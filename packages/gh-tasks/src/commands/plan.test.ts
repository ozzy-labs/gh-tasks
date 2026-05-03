import { describe, expect, it } from 'vitest';
import type { GraphQLClient, RestClient, RestRequestOptions } from '../lib/github.ts';
import { plan } from './plan.ts';

interface IssueFixture {
  id?: string;
  number: number;
  title: string;
  updatedAt: string;
  milestone?: { id: string; number: number; title: string } | null;
}

interface MilestoneFixture {
  id: string;
  number: number;
  title: string;
}

interface RecordedRequest {
  query: string;
  vars: Record<string, unknown>;
}

interface RecordedRest {
  method: string;
  url: string;
  body?: Record<string, unknown>;
}

function makeMockClient(
  issues: IssueFixture[],
  options: {
    milestones?: MilestoneFixture[];
    recorded?: RecordedRequest[];
  } = {}
): GraphQLClient {
  return {
    async request<T>(query: string, vars: Record<string, unknown> = {}): Promise<T> {
      options.recorded?.push({ query, vars });
      if (query.includes('ListRepoIssuesWithMilestone')) {
        return {
          repository: {
            issues: {
              nodes: issues.map((i, idx) => ({
                id: i.id ?? `I${idx}`,
                number: i.number,
                title: i.title,
                url: `https://github.com/o/n/issues/${i.number}`,
                updatedAt: i.updatedAt,
                milestone: i.milestone ?? null,
              })),
            },
          },
        } as T;
      }
      if (query.includes('ListMilestones')) {
        return {
          repository: {
            milestones: { nodes: options.milestones ?? [] },
          },
        } as T;
      }
      if (query.includes('UpdateIssueMilestone')) {
        const input = vars.input as { id: string; milestoneId: string };
        return {
          updateIssue: {
            issue: {
              id: input.id,
              number: 0,
              url: '',
              milestone: { id: input.milestoneId, number: 0, title: '' },
            },
          },
        } as T;
      }
      throw new Error(`unexpected query: ${query}`);
    },
  };
}

function makeMockRest(
  created: { node_id: string; id: number; number: number; title: string },
  recorded?: RecordedRest[]
): RestClient {
  return {
    async request<T>(opts: RestRequestOptions): Promise<T> {
      recorded?.push({ method: opts.method, url: opts.url, body: opts.body });
      if (opts.method === 'POST' && opts.url.includes('/milestones')) {
        return created as T;
      }
      throw new Error(`unexpected REST request: ${opts.method} ${opts.url}`);
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

describe('plan command (dry-run)', () => {
  const NOW = new Date('2026-05-03T12:00:00Z'); // Sunday

  it('proposes a daily milestone and lists candidates updated today', async () => {
    const stdout = makeStream();
    const code = await plan(
      ['--period=daily', '--dry-run', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient([
          { number: 1, title: 'today-a', updatedAt: '2026-05-03T08:00:00Z' },
          { number: 2, title: 'old', updatedAt: '2026-05-01T08:00:00Z' },
        ]),
        rest: makeMockRest({ node_id: 'unused', id: 0, number: 0, title: '' }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('Daily 2026-05-03');
    expect(stdout.written).toContain('#1');
    expect(stdout.written).not.toContain('#2');
    expect(stdout.written).toContain('--dry-run');
  });

  it('proposes a weekly milestone anchored on Monday', async () => {
    const stdout = makeStream();
    const code = await plan(
      ['--period=weekly', '--dry-run', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient([
          { number: 1, title: 'this-week', updatedAt: '2026-04-29T08:00:00Z' },
          { number: 2, title: 'last-week', updatedAt: '2026-04-20T08:00:00Z' },
        ]),
        rest: makeMockRest({ node_id: 'unused', id: 0, number: 0, title: '' }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('Week of 2026-04-27');
    expect(stdout.written).toContain('#1');
    expect(stdout.written).not.toContain('#2');
  });

  it('proposes a sprint milestone covering 14 days', async () => {
    const stdout = makeStream();
    const code = await plan(
      ['--period=sprint', '--dry-run', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient([
          { number: 1, title: 'in-sprint', updatedAt: '2026-05-10T08:00:00Z' },
          { number: 2, title: 'past-sprint', updatedAt: '2026-04-25T08:00:00Z' },
        ]),
        rest: makeMockRest({ node_id: 'unused', id: 0, number: 0, title: '' }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('Sprint 2026-05-03');
    expect(stdout.written).toContain('#1');
    expect(stdout.written).not.toContain('#2');
  });

  it('defaults to weekly when --period is omitted', async () => {
    const stdout = makeStream();
    const code = await plan(
      ['--dry-run', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient([]),
        rest: makeMockRest({ node_id: 'unused', id: 0, number: 0, title: '' }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('Week of');
  });

  it('shows empty message when no candidates fall in range', async () => {
    const stdout = makeStream();
    const code = await plan(
      ['--period=daily', '--dry-run', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient([{ number: 1, title: 'old', updatedAt: '2026-04-01T00:00:00Z' }]),
        rest: makeMockRest({ node_id: 'unused', id: 0, number: 0, title: '' }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toMatch(/(候補|no candidates)/i);
  });

  it('returns 2 when --scope user is given without a project ref', async () => {
    const stderr = makeStream();
    const code = await plan(['--period=daily', '--dry-run', '--scope=user'], {
      client: makeMockClient([]),
      rest: makeMockRest({ node_id: 'unused', id: 0, number: 0, title: '' }),
      stdout: makeStream(),
      stderr,
      hasGitRemote: () => false,
      now: () => NOW,
    });
    expect(code).toBe(2);
    expect(stderr.written.toLowerCase()).toContain('project');
  });
});

describe('plan command (write mode)', () => {
  const NOW = new Date('2026-05-03T12:00:00Z');

  it('creates a new milestone via REST and binds candidate Issues', async () => {
    const recordedGql: RecordedRequest[] = [];
    const recordedRest: RecordedRest[] = [];
    const stdout = makeStream();

    const code = await plan(
      ['--period=daily', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient(
          [
            { id: 'I_1', number: 1, title: 'a', updatedAt: '2026-05-03T08:00:00Z' },
            { id: 'I_2', number: 2, title: 'b', updatedAt: '2026-05-03T09:00:00Z' },
          ],
          { milestones: [], recorded: recordedGql }
        ),
        rest: makeMockRest(
          { node_id: 'MI_new', id: 99, number: 7, title: 'Daily 2026-05-03' },
          recordedRest
        ),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );

    expect(code).toBe(0);
    expect(recordedRest).toHaveLength(1);
    expect(recordedRest[0]?.url).toBe('/repos/ozzy-labs/gh-tasks/milestones');
    expect(recordedRest[0]?.body).toEqual({ title: 'Daily 2026-05-03' });
    const updates = recordedGql.filter((r) => r.query.includes('UpdateIssueMilestone'));
    expect(updates).toHaveLength(2);
    for (const u of updates) {
      expect(u.vars.input).toMatchObject({ milestoneId: 'MI_new' });
    }
    expect(stdout.written).toContain('Milestone created');
    expect(stdout.written).toContain('Issue bound to milestone');
    expect(stdout.written).toContain('/milestone/7');
  });

  it('reuses an existing milestone with the same title and skips REST create', async () => {
    const recordedGql: RecordedRequest[] = [];
    const recordedRest: RecordedRest[] = [];
    const stdout = makeStream();

    const code = await plan(
      ['--period=daily', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient(
          [{ id: 'I_1', number: 1, title: 'a', updatedAt: '2026-05-03T08:00:00Z' }],
          {
            milestones: [{ id: 'MI_old', number: 4, title: 'Daily 2026-05-03' }],
            recorded: recordedGql,
          }
        ),
        rest: makeMockRest({ node_id: 'MI_unused', id: 0, number: 0, title: '' }, recordedRest),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );

    expect(code).toBe(0);
    expect(recordedRest).toHaveLength(0);
    const updates = recordedGql.filter((r) => r.query.includes('UpdateIssueMilestone'));
    expect(updates).toHaveLength(1);
    expect(updates[0]?.vars.input).toMatchObject({ id: 'I_1', milestoneId: 'MI_old' });
    expect(stdout.written).toContain('Reused');
    expect(stdout.written).toContain('/milestone/4');
  });

  it('skips Issues already bound to a different milestone and does not unbind', async () => {
    const recordedGql: RecordedRequest[] = [];
    const stdout = makeStream();

    const code = await plan(
      ['--period=daily', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient(
          [
            {
              id: 'I_1',
              number: 1,
              title: 'free',
              updatedAt: '2026-05-03T08:00:00Z',
            },
            {
              id: 'I_2',
              number: 2,
              title: 'already-bound',
              updatedAt: '2026-05-03T09:00:00Z',
              milestone: { id: 'MI_other', number: 99, title: 'Other' },
            },
          ],
          { milestones: [], recorded: recordedGql }
        ),
        rest: makeMockRest({ node_id: 'MI_new', id: 1, number: 5, title: 'Daily 2026-05-03' }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );

    expect(code).toBe(0);
    const updates = recordedGql.filter((r) => r.query.includes('UpdateIssueMilestone'));
    expect(updates).toHaveLength(1);
    expect(updates[0]?.vars.input).toMatchObject({ id: 'I_1' });
    expect(stdout.written).toContain('Skipped');
    expect(stdout.written).toContain('#2');
  });
});

interface ProjectMockOptions {
  orgProjectId?: string | null;
  userProjectId?: string | null;
  /** Items returned by ListProjectV2Items. */
  items?: Array<unknown>;
  /** Field nodes returned by ListProjectV2Fields. */
  fields?: Array<unknown>;
}

function defaultIterationFieldNodes(): Array<unknown> {
  return [
    {
      id: 'F_iter',
      name: 'Iteration',
      dataType: 'ITERATION',
      configuration: {
        iterations: [
          // Iteration containing 2026-05-03 (Sun): starts 2026-05-03, 7-day duration.
          {
            id: 'iter-current',
            title: 'Iteration W19',
            startDate: '2026-05-03',
            duration: 7,
          },
          {
            id: 'iter-next',
            title: 'Iteration W20',
            startDate: '2026-05-10',
            duration: 7,
          },
        ],
        completedIterations: [],
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
  const items = opts.items ?? [];
  const fields = opts.fields ?? defaultIterationFieldNodes();

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

function projectIssueItem(args: {
  id: string;
  number: number;
  title: string;
  updatedAt: string;
  iterationId?: string;
}): unknown {
  const fieldValues = args.iterationId
    ? [
        {
          __typename: 'ProjectV2ItemFieldIterationValue',
          iterationId: args.iterationId,
          title: 'X',
          startDate: '2026-05-03',
          duration: 7,
          field: { id: 'F_iter', name: 'Iteration' },
        },
      ]
    : [];
  return {
    id: args.id,
    updatedAt: args.updatedAt,
    content: {
      __typename: 'Issue',
      id: `I_${args.number}`,
      number: args.number,
      title: args.title,
      url: `https://github.com/o/n/issues/${args.number}`,
      state: 'OPEN',
      updatedAt: args.updatedAt,
      closedAt: null,
      author: { login: 'o' },
      assignees: { nodes: [] },
    },
    fieldValues: { nodes: fieldValues },
  };
}

describe('plan command (org / user scope)', () => {
  // 2026-05-03 is Sunday. The default iteration fixture has W19 starting that
  // day with duration 7 → "current iteration" for `now` and the daily/weekly
  // fallback target.
  const NOW = new Date('2026-05-03T12:00:00Z');

  it('previews candidate project items in dry-run without writes', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await plan(
      ['--period=daily', '--dry-run', '--scope=org', '--project=ozzy-labs/7', '--lang=en'],
      {
        client: makeProjectMockClient(recorded, {
          items: [
            projectIssueItem({
              id: 'PVTI_1',
              number: 11,
              title: 'in-range',
              updatedAt: '2026-05-03T08:00:00Z',
            }),
            projectIssueItem({
              id: 'PVTI_2',
              number: 12,
              title: 'old',
              updatedAt: '2026-04-25T08:00:00Z',
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
    // No mutation in dry-run.
    for (const r of recorded) {
      expect(r.query).not.toContain('updateProjectV2ItemFieldValue');
    }
    expect(stdout.written).toContain('#11');
    expect(stdout.written).not.toContain('#12');
    expect(stdout.written).toContain('--dry-run');
  });

  it('updates the iteration field on every in-range item in write mode', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await plan(
      ['--period=daily', '--scope=org', '--project=ozzy-labs/7', '--lang=en'],
      {
        client: makeProjectMockClient(recorded, {
          items: [
            projectIssueItem({
              id: 'PVTI_1',
              number: 11,
              title: 'a',
              updatedAt: '2026-05-03T08:00:00Z',
            }),
            projectIssueItem({
              id: 'PVTI_2',
              number: 12,
              title: 'b',
              updatedAt: '2026-05-03T09:00:00Z',
            }),
            // Already on the target iteration → should be skipped (no mutation).
            projectIssueItem({
              id: 'PVTI_3',
              number: 13,
              title: 'already-on',
              updatedAt: '2026-05-03T10:00:00Z',
              iterationId: 'iter-current',
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
    const updates = recorded.filter((r) => r.query.includes('updateProjectV2ItemFieldValue'));
    expect(updates).toHaveLength(2);
    for (const u of updates) {
      expect(u.vars.input).toMatchObject({
        projectId: 'PVT_ORG',
        fieldId: 'F_iter',
        value: { iterationId: 'iter-current' },
      });
    }
    const updatedItemIds = updates.map((u) => (u.vars.input as { itemId: string }).itemId).sort();
    expect(updatedItemIds).toEqual(['PVTI_1', 'PVTI_2']);
    expect(stdout.written).toContain('#11');
    expect(stdout.written).toContain('#12');
  });

  it('exits 1 when the project has no Iteration field', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const stdout = makeStream();

    const code = await plan(
      ['--period=daily', '--scope=org', '--project=ozzy-labs/7', '--lang=en'],
      {
        client: makeProjectMockClient(recorded, {
          // No ITERATION field — only a Status SINGLE_SELECT.
          fields: [
            {
              id: 'F_status',
              name: 'Status',
              dataType: 'SINGLE_SELECT',
              options: [{ id: 'opt-todo', name: 'Todo' }],
            },
          ],
        }),
        hasGitRemote: () => false,
        stdout,
        stderr,
        now: () => NOW,
      }
    );

    expect(code).toBe(1);
    expect(stderr.written.toLowerCase()).toContain('iteration');
    // Should NOT issue any mutation or list items.
    for (const r of recorded) {
      expect(r.query).not.toContain('updateProjectV2ItemFieldValue');
      expect(r.query).not.toContain('ListProjectV2Items');
    }
  });
});
