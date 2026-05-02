import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { appendCloseLink, containsCloseLink, link } from './link.ts';

interface RecordedRequest {
  query: string;
  vars: Record<string, unknown>;
}

function makeMockClient(
  recorded: RecordedRequest[],
  options: { body?: string; missing?: boolean } = {}
): GraphQLClient {
  return {
    async request<T>(query: string, vars: Record<string, unknown> = {}): Promise<T> {
      recorded.push({ query, vars });
      if (query.includes('GetPullRequestByNumber')) {
        if (options.missing) return { repository: { pullRequest: null } } as T;
        return {
          repository: {
            pullRequest: {
              id: 'PR_1',
              number: 7,
              url: 'https://github.com/ozzy-labs/gh-tasks/pull/7',
              body: options.body ?? 'PR body',
            },
          },
        } as T;
      }
      if (query.includes('updatePullRequest')) {
        return {
          updatePullRequest: {
            pullRequest: {
              id: 'PR_1',
              number: 7,
              url: 'https://github.com/ozzy-labs/gh-tasks/pull/7',
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

describe('containsCloseLink', () => {
  it('matches Closes #N (case-insensitive)', () => {
    expect(containsCloseLink('Closes #42', 42)).toBe(true);
    expect(containsCloseLink('closes #42', 42)).toBe(true);
    expect(containsCloseLink('Fixes #42', 42)).toBe(true);
    expect(containsCloseLink('Resolves #42', 42)).toBe(true);
  });

  it('does not match different numbers', () => {
    expect(containsCloseLink('Closes #41', 42)).toBe(false);
    expect(containsCloseLink('Closes #420', 42)).toBe(false);
  });

  it('returns false when body has no close keyword', () => {
    expect(containsCloseLink('Just a description', 42)).toBe(false);
    expect(containsCloseLink('', 42)).toBe(false);
  });
});

describe('appendCloseLink', () => {
  it('appends Closes #N to a non-empty body with blank-line separator', () => {
    expect(appendCloseLink('summary line', 42)).toBe('summary line\n\nCloses #42\n');
  });

  it('does not double the separator when body has trailing newlines', () => {
    expect(appendCloseLink('summary\n\n', 42)).toBe('summary\n\nCloses #42\n');
  });

  it('handles empty body', () => {
    expect(appendCloseLink('', 42)).toBe('Closes #42\n');
  });
});

describe('link command', () => {
  it('appends Closes #task to PR body and reports the URL', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const code = await link(['7', '42', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'], {
      client: makeMockClient(recorded, { body: 'summary' }),
      stdout,
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(stdout.written).toContain('https://github.com/ozzy-labs/gh-tasks/pull/7');
    expect(recorded[1]?.vars).toEqual({
      input: { pullRequestId: 'PR_1', body: 'summary\n\nCloses #42\n' },
    });
  });

  it('skips update when PR body already references the task', async () => {
    const recorded: RecordedRequest[] = [];
    const code = await link(['7', '42', '--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient(recorded, { body: 'summary\n\nCloses #42\n' }),
      stdout: makeStream(),
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(recorded).toHaveLength(1); // only the GET, no UPDATE
  });

  it('returns 2 when fewer than 2 positionals are given', async () => {
    const stderr = makeStream();
    const code = await link(['7', '--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient([]),
      stdout: makeStream(),
      stderr,
    });
    expect(code).toBe(2);
    expect(stderr.written).toMatch(/(必要|required)/i);
  });

  it('accepts # prefix on positionals (#7 #42)', async () => {
    const recorded: RecordedRequest[] = [];
    const code = await link(['#7', '#42', '--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient(recorded, { body: 'b' }),
      stdout: makeStream(),
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(recorded[0]?.vars).toEqual({ owner: 'ozzy-labs', name: 'gh-tasks', number: 7 });
  });

  it('returns 1 when the PR is not found', async () => {
    const stderr = makeStream();
    const code = await link(['999', '42', '--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient([], { missing: true }),
      stdout: makeStream(),
      stderr,
    });
    expect(code).toBe(1);
    expect(stderr.written).toContain('not found');
  });

  it('returns 2 for non-repo scopes', async () => {
    const stderr = makeStream();
    const code = await link(['7', '42', '--scope=org'], {
      client: makeMockClient([]),
      stdout: makeStream(),
      stderr,
    });
    expect(code).toBe(2);
    expect(stderr.written).toContain('--scope org');
  });
});
