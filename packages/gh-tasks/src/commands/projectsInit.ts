import { readFile } from 'node:fs/promises';
import { parse as parseYaml } from 'yaml';
import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import {
  CREATE_PROJECT_V2,
  CREATE_PROJECT_V2_FIELD,
  type CreateProjectV2FieldResponse,
  type CreateProjectV2Response,
  GET_OWNER_ID,
  GET_VIEWER_ID,
  type GetOwnerIdResponse,
  type GetViewerIdResponse,
  LIST_PROJECT_V2_FIELDS,
  type ListProjectV2FieldsResponse,
} from '../lib/queries/index.ts';
import { BUNDLED_ORG_TEMPLATE, BUNDLED_USER_TEMPLATE } from './projectsInitTemplates.ts';

export interface ProjectsInitDeps {
  client?: GraphQLClient;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  config?: AppConfig;
  /** Read a YAML file. Defaults to `node:fs/promises#readFile`. Tests inject. */
  readYaml?: (path: string) => Promise<string>;
}

export type ProjectV2CustomFieldType = 'TEXT' | 'NUMBER' | 'DATE' | 'SINGLE_SELECT' | 'ITERATION';

interface TemplateField {
  name: string;
  type: 'text' | 'number' | 'date' | 'single_select' | 'iteration' | 'repository';
  options?: string[];
}

interface ParsedTemplate {
  fields: TemplateField[];
}

interface ParsedArgs {
  yamlPath: string | null;
  template: 'user' | 'org' | null;
  owner: string;
  title: string | null;
  dryRun: boolean;
}

/**
 * Bootstrap a Projects v2 board from a YAML field template.
 *
 * Routing:
 *   - `gh tasks projects init` → this function (positional arg `init` already
 *     stripped by the `projects` dispatcher)
 *
 * Steps:
 *   1. Resolve owner node id (viewer for `@me`, otherwise repositoryOwner)
 *   2. createProjectV2(ownerId, title) → projectId
 *   3. For each YAML field:
 *      - skip type=repository (built-in on every Projects v2 board)
 *      - createProjectV2Field with the matching `ProjectV2CustomFieldType`
 *
 * Built-in fields (Status with default Todo / In Progress / Done options,
 * Repository, etc.) are auto-attached on `createProjectV2`, so calling
 * `createProjectV2Field` for them would 422. We list the post-creation
 * fields and drop YAML entries whose name (case-insensitive) is already
 * present, reporting the skip in stdout so the user knows.
 */
export async function projectsInit(
  argv: readonly string[],
  deps: ProjectsInitDeps = {}
): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const args = parseArgs(argv);

  if (args.title === null) {
    stderr.write(`${t(locale, 'error.projectsInit.titleRequired')}\n`);
    return 2;
  }

  if (args.yamlPath === null && args.template === null) {
    stderr.write(`${t(locale, 'error.projectsInit.templateRequired')}\n`);
    return 2;
  }
  if (args.yamlPath !== null && args.template !== null) {
    stderr.write(`${t(locale, 'error.projectsInit.templateConflict')}\n`);
    return 2;
  }

  let template: ParsedTemplate;
  try {
    const raw = await loadTemplateRaw(args, deps);
    template = parseTemplate(raw);
  } catch (err) {
    const source = args.yamlPath ?? `--template ${args.template}`;
    stderr.write(
      `${t(locale, 'error.projectsInit.yamlRead', { path: source, reason: errMessage(err) })}\n`
    );
    return 1;
  }

  const fieldsToCreate = template.fields
    .filter((f) => f.type !== 'repository')
    .map(toMutationInput);

  if (args.dryRun) {
    stdout.write(
      `${t(locale, 'projectsInit.dryRunHeader', { title: args.title, owner: args.owner })}\n`
    );
    for (const field of fieldsToCreate) {
      const opts = field.singleSelectOptions
        ? ` [${field.singleSelectOptions.map((o) => o.name).join(', ')}]`
        : '';
      stdout.write(`  - ${field.name} (${field.dataType})${opts}\n`);
    }
    return 0;
  }

  const client = deps.client ?? createClient(resolveToken());
  const ownerId = await resolveOwnerId(client, args.owner);
  if (ownerId === null) {
    stderr.write(`${t(locale, 'error.projectsInit.ownerNotFound', { owner: args.owner })}\n`);
    return 1;
  }

  const projectData = await client.request<CreateProjectV2Response>(CREATE_PROJECT_V2, {
    input: { ownerId, title: args.title },
  });
  const project = projectData.createProjectV2.projectV2;
  stdout.write(`${t(locale, 'projectsInit.created', { url: project.url })}\n`);

  const existingFields = await client.request<ListProjectV2FieldsResponse>(LIST_PROJECT_V2_FIELDS, {
    projectId: project.id,
    first: 100,
  });
  const existingNames = new Set(
    (existingFields.node?.fields.nodes ?? []).map((f) => f.name.toLowerCase())
  );

  for (const field of fieldsToCreate) {
    if (existingNames.has(field.name.toLowerCase())) {
      stdout.write(`${t(locale, 'projectsInit.fieldSkipped', { name: field.name })}\n`);
      continue;
    }
    const created = await client.request<CreateProjectV2FieldResponse>(CREATE_PROJECT_V2_FIELD, {
      input: { projectId: project.id, ...field },
    });
    stdout.write(
      `${t(locale, 'projectsInit.fieldCreated', {
        name: created.createProjectV2Field.projectV2Field.name,
        dataType: created.createProjectV2Field.projectV2Field.dataType,
      })}\n`
    );
  }

  return 0;
}

interface CreateFieldInput {
  dataType: ProjectV2CustomFieldType;
  name: string;
  singleSelectOptions?: Array<{ name: string; color: string; description: string }>;
}

function toMutationInput(field: TemplateField): CreateFieldInput {
  const dataType = templateTypeToDataType(field.type);
  if (dataType === 'SINGLE_SELECT') {
    return {
      dataType,
      name: field.name,
      singleSelectOptions: (field.options ?? []).map((name) => ({
        name,
        // GitHub's GraphQL schema requires a color and description per option.
        // GRAY + empty description keeps the YAML minimal; users can recolor
        // in the UI afterwards.
        color: 'GRAY',
        description: '',
      })),
    };
  }
  return { dataType, name: field.name };
}

function templateTypeToDataType(type: TemplateField['type']): ProjectV2CustomFieldType {
  switch (type) {
    case 'text':
      return 'TEXT';
    case 'number':
      return 'NUMBER';
    case 'date':
      return 'DATE';
    case 'single_select':
      return 'SINGLE_SELECT';
    case 'iteration':
      return 'ITERATION';
    case 'repository':
      // Filtered out before this map; assertion to satisfy exhaustiveness.
      throw new Error("unreachable: 'repository' is built-in and must be skipped");
  }
}

async function resolveOwnerId(client: GraphQLClient, owner: string): Promise<string | null> {
  if (owner === '@me') {
    const data = await client.request<GetViewerIdResponse>(GET_VIEWER_ID);
    return data.viewer.id;
  }
  const data = await client.request<GetOwnerIdResponse>(GET_OWNER_ID, { login: owner });
  return data.repositoryOwner?.id ?? null;
}

function parseTemplate(raw: string): ParsedTemplate {
  const doc = parseYaml(raw);
  if (!doc || typeof doc !== 'object' || !Array.isArray((doc as { fields?: unknown }).fields)) {
    throw new Error("template must have a 'fields' array");
  }
  const fields: TemplateField[] = [];
  for (const entry of (doc as { fields: unknown[] }).fields) {
    if (!entry || typeof entry !== 'object') continue;
    const f = entry as { name?: unknown; type?: unknown; options?: unknown };
    if (typeof f.name !== 'string' || typeof f.type !== 'string') {
      throw new Error('each field must have string name and type');
    }
    const type = f.type as TemplateField['type'];
    if (
      type !== 'text' &&
      type !== 'number' &&
      type !== 'date' &&
      type !== 'single_select' &&
      type !== 'iteration' &&
      type !== 'repository'
    ) {
      throw new Error(`unsupported field type: '${type}' (field '${f.name}')`);
    }
    const field: TemplateField = { name: f.name, type };
    if (type === 'single_select') {
      if (!Array.isArray(f.options) || f.options.some((o) => typeof o !== 'string')) {
        throw new Error(`single_select field '${f.name}' requires a string options[] array`);
      }
      field.options = f.options as string[];
    }
    fields.push(field);
  }
  return { fields };
}

/**
 * Resolve the YAML content for the requested source.
 *
 * - `--template user|org` returns the inlined string constant. The constants
 *   are checked against the on-disk YAML by `projectsInitTemplates.test.ts`,
 *   so they cannot drift silently.
 * - A positional YAML path goes through `deps.readYaml` (defaulting to
 *   `fs.readFile`) so users can point at any file.
 */
async function loadTemplateRaw(args: ParsedArgs, deps: ProjectsInitDeps): Promise<string> {
  if (args.template === 'user') return BUNDLED_USER_TEMPLATE;
  if (args.template === 'org') return BUNDLED_ORG_TEMPLATE;
  // args.yamlPath must be set here per the earlier validation.
  const reader = deps.readYaml ?? defaultReadYaml;
  return reader(args.yamlPath as string);
}

function defaultReadYaml(path: string): Promise<string> {
  return readFile(path, 'utf8');
}

function errMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

const VALUE_FLAGS = new Set([
  '--scope',
  '--repo',
  '--lang',
  '--template',
  '--owner',
  '--title',
  '--project',
]);

function parseArgs(argv: readonly string[]): ParsedArgs {
  let yamlPath: string | null = null;
  let template: 'user' | 'org' | null = null;
  let owner = '@me';
  let title: string | null = null;
  let dryRun = false;

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;

    if (arg === '--dry-run') {
      dryRun = true;
      continue;
    }
    if (arg.startsWith('--template=')) {
      template = parseTemplateValue(arg.slice('--template='.length));
      continue;
    }
    if (arg === '--template') {
      template = parseTemplateValue(argv[i + 1]);
      i++;
      continue;
    }
    if (arg.startsWith('--owner=')) {
      owner = arg.slice('--owner='.length);
      continue;
    }
    if (arg === '--owner') {
      const next = argv[i + 1];
      if (next !== undefined) owner = next;
      i++;
      continue;
    }
    if (arg.startsWith('--title=')) {
      title = arg.slice('--title='.length);
      continue;
    }
    if (arg === '--title') {
      const next = argv[i + 1];
      if (next !== undefined) title = next;
      i++;
      continue;
    }
    if (arg.startsWith('--')) {
      if (!arg.includes('=') && VALUE_FLAGS.has(arg)) {
        i++;
      }
      continue;
    }
    // Positional: yaml path
    if (yamlPath === null) {
      yamlPath = arg;
    }
  }

  return { yamlPath, template, owner, title, dryRun };
}

function parseTemplateValue(value: string | undefined): 'user' | 'org' | null {
  if (value === 'user' || value === 'org') return value;
  return null;
}
