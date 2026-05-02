import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { list } from './list.ts';

interface RecordedRequest {
  query: string;
  vars: Record<string, unknown>;
}

function makeMockClient(
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
      client: makeMockClient(recorded),
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
      client: makeMockClient(recorded),
      stdout: makeStream(),
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(recorded[0]?.vars).toEqual({ owner: 'ozzy-labs', name: 'gh-tasks', first: 5 });
  });

  it('shows empty message when there are no issues', async () => {
    const stdout = makeStream();
    const code = await list(['--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient([], []),
      stdout,
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(stdout.written).toMatch(/(オープン|open)/i);
  });

  it('returns 2 for non-repo scopes', async () => {
    const stderr = makeStream();
    const code = await list(['--scope=org'], {
      client: makeMockClient([]),
      stdout: makeStream(),
      stderr,
    });
    expect(code).toBe(2);
    expect(stderr.written).toContain('--scope org');
  });
});
