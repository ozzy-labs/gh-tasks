import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { plan } from './plan.ts';

interface IssueFixture {
  number: number;
  title: string;
  updatedAt: string;
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
              updatedAt: i.updatedAt,
            })),
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

describe('plan command', () => {
  const NOW = new Date('2026-05-03T12:00:00Z'); // Sunday

  it('proposes a daily milestone and lists candidates updated today', async () => {
    const stdout = makeStream();
    const code = await plan(
      ['--period=daily', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient([
          { number: 1, title: 'today-a', updatedAt: '2026-05-03T08:00:00Z' },
          { number: 2, title: 'old', updatedAt: '2026-05-01T08:00:00Z' },
        ]),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('Daily 2026-05-03');
    expect(stdout.written).toContain('#1');
    expect(stdout.written).not.toContain('#2');
  });

  it('proposes a weekly milestone anchored on Monday', async () => {
    const stdout = makeStream();
    const code = await plan(
      ['--period=weekly', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient([
          { number: 1, title: 'this-week', updatedAt: '2026-04-29T08:00:00Z' },
          { number: 2, title: 'last-week', updatedAt: '2026-04-20T08:00:00Z' },
        ]),
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
      ['--period=sprint', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient([
          { number: 1, title: 'in-sprint', updatedAt: '2026-05-10T08:00:00Z' },
          { number: 2, title: 'past-sprint', updatedAt: '2026-04-25T08:00:00Z' },
        ]),
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
    const code = await plan(['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'], {
      client: makeMockClient([]),
      stdout,
      stderr: makeStream(),
      now: () => NOW,
    });
    expect(code).toBe(0);
    expect(stdout.written).toContain('Week of');
  });

  it('shows empty message when no candidates fall in range', async () => {
    const stdout = makeStream();
    const code = await plan(
      ['--period=daily', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeMockClient([{ number: 1, title: 'old', updatedAt: '2026-04-01T00:00:00Z' }]),
        stdout,
        stderr: makeStream(),
        now: () => NOW,
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toMatch(/(候補|no candidates)/i);
  });

  it('returns 2 for non-repo scopes', async () => {
    const stderr = makeStream();
    const code = await plan(['--period=daily', '--scope=user'], {
      client: makeMockClient([]),
      stdout: makeStream(),
      stderr,
      now: () => NOW,
    });
    expect(code).toBe(2);
    expect(stderr.written).toContain('--scope user');
  });
});
