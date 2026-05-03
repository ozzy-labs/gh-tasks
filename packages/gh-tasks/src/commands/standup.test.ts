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

  it('returns 2 for non-repo scopes', async () => {
    const stderr = makeStream();
    const code = await standup(['--scope=user'], {
      client: makeMockClient({}),
      stdout: makeStream(),
      stderr,
      now: () => NOW,
    });
    expect(code).toBe(2);
    expect(stderr.written).toContain('--scope user');
  });
});
