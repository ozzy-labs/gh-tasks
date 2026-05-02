import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { triage } from './triage.ts';

interface IssueFixture {
  number: number;
  title: string;
  labels: string[];
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
              updatedAt: '2026-05-01T00:00:00Z',
              labels: { nodes: i.labels.map((name) => ({ name })) },
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

describe('triage command', () => {
  it('lists Issues that have no labels', async () => {
    const stdout = makeStream();
    const code = await triage(['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'], {
      client: makeMockClient([
        { number: 1, title: 'untriaged-a', labels: [] },
        { number: 2, title: 'has-label', labels: ['feat'] },
        { number: 3, title: 'untriaged-b', labels: [] },
      ]),
      stdout,
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(stdout.written).toContain('#1');
    expect(stdout.written).toContain('untriaged-a');
    expect(stdout.written).toContain('#3');
    expect(stdout.written).not.toContain('#2');
    expect(stdout.written).not.toContain('has-label');
  });

  it('honors --limit on the filtered list', async () => {
    const stdout = makeStream();
    const code = await triage(
      ['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--limit=1', '--lang=en'],
      {
        client: makeMockClient([
          { number: 1, title: 'a', labels: [] },
          { number: 2, title: 'b', labels: [] },
          { number: 3, title: 'c', labels: [] },
        ]),
        stdout,
        stderr: makeStream(),
      }
    );
    expect(code).toBe(0);
    expect(stdout.written).toContain('#1');
    expect(stdout.written).not.toContain('#2');
  });

  it('shows empty message when everything is triaged', async () => {
    const stdout = makeStream();
    const code = await triage(['--scope=repo', '--repo=ozzy-labs/gh-tasks', '--lang=en'], {
      client: makeMockClient([{ number: 1, title: 'has-label', labels: ['feat'] }]),
      stdout,
      stderr: makeStream(),
    });
    expect(code).toBe(0);
    expect(stdout.written).toMatch(/(no untriaged|未トリアージ)/i);
  });

  it('returns 2 for non-repo scopes', async () => {
    const stderr = makeStream();
    const code = await triage(['--scope=user'], {
      client: makeMockClient([]),
      stdout: makeStream(),
      stderr,
    });
    expect(code).toBe(2);
    expect(stderr.written).toContain('--scope user');
  });
});
