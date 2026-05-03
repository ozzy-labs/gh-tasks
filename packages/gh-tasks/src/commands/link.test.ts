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

interface ProjectMockOptions {
  orgProjectId?: string | null;
  userProjectId?: string | null;
  prMissing?: boolean;
  issueMissing?: boolean;
}

function makeProjectMockClient(
  recorded: RecordedRequest[],
  opts: ProjectMockOptions = {}
): GraphQLClient {
  const orgProjectId = opts.orgProjectId === undefined ? 'PVT_ORG' : opts.orgProjectId;
  const userProjectId = opts.userProjectId === undefined ? 'PVT_USER' : opts.userProjectId;

  return {
    async request<T>(query: string, vars: Record<string, unknown> = {}): Promise<T> {
      recorded.push({ query, vars });
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
      if (query.includes('GetPullRequestByNumber')) {
        if (opts.prMissing) return { repository: { pullRequest: null } } as T;
        return {
          repository: {
            pullRequest: {
              id: 'PR_1',
              number: 7,
              url: 'https://github.com/ozzy-labs/gh-tasks/pull/7',
              body: 'PR body',
            },
          },
        } as T;
      }
      if (query.includes('GetIssueByNumber')) {
        if (opts.issueMissing) return { repository: { issue: null } } as T;
        return {
          repository: {
            issue: {
              id: 'I_42',
              number: 42,
              url: 'https://github.com/ozzy-labs/gh-tasks/issues/42',
              state: 'OPEN',
            },
          },
        } as T;
      }
      if (query.includes('addProjectV2ItemById')) {
        return { addProjectV2ItemById: { item: { id: 'PVTI_added' } } } as T;
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

  it('adds PR and task to the project in org scope (idempotent double-add)', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await link(
      ['7', '42', '--scope=org', '--project=ozzy-labs/7', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeProjectMockClient(recorded),
        hasGitRemote: () => false,
        stdout,
        stderr,
      }
    );

    expect(code).toBe(0);
    expect(stderr.written).toBe('');

    // Order: GetOrgProjectV2 → GetPullRequestByNumber → GetIssueByNumber
    //        → addProjectV2ItemById (PR) → addProjectV2ItemById (Issue)
    expect(recorded).toHaveLength(5);
    expect(recorded[0]?.query).toContain('GetOrgProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy-labs', number: 7 });
    expect(recorded[1]?.query).toContain('GetPullRequestByNumber');
    expect(recorded[2]?.query).toContain('GetIssueByNumber');

    const adds = recorded.filter((r) => r.query.includes('addProjectV2ItemById'));
    expect(adds).toHaveLength(2);
    expect(adds[0]?.vars).toEqual({ input: { projectId: 'PVT_ORG', contentId: 'PR_1' } });
    expect(adds[1]?.vars).toEqual({ input: { projectId: 'PVT_ORG', contentId: 'I_42' } });

    expect(stdout.written).toContain('https://github.com/ozzy-labs/gh-tasks/pull/7');
    expect(stdout.written).toContain('https://github.com/ozzy-labs/gh-tasks/issues/42');
  });

  it('adds PR and task to the project in user scope (idempotent double-add)', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();
    const stderr = makeStream();

    const code = await link(
      ['7', '42', '--scope=user', '--project=ozzy/3', '--repo=ozzy-labs/gh-tasks', '--lang=en'],
      {
        client: makeProjectMockClient(recorded),
        hasGitRemote: () => false,
        stdout,
        stderr,
      }
    );

    expect(code).toBe(0);
    expect(stderr.written).toBe('');

    expect(recorded[0]?.query).toContain('GetUserProjectV2');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy', number: 3 });

    const adds = recorded.filter((r) => r.query.includes('addProjectV2ItemById'));
    expect(adds).toHaveLength(2);
    expect(adds[0]?.vars).toEqual({ input: { projectId: 'PVT_USER', contentId: 'PR_1' } });
    expect(adds[1]?.vars).toEqual({ input: { projectId: 'PVT_USER', contentId: 'I_42' } });
  });

  it('returns 2 when --scope org is given without a project ref', async () => {
    const recorded: RecordedRequest[] = [];
    const stderr = makeStream();
    const code = await link(['7', '42', '--scope=org', '--repo=ozzy-labs/gh-tasks'], {
      client: makeProjectMockClient(recorded),
      hasGitRemote: () => false,
      stdout: makeStream(),
      stderr,
    });

    expect(code).toBe(2);
    expect(stderr.written.toLowerCase()).toContain('project');
    expect(recorded).toHaveLength(0);
  });
});
