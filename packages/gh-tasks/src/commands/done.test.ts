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

  it('returns 2 for non-repo scopes', async () => {
    const stderr = makeStream();
    const code = await done(['42', '--scope=user'], {
      client: makeMockClient([]),
      stdout: makeStream(),
      stderr,
    });
    expect(code).toBe(2);
    expect(stderr.written).toContain('--scope user');
  });
});
