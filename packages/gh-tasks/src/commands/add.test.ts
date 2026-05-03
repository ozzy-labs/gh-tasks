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
      if (query.includes('GetOrgProjectV2')) {
        return { organization: { projectV2: { id: 'PVT_ORG', number: 7, title: 'Org' } } } as T;
      }
      if (query.includes('GetUserProjectV2')) {
        return { user: { projectV2: { id: 'PVT_USER', number: 3, title: 'User' } } } as T;
      }
      if (query.includes('addProjectV2DraftIssue')) {
        return {
          addProjectV2DraftIssue: { projectItem: { id: 'PVTI_DRAFT_1' } },
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

  it('adds a draft item via addProjectV2DraftIssue in org scope', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await add(['draft title', '--scope=org', '--project=ozzy-labs/7', '--lang=en'], {
      client: makeMockClient(recorded),
      hasGitRemote: () => false,
      stdout,
      stderr,
    });

    expect(code).toBe(0);
    expect(stderr.written).toBe('');
    expect(stdout.written).toContain('PVTI_DRAFT_1');

    expect(recorded).toHaveLength(2);
    expect(recorded[0]?.query).toContain('GetOrgProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy-labs', number: 7 });
    expect(recorded[1]?.query).toContain('addProjectV2DraftIssue');
    expect(recorded[1]?.vars).toEqual({
      input: { projectId: 'PVT_ORG', title: 'draft title' },
    });
  });

  it('adds a draft item via addProjectV2DraftIssue in user scope', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await add(
      ['my draft', '--scope=user', '--project=ozzy3/3', '--body=hello', '--lang=en'],
      {
        client: makeMockClient(recorded),
        hasGitRemote: () => false,
        stdout,
        stderr,
      }
    );

    expect(code).toBe(0);
    expect(stderr.written).toBe('');
    expect(stdout.written).toContain('PVTI_DRAFT_1');

    expect(recorded).toHaveLength(2);
    expect(recorded[0]?.query).toContain('GetUserProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy3', number: 3 });
    expect(recorded[1]?.query).toContain('addProjectV2DraftIssue');
    expect(recorded[1]?.vars).toEqual({
      input: { projectId: 'PVT_USER', title: 'my draft', body: 'hello' },
    });
  });

  it('returns 2 when --scope org is given without a project ref (flag or config)', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await add(['my title', '--scope=org'], {
      client: makeMockClient(recorded),
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
    });

    expect(code).toBe(2);
    expect(stderr.written.toLowerCase()).toContain('project');
    // No GraphQL request should have been issued before bailing out.
    expect(recorded).toHaveLength(0);
  });

  it('returns 1 when the org project cannot be resolved', async () => {
    const recorded: RecordedRequest[] = [];
    const client: GraphQLClient = {
      async request<T>(query: string, vars: Record<string, unknown> = {}): Promise<T> {
        recorded.push({ query, vars });
        if (query.includes('GetOrgProjectV2')) {
          return { organization: { projectV2: null } } as T;
        }
        throw new Error(`unexpected query: ${query}`);
      },
    };
    const stderr = makeStream();
    const code = await add(['draft title', '--scope=org', '--project=ozzy-labs/999'], {
      client,
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
    });

    expect(code).toBe(1);
    expect(stderr.written).toContain('project not found');
    expect(recorded).toHaveLength(1);
  });
});
