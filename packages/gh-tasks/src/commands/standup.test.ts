import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { standup } from './standup.ts';

interface ItemFixture {
  number: number;
  title: string;
  /** When omitted, falls back to a fixture-supplied default author. */
  author?: string;
  assignees?: string[];
}

interface Fixture {
  closed?: Array<ItemFixture & { closedAt: string }>;
  merged?: Array<ItemFixture & { mergedAt: string }>;
  open?: Array<ItemFixture & { updatedAt: string }>;
  viewerLogin?: string;
  defaultAuthor?: string;
}

function makeMockClient(f: Fixture): GraphQLClient {
  const auth = (i: ItemFixture): { login: string } | null => {
    const login = i.author ?? f.defaultAuthor;
    return login ? { login } : null;
  };
  const assignees = (i: ItemFixture) => ({
    nodes: (i.assignees ?? []).map((login) => ({ login })),
  });
  return {
    async request<T>(query: string): Promise<T> {
      if (query.includes('GetViewerLogin')) {
        return { viewer: { login: f.viewerLogin ?? 'me' } } as T;
      }
      if (query.includes('ListClosedIssues')) {
        return {
          repository: {
            issues: {
              nodes: (f.closed ?? []).map((i, idx) => ({
                id: `I${idx}`,
                number: i.number,
                title: i.title,
                url: `https://github.com/o/n/issues/${i.number}`,
                closedAt: i.closedAt,
                author: auth(i),
                assignees: assignees(i),
              })),
            },
          },
        } as T;
      }
      if (query.includes('ListMergedPRs')) {
        return {
          repository: {
            pullRequests: {
              nodes: (f.merged ?? []).map((p, idx) => ({
                id: `P${idx}`,
                number: p.number,
                title: p.title,
                url: `https://github.com/o/n/pull/${p.number}`,
                mergedAt: p.mergedAt,
                author: auth(p),
                assignees: assignees(p),
              })),
            },
          },
        } as T;
      }
      if (query.includes('ListRepoIssues')) {
        return {
          repository: {
            issues: {
              nodes: (f.open ?? []).map((i, idx) => ({
                id: `O${idx}`,
                number: i.number,
                title: i.title,
                url: `https://github.com/o/n/issues/${i.number}`,
                updatedAt: i.updatedAt,
                author: auth(i),
                assignees: assignees(i),
              })),
            },
          },
        } as T;
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

describe('standup command', () => {
  const NOW = new Date('2026-05-03T12:00:00Z');
  // 24h ago = 2026-05-02T12:00:00Z

  it('renders Yesterday / Today / Blockers sections from the last 24h by default', async () => {
    const stdout = makeStream();
    const code = await standup(['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'], {
      client: makeMockClient({
        closed: [
          { number: 1, title: 'closed-recent', closedAt: '2026-05-03T08:00:00Z' },
          { number: 2, title: 'closed-old', closedAt: '2026-05-01T00:00:00Z' },
        ],
        merged: [{ number: 7, title: 'merged-recent', mergedAt: '2026-05-03T07:00:00Z' }],
        open: [{ number: 9, title: 'open-recent', updatedAt: '2026-05-03T08:00:00Z' }],
      }),
      stdout,
      stderr: makeStream(),
      now: () => NOW,
    });
    expect(code).toBe(0);
    expect(stdout.written).toContain('## ');
    expect(stdout.written).toContain('Yesterday');
    expect(stdout.written).toContain('Today');
    expect(stdout.written).toContain('Blockers');
    expect(stdout.written).toContain('#1 closed-recent');
    expect(stdout.written).not.toContain('#2 closed-old');
    expect(stdout.written).toContain('#7 merged-recent');
    expect(stdout.written).toContain('#9 open-recent');
  });

  it('honors --since with an explicit ISO timestamp', async () => {
    const stdout = makeStream();
    const code = await standup(
      ['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--since=2026-05-01T00:00:00Z', '--lang=en'],
      {
        client: makeMockClient({
          closed: [{ number: 1, title: 'three-days-ago', closedAt: '2026-05-01T08:00:00Z' }],
        }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('#1 three-days-ago');
  });

  it('annotates the heading with viewer login when --mine is given', async () => {
    const stdout = makeStream();
    const code = await standup(
      ['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--mine', '--lang=en'],
      {
        client: makeMockClient({ viewerLogin: 'ozzy-3' }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('@ozzy-3');
  });

  it('--mine filters to items authored by viewer', async () => {
    const stdout = makeStream();
    const code = await standup(
      ['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--mine', '--lang=en'],
      {
        client: makeMockClient({
          viewerLogin: 'me',
          closed: [
            { number: 1, title: 'mine-closed', author: 'me', closedAt: '2026-05-03T08:00:00Z' },
            {
              number: 2,
              title: 'theirs-closed',
              author: 'other',
              closedAt: '2026-05-03T08:00:00Z',
            },
          ],
          merged: [
            { number: 7, title: 'mine-merged', author: 'me', mergedAt: '2026-05-03T07:00:00Z' },
            {
              number: 8,
              title: 'theirs-merged',
              author: 'other',
              mergedAt: '2026-05-03T07:00:00Z',
            },
          ],
          open: [
            { number: 9, title: 'mine-open', author: 'me', updatedAt: '2026-05-03T08:00:00Z' },
            {
              number: 10,
              title: 'theirs-open',
              author: 'other',
              updatedAt: '2026-05-03T08:00:00Z',
            },
          ],
        }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('#1 mine-closed');
    expect(stdout.written).not.toContain('#2 theirs-closed');
    expect(stdout.written).toContain('#7 mine-merged');
    expect(stdout.written).not.toContain('#8 theirs-merged');
    expect(stdout.written).toContain('#9 mine-open');
    expect(stdout.written).not.toContain('#10 theirs-open');
  });

  it('--mine also matches items where viewer is an assignee (not author)', async () => {
    const stdout = makeStream();
    const code = await standup(
      ['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--mine', '--lang=en'],
      {
        client: makeMockClient({
          viewerLogin: 'me',
          open: [
            {
              number: 1,
              title: 'assigned-to-me',
              author: 'someone-else',
              assignees: ['me'],
              updatedAt: '2026-05-03T08:00:00Z',
            },
            {
              number: 2,
              title: 'unrelated',
              author: 'someone-else',
              assignees: ['third-party'],
              updatedAt: '2026-05-03T08:00:00Z',
            },
          ],
        }),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('#1 assigned-to-me');
    expect(stdout.written).not.toContain('#2 unrelated');
  });

  it('without --mine, all activity is shown regardless of author', async () => {
    const stdout = makeStream();
    const code = await standup(['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'], {
      client: makeMockClient({
        defaultAuthor: 'someone-else',
        closed: [{ number: 1, title: 'foreign', closedAt: '2026-05-03T08:00:00Z' }],
      }),
      stdout,
      stderr: makeStream(),
      now: () => NOW,
    });
    expect(code).toBe(0);
    expect(stdout.written).toContain('#1 foreign');
  });

  it('does not emit the deprecated mineNote', async () => {
    const stdout = makeStream();
    await standup(['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--mine', '--lang=en'], {
      client: makeMockClient({ viewerLogin: 'ozzy-3' }),
      stdout,
      stderr: makeStream(),
      now: () => NOW,
    });
    expect(stdout.written).not.toMatch(/v0\.2\.0/);
    expect(stdout.written).not.toMatch(/lands in/);
  });
});

interface RecordedRequest {
  query: string;
  vars: Record<string, unknown>;
}

interface ProjectMockOptions {
  orgProjectId?: string | null;
  userProjectId?: string | null;
  items?: Array<unknown>;
  viewerLogin?: string;
  nodeMissing?: boolean;
}

/**
 * Build a Projects v2 item fixture with a Status single-select value and an
 * Issue / PullRequest content node carrying author + assignees. Mirrors the
 * shape returned by `LIST_PROJECT_V2_ITEMS`.
 */
function projectIssueItem(args: {
  id: string;
  number: number;
  title: string;
  updatedAt: string;
  status?: string | null;
  author?: string;
  assignees?: string[];
  contentType?: 'Issue' | 'PullRequest' | 'DraftIssue';
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
  if (typename === 'DraftIssue') {
    return {
      id: args.id,
      updatedAt: args.updatedAt,
      content: {
        __typename: 'DraftIssue',
        id: `D_${args.number}`,
        title: args.title,
        body: null,
      },
      fieldValues: { nodes: fieldValues },
    };
  }
  const baseContent = {
    __typename: typename,
    id: `I_${args.number}`,
    number: args.number,
    title: args.title,
    url:
      typename === 'PullRequest'
        ? `https://github.com/o/n/pull/${args.number}`
        : `https://github.com/o/n/issues/${args.number}`,
    state: typename === 'PullRequest' ? 'OPEN' : 'OPEN',
    updatedAt: args.updatedAt,
    author: { login: args.author ?? 'someone' },
    assignees: { nodes: (args.assignees ?? []).map((login) => ({ login })) },
  };
  const content =
    typename === 'PullRequest'
      ? { ...baseContent, mergedAt: null }
      : { ...baseContent, closedAt: null };
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
      if (query.includes('GetViewerLogin')) {
        return { viewer: { login: opts.viewerLogin ?? 'me' } } as T;
      }
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

describe('standup command (org / user scope)', () => {
  // 24h ago = 2026-05-02T12:00:00Z
  const NOW = new Date('2026-05-03T12:00:00Z');

  it('splits in-range project items by Status (Done → Yesterday, others → Today)', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await standup(['--scope=org', '--project=ozzy-labs/7', '--lang=en'], {
      client: makeProjectMockClient(recorded, {
        items: [
          projectIssueItem({
            id: 'PVTI_done',
            number: 11,
            title: 'wrapped-up',
            updatedAt: '2026-05-03T08:00:00Z',
            status: 'Done',
          }),
          projectIssueItem({
            id: 'PVTI_inprog',
            number: 12,
            title: 'still-going',
            updatedAt: '2026-05-03T09:00:00Z',
            status: 'In Progress',
          }),
          projectIssueItem({
            id: 'PVTI_unset',
            number: 13,
            title: 'no-status',
            updatedAt: '2026-05-03T10:00:00Z',
            status: null,
          }),
          // Out-of-range → excluded entirely.
          projectIssueItem({
            id: 'PVTI_old',
            number: 14,
            title: 'last-week',
            updatedAt: '2026-04-30T08:00:00Z',
            status: 'Done',
          }),
        ],
      }),
      hasGitRemote: () => false,
      stdout,
      stderr,
      now: () => NOW,
    });

    expect(code).toBe(0);
    expect(stderr.written).toBe('');
    expect(recorded[0]?.query).toContain('GetOrgProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy-labs', number: 7 });
    expect(recorded[1]?.query).toContain('ListProjectV2Items');
    expect(recorded[1]?.vars).toEqual({ projectId: 'PVT_ORG', first: 100 });

    const out = stdout.written;
    expect(out).toContain('Yesterday');
    expect(out).toContain('Today');
    expect(out).toContain('Blockers');
    // Yesterday section: Done item present.
    expect(out).toContain('done: #11 wrapped-up');
    // Today section: non-Done + unset both shown.
    expect(out).toContain('in-progress: #12 still-going');
    expect(out).toContain('in-progress: #13 no-status');
    // Out-of-range item filtered out.
    expect(out).not.toContain('#14');
    expect(out).not.toContain('last-week');
  });

  it('--mine filters items to viewer author / assignee and excludes DraftIssues', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();

    const code = await standup(['--scope=user', '--project=ozzy3/3', '--mine', '--lang=en'], {
      client: makeProjectMockClient(recorded, {
        viewerLogin: 'me',
        items: [
          // Authored by viewer → kept.
          projectIssueItem({
            id: 'PVTI_mine_done',
            number: 21,
            title: 'mine-done',
            updatedAt: '2026-05-03T08:00:00Z',
            status: 'Done',
            author: 'me',
          }),
          // Assigned to viewer → kept.
          projectIssueItem({
            id: 'PVTI_mine_assigned',
            number: 22,
            title: 'mine-assigned',
            updatedAt: '2026-05-03T08:00:00Z',
            status: 'In Progress',
            author: 'other',
            assignees: ['me'],
          }),
          // Author / assignee both not viewer → excluded.
          projectIssueItem({
            id: 'PVTI_theirs',
            number: 23,
            title: 'theirs',
            updatedAt: '2026-05-03T08:00:00Z',
            status: 'In Progress',
            author: 'other',
          }),
          // DraftIssue: no author/assignee → excluded under --mine.
          projectIssueItem({
            id: 'PVTI_draft',
            number: 24,
            title: 'draft-item',
            updatedAt: '2026-05-03T08:00:00Z',
            status: 'In Progress',
            contentType: 'DraftIssue',
          }),
        ],
      }),
      hasGitRemote: () => false,
      stdout,
      stderr: makeStream(),
      now: () => NOW,
    });

    expect(code).toBe(0);
    const out = stdout.written;
    expect(out).toContain('@me');
    expect(out).toContain('#21 mine-done');
    expect(out).toContain('#22 mine-assigned');
    expect(out).not.toContain('#23');
    expect(out).not.toContain('theirs');
    expect(out).not.toContain('draft-item');
    expect(recorded[0]?.query).toContain('GetViewerLogin');
    expect(recorded[1]?.query).toContain('GetUserProjectV2');
    expect(recorded[1]?.vars).toEqual({ login: 'ozzy3', number: 3 });
  });

  it('returns 2 when --scope org is given without a project ref', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await standup(['--scope=org'], {
      client: makeProjectMockClient(recorded),
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
      now: () => NOW,
    });

    expect(code).toBe(2);
    expect(stderr.written.toLowerCase()).toContain('project');
    // Project ref unresolvable → must not call GraphQL.
    expect(recorded).toHaveLength(0);
  });

  it('returns 1 when the org project cannot be resolved', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await standup(['--scope=org', '--project=ozzy-labs/999', '--lang=en'], {
      client: makeProjectMockClient(recorded, { orgProjectId: null }),
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
      now: () => NOW,
    });

    expect(code).toBe(1);
    expect(stderr.written).toContain('project not found');
  });

  it('honors --since for org scope', async () => {
    const stdout = makeStream();
    const code = await standup(
      ['--scope=org', '--project=ozzy-labs/7', '--since=2026-04-01T00:00:00Z', '--lang=en'],
      {
        client: makeProjectMockClient([], {
          items: [
            // Without --since this would be out of the default 24h window.
            projectIssueItem({
              id: 'PVTI_april',
              number: 31,
              title: 'april-done',
              updatedAt: '2026-04-15T08:00:00Z',
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
    expect(stdout.written).toContain('done: #31 april-done');
  });
});
