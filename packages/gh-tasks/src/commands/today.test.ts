import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { today } from './today.ts';

interface IssueFixture {
  number: number;
  title: string;
  url: string;
  updatedAt: string;
}

function makeMockClient(issues: IssueFixture[]): GraphQLClient {
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
      client: makeMockClient([
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
      client: makeMockClient([
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

  it('returns 2 for non-repo scopes', async () => {
    const stderr = makeStream();
    const code = await today(['--scope=user'], {
      client: makeMockClient([]),
      stdout: makeStream(),
      stderr,
      now: () => NOW,
    });
    expect(code).toBe(2);
    expect(stderr.written).toContain('--scope user');
  });
});
