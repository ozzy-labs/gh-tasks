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

  it('returns 2 for non-repo scopes', async () => {
    const stderr = makeStream();
    const code = await review(['--period=daily', '--scope=org'], {
      client: makeMockClient({}),
      stdout: makeStream(),
      stderr,
      now: () => NOW,
    });
    expect(code).toBe(2);
    expect(stderr.written).toContain('--scope org');
  });
});
