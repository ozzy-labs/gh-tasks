import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { add } from './add.ts';

interface RecordedRequest {
  query: string;
  vars: Record<string, unknown>;
}

function makeMockClient(recorded: RecordedRequest[]): GraphQLClient {
  return {
    async request<T>(query: string, vars: Record<string, unknown> = {}): Promise<T> {
      recorded.push({ query, vars });
      if (query.includes('GetRepositoryId')) {
        return { repository: { id: 'REPO_ID' } } as T;
      }
      if (query.includes('createIssue')) {
        return {
          createIssue: {
            issue: { id: 'I1', number: 42, url: 'https://github.com/ozzy-labs/gh-tasks/issues/42' },
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

describe('add command', () => {
  it('creates an Issue in repo scope and prints the URL', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await add(['my title', '--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient(recorded),
      stdout,
      stderr,
    });

    expect(code).toBe(0);
    expect(stderr.written).toBe('');
    expect(stdout.written).toContain('https://github.com/ozzy-labs/gh-tasks/issues/42');
    expect(recorded).toHaveLength(2);
    expect(recorded[0]?.vars).toEqual({ owner: 'ozzy-labs', name: 'gh-tasks' });
    expect(recorded[1]?.vars).toEqual({
      input: { repositoryId: 'REPO_ID', title: 'my title' },
    });
  });

  it('passes through --body when provided', async () => {
    const recorded: RecordedRequest[] = [];
    const code = await add(
      ['my title', '--scope=repo', '--repo=ozzy-labs/gh-tasks', '--body', 'detail'],
      {
        client: makeMockClient(recorded),
        stdout: makeStream(),
        stderr: makeStream(),
      }
    );

    expect(code).toBe(0);
    expect(recorded[1]?.vars).toEqual({
      input: { repositoryId: 'REPO_ID', title: 'my title', body: 'detail' },
    });
  });

  it('returns 2 with an error message when title is missing', async () => {
    const stderr = makeStream();
    const code = await add(['--scope=repo', '--repo=ozzy-labs/gh-tasks'], {
      client: makeMockClient([]),
      stdout: makeStream(),
      stderr,
    });

    expect(code).toBe(2);
    expect(stderr.written).toContain('title');
  });

  it('parses title correctly when value-flags appear before it', async () => {
    const recorded: RecordedRequest[] = [];
    const code = await add(
      ['--lang', 'ja', '--scope=repo', '--repo=ozzy-labs/gh-tasks', 'my title'],
      {
        client: makeMockClient(recorded),
        stdout: makeStream(),
        stderr: makeStream(),
      }
    );

    expect(code).toBe(0);
    expect(recorded[1]?.vars).toEqual({
      input: { repositoryId: 'REPO_ID', title: 'my title' },
    });
  });

  it('returns 2 for non-repo scopes (org / user) until they are implemented', async () => {
    const stderr = makeStream();
    const code = await add(['my title', '--scope=org'], {
      client: makeMockClient([]),
      stdout: makeStream(),
      stderr,
    });

    expect(code).toBe(2);
    expect(stderr.written).toContain('--scope org');
  });
});
