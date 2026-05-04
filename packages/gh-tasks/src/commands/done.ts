import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { createClient, type GraphQLClient, resolveToken } from '../lib/github.ts';
import { ProjectError, type ProjectRef, resolveProjectRef } from '../lib/project.ts';
import { resolveProjectNodeId } from '../lib/projectItem.ts';
import {
  CLOSE_ISSUE,
  type CloseIssueResponse,
  GET_ISSUE_BY_NUMBER,
  type GetIssueByNumberResponse,
  LIST_PROJECT_V2_FIELDS,
  LIST_PROJECT_V2_ITEMS,
  type ListProjectV2FieldsResponse,
  type ListProjectV2ItemsResponse,
  type ProjectV2FieldNode,
  type ProjectV2ItemNode,
  UPDATE_PROJECT_V2_ITEM_FIELD_VALUE,
  type UpdateProjectV2ItemFieldValueResponse,
} from '../lib/queries/index.ts';
import { resolveRepo } from '../lib/repo.ts';
import { detectScope, type Scope } from '../lib/scope.ts';

export interface DoneCommandDeps {
  client?: GraphQLClient;
  hasGitRemote?: () => boolean;
  getRemoteUrl?: () => string | null;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  config?: AppConfig;
}

const FIELDS_FETCH_LIMIT = 50;
const ITEMS_FETCH_LIMIT = 100;

export async function done(argv: readonly string[], deps: DoneCommandDeps = {}): Promise<number> {
  const stdout = deps.stdout ?? process.stdout;
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  const rawId = parsePositionalId(argv);
  if (rawId === null) {
    stderr.write(`${t(locale, 'error.done.idRequired')}\n`);
    return 2;
  }

  const scope = detectScope({ argv, hasGitRemote: deps.hasGitRemote, config: deps.config });

  if (scope === 'repo') {
    return await doneRepoIssue({ argv, deps, locale, stdout, stderr, rawId });
  }

  return await doneProjectItem({ argv, deps, locale, scope, stdout, stderr, rawId });
}

interface DoneRepoContext {
  argv: readonly string[];
  deps: DoneCommandDeps;
  locale: 'ja' | 'en';
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  rawId: string;
}

async function doneRepoIssue(ctx: DoneRepoContext): Promise<number> {
  const { argv, deps, locale, stdout, stderr, rawId } = ctx;

  const issueNumber = toIssueNumber(rawId);
  if (issueNumber === null) {
    stderr.write(`${t(locale, 'error.done.idRequired')}\n`);
    return 2;
  }

  const repo = resolveRepo({ argv, getRemoteUrl: deps.getRemoteUrl });
  const client = deps.client ?? createClient(resolveToken());

  const issueData = await client.request<GetIssueByNumberResponse>(GET_ISSUE_BY_NUMBER, {
    owner: repo.owner,
    name: repo.name,
    number: issueNumber,
  });
  const issue = issueData.repository?.issue;
  if (!issue) {
    stderr.write(`Issue not found: ${repo.owner}/${repo.name}#${issueNumber}\n`);
    return 1;
  }
  if (issue.state === 'CLOSED') {
    stdout.write(`${t(locale, 'done.alreadyClosed')}: ${issue.url}\n`);
    return 0;
  }

  const closed = await client.request<CloseIssueResponse>(CLOSE_ISSUE, {
    input: { issueId: issue.id },
  });
  stdout.write(`${t(locale, 'done.closed')}: ${closed.closeIssue.issue.url}\n`);
  return 0;
}

interface DoneProjectContext {
  argv: readonly string[];
  deps: DoneCommandDeps;
  locale: 'ja' | 'en';
  scope: Exclude<Scope, 'repo'>;
  stdout: NodeJS.WritableStream;
  stderr: NodeJS.WritableStream;
  rawId: string;
}

async function doneProjectItem(ctx: DoneProjectContext): Promise<number> {
  const { argv, deps, locale, scope, stdout, stderr, rawId } = ctx;

  // For org/user scope `<id>` is the Projects v2 item node id (e.g. `PVTI_xxx`).
  // Looking up a Project item from a repo Issue number is a future extension.
  const itemId = rawId;

  let projectRef: ProjectRef;
  try {
    projectRef = resolveProjectRef({ scope, argv, config: deps.config });
  } catch (err) {
    if (err instanceof ProjectError) {
      stderr.write(`${t(locale, err.i18nKey, err.i18nArgs)}\n`);
      return 2;
    }
    throw err;
  }

  const client = deps.client ?? createClient(resolveToken());
  const projectId = await resolveProjectNodeId({ client, scope, projectRef });
  if (!projectId) {
    stderr.write(
      `project not found: ${projectRef.owner}/${projectRef.number} (--scope ${scope})\n`
    );
    return 1;
  }

  const fieldsData = await client.request<ListProjectV2FieldsResponse>(LIST_PROJECT_V2_FIELDS, {
    projectId,
    first: FIELDS_FETCH_LIMIT,
  });
  if (!fieldsData.node) {
    stderr.write(
      `project not found: ${projectRef.owner}/${projectRef.number} (--scope ${scope})\n`
    );
    return 1;
  }

  const statusField = findStatusField(fieldsData.node.fields.nodes);
  if (!statusField) {
    stderr.write(`${t(locale, 'error.done.statusFieldMissing')}\n`);
    return 1;
  }
  const doneOption = statusField.options.find((o) => o.name.toLowerCase() === 'done');
  if (!doneOption) {
    stderr.write(`${t(locale, 'error.done.doneOptionMissing')}\n`);
    return 1;
  }

  const itemsData = await client.request<ListProjectV2ItemsResponse>(LIST_PROJECT_V2_ITEMS, {
    projectId,
    first: ITEMS_FETCH_LIMIT,
  });
  const targetItem = itemsData.node?.items.nodes.find(
    (item: ProjectV2ItemNode) => item.id === itemId
  );
  if (!targetItem) {
    stderr.write(`item not found in project: ${itemId}\n`);
    return 1;
  }

  if (isAlreadyDone(targetItem, statusField.id, doneOption.id)) {
    stdout.write(`${t(locale, 'done.alreadyDone.project')}: ${itemId}\n`);
    return 0;
  }

  await client.request<UpdateProjectV2ItemFieldValueResponse>(UPDATE_PROJECT_V2_ITEM_FIELD_VALUE, {
    input: {
      projectId,
      itemId,
      fieldId: statusField.id,
      value: { singleSelectOptionId: doneOption.id },
    },
  });
  stdout.write(`${t(locale, 'done.statusUpdated.project')}: ${itemId}\n`);
  return 0;
}

interface SingleSelectStatusField {
  id: string;
  name: string;
  options: Array<{ id: string; name: string }>;
}

function findStatusField(fields: readonly ProjectV2FieldNode[]): SingleSelectStatusField | null {
  for (const f of fields) {
    if (f.dataType === 'SINGLE_SELECT' && f.name.toLowerCase() === 'status') {
      return { id: f.id, name: f.name, options: f.options };
    }
  }
  return null;
}

function isAlreadyDone(
  item: ProjectV2ItemNode,
  statusFieldId: string,
  doneOptionId: string
): boolean {
  for (const v of item.fieldValues.nodes) {
    if (
      v.__typename === 'ProjectV2ItemFieldSingleSelectValue' &&
      v.field.id === statusFieldId &&
      v.optionId === doneOptionId
    ) {
      return true;
    }
  }
  return false;
}

/**
 * Pull the first positional argument from argv, returning it as a raw string.
 * In repo scope this is later coerced to an Issue number; in org/user scope it
 * is treated as a Projects v2 item node id (e.g. `PVTI_xxx`).
 */
function parsePositionalId(argv: readonly string[]): string | null {
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;
    if (arg.startsWith('--')) {
      // skip value-flags' values
      if (
        !arg.includes('=') &&
        (arg === '--scope' || arg === '--repo' || arg === '--lang' || arg === '--project')
      ) {
        i++;
      }
      continue;
    }
    return arg;
  }
  return null;
}

function toIssueNumber(value: string): number | null {
  const stripped = value.startsWith('#') ? value.slice(1) : value;
  const n = Number.parseInt(stripped, 10);
  return Number.isFinite(n) && n > 0 ? n : null;
}
