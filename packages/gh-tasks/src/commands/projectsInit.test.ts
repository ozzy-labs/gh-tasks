import { describe, expect, it } from 'vitest';
import type { GraphQLClient } from '../lib/github.ts';
import { projectsInit } from './projectsInit.ts';

interface RecordedRequest {
  query: string;
  vars: Record<string, unknown>;
}

interface MockClientOptions {
  /**
   * Pre-existing field names returned by ListProjectV2Fields after the
   * project is created. Defaults to empty (greenfield project, all YAML
   * fields end up created).
   */
  existingFields?: string[];
}

function makeMockClient(
  recorded: RecordedRequest[],
  options: MockClientOptions = {}
): GraphQLClient {
  let projectFieldCounter = 0;
  const existing = options.existingFields ?? [];
  return {
    async request<T>(query: string, vars: Record<string, unknown> = {}): Promise<T> {
      recorded.push({ query, vars });
      if (query.includes('GetViewerId')) {
        return { viewer: { id: 'U_VIEWER', login: 'me' } } as T;
      }
      if (query.includes('GetOwnerId')) {
        return {
          repositoryOwner: { __typename: 'Organization', id: 'O_ORG', login: 'ozzy-labs' },
        } as T;
      }
      if (query.includes('CreateProjectV2(')) {
        return {
          createProjectV2: {
            projectV2: {
              id: 'PVT_NEW',
              number: 99,
              title: (vars.input as { title: string }).title,
              url: 'https://github.com/users/me/projects/99',
            },
          },
        } as T;
      }
      if (query.includes('ListProjectV2Fields')) {
        return {
          node: {
            fields: {
              nodes: existing.map((name, i) => ({
                id: `BUILTIN_${i}`,
                name,
                dataType: 'TEXT',
              })),
            },
          },
        } as T;
      }
      if (query.includes('CreateProjectV2Field(')) {
        projectFieldCounter += 1;
        const input = vars.input as { name: string; dataType: string };
        return {
          createProjectV2Field: {
            projectV2Field: {
              id: `F_${projectFieldCounter}`,
              name: input.name,
              dataType: input.dataType,
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

const USER_TEMPLATE = `name: gh-tasks user scope
description: Personal Project v2 fields for gh-tasks (user scope)
fields:
  - name: Status
    type: single_select
    options:
      - Triage
      - Todo
      - In Progress
      - Done
  - name: Iteration
    type: iteration
`;

const ORG_TEMPLATE = `name: gh-tasks org scope
description: Team Project v2 fields for gh-tasks (org scope)
fields:
  - name: Status
    type: single_select
    options:
      - Triage
      - Todo
      - In Progress
      - Done
  - name: Iteration
    type: iteration
  - name: Repository
    type: repository
  - name: Project
    type: single_select
    options:
      - Platform
      - Docs
      - Infra
`;

function readUserTemplate(): Promise<string> {
  return Promise.resolve(USER_TEMPLATE);
}

function readOrgTemplate(): Promise<string> {
  return Promise.resolve(ORG_TEMPLATE);
}

describe('projects init', () => {
  it('refuses without --title', async () => {
    const stderr = makeStream();
    const code = await projectsInit(['--template=user'], { stderr });
    expect(code).toBe(2);
    expect(stderr.written).toMatch(/title/i);
  });

  it('refuses without --template or yaml path', async () => {
    const stderr = makeStream();
    const code = await projectsInit(['--title=test'], { stderr });
    expect(code).toBe(2);
    expect(stderr.written).toMatch(/template|YAML/);
  });

  it('refuses when --template and yaml path are both given', async () => {
    const stderr = makeStream();
    const code = await projectsInit(['some.yaml', '--template=user', '--title=test'], { stderr });
    expect(code).toBe(2);
    expect(stderr.written).toMatch(/同時|cannot be combined/);
  });

  it('--dry-run lists fields without issuing mutations', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();

    const code = await projectsInit(
      ['--template=user', '--title=my todo', '--dry-run', '--lang=en'],
      {
        client: makeMockClient(recorded),
        stdout,
        readYaml: readUserTemplate,
      }
    );

    expect(code).toBe(0);
    expect(recorded).toHaveLength(0);
    expect(stdout.written).toContain('my todo');
    expect(stdout.written).toContain('Status (SINGLE_SELECT)');
    expect(stdout.written).toContain('Triage');
    expect(stdout.written).toContain('Iteration (ITERATION)');
  });

  it('creates a user project with Status (single_select) and Iteration fields', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();

    const code = await projectsInit(
      ['--template=user', '--title=my todo', '--owner=@me', '--lang=en'],
      {
        client: makeMockClient(recorded),
        stdout,
        readYaml: readUserTemplate,
      }
    );

    expect(code).toBe(0);
    expect(stdout.written).toContain('Project created');
    expect(stdout.written).toContain('Status (SINGLE_SELECT)');
    expect(stdout.written).toContain('Iteration (ITERATION)');

    // Mutation order: GetViewerId → CreateProjectV2 → ListProjectV2Fields →
    // CreateProjectV2Field × 2 (greenfield: nothing pre-exists).
    expect(recorded).toHaveLength(5);
    expect(recorded[0]?.query).toContain('GetViewerId');

    expect(recorded[1]?.query).toContain('CreateProjectV2(');
    expect(recorded[1]?.vars).toEqual({ input: { ownerId: 'U_VIEWER', title: 'my todo' } });

    expect(recorded[2]?.query).toContain('ListProjectV2Fields');

    const statusCall = recorded[3];
    expect(statusCall?.query).toContain('CreateProjectV2Field(');
    expect(statusCall?.vars.input).toMatchObject({
      projectId: 'PVT_NEW',
      name: 'Status',
      dataType: 'SINGLE_SELECT',
    });
    const statusOptions = (statusCall?.vars.input as { singleSelectOptions: { name: string }[] })
      .singleSelectOptions;
    expect(statusOptions.map((o) => o.name)).toEqual(['Triage', 'Todo', 'In Progress', 'Done']);
    // Each option carries a default color + description that the GraphQL
    // schema requires.
    expect(statusOptions[0]).toMatchObject({ color: 'GRAY', description: '' });

    const iterationCall = recorded[4];
    expect(iterationCall?.vars.input).toMatchObject({
      projectId: 'PVT_NEW',
      name: 'Iteration',
      dataType: 'ITERATION',
    });
    expect(iterationCall?.vars.input).not.toHaveProperty('singleSelectOptions');
  });

  it('skips fields that already exist on the new project (e.g. built-in Status)', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();

    const code = await projectsInit(['--template=user', '--title=my todo', '--lang=en'], {
      client: makeMockClient(recorded, { existingFields: ['Title', 'Status', 'Repository'] }),
      stdout,
      readYaml: readUserTemplate,
    });

    expect(code).toBe(0);
    // YAML lists Status + Iteration; Status already exists → only Iteration
    // gets a CreateProjectV2Field call.
    const fieldCalls = recorded.filter((r) => r.query.includes('CreateProjectV2Field('));
    const fieldNames = fieldCalls.map((r) => (r.vars.input as { name: string }).name);
    expect(fieldNames).toEqual(['Iteration']);
    expect(stdout.written).toContain('field skipped');
    expect(stdout.written).toContain('Status');
  });

  it('skips Repository (built-in) when applying the org template', async () => {
    const recorded: RecordedRequest[] = [];
    const stdout = makeStream();

    const code = await projectsInit(
      ['--template=org', '--owner=ozzy-labs', '--title=Platform', '--lang=en'],
      {
        client: makeMockClient(recorded),
        stdout,
        readYaml: readOrgTemplate,
      }
    );

    expect(code).toBe(0);
    expect(recorded[0]?.query).toContain('GetOwnerId');
    expect(recorded[0]?.vars).toEqual({ login: 'ozzy-labs' });

    // Org template has 4 fields but Repository is filtered out, leaving 3
    // CreateProjectV2Field calls (Status, Iteration, Project).
    const fieldNames = recorded
      .filter((r) => r.query.includes('CreateProjectV2Field('))
      .map((r) => (r.vars.input as { name: string }).name);
    expect(fieldNames).toEqual(['Status', 'Iteration', 'Project']);
  });

  it('reports owner not found when GetOwnerId returns null', async () => {
    const stderr = makeStream();
    const stdout = makeStream();

    const client: GraphQLClient = {
      async request<T>(query: string): Promise<T> {
        if (query.includes('GetOwnerId')) {
          return { repositoryOwner: null } as T;
        }
        throw new Error(`unexpected query: ${query}`);
      },
    };

    const code = await projectsInit(
      ['--template=user', '--owner=ghost-user', '--title=x', '--lang=en'],
      {
        client,
        stdout,
        stderr,
        readYaml: readUserTemplate,
      }
    );
    expect(code).toBe(1);
    expect(stderr.written).toContain('ghost-user');
  });

  it('rejects malformed YAML with a readable error', async () => {
    const stderr = makeStream();
    const code = await projectsInit(['some.yaml', '--title=x', '--lang=en'], {
      stderr,
      readYaml: async () => 'fields:\n  - name: bad\n    type: bogus_type\n',
    });
    expect(code).toBe(1);
    expect(stderr.written).toMatch(/bogus_type/);
  });
});
