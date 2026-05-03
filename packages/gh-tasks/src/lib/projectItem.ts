/**
 * Shared helpers for working with Projects v2 items.
 *
 * These were previously duplicated as private functions across many
 * `commands/*.ts` files. The behavior of each helper exported here matches
 * the pre-existing implementations exactly — this module is a refactor
 * landing zone, not a place to change formatting or semantics.
 */

import type { GraphQLClient } from './github.ts';
import type { ProjectRef } from './project.ts';
import {
  GET_ORG_PROJECT_V2,
  GET_USER_PROJECT_V2,
  type GetOrgProjectV2Response,
  type GetUserProjectV2Response,
  type ProjectV2FieldValue,
  type ProjectV2ItemNode,
} from './queries/index.ts';
import type { Scope } from './scope.ts';

export interface ResolveProjectNodeIdOptions {
  client: GraphQLClient;
  scope: Exclude<Scope, 'repo'>;
  projectRef: ProjectRef;
}

/**
 * Resolve a `ProjectRef` (owner + number) to its Projects v2 node id by
 * issuing the appropriate GraphQL query for the scope. Returns `null` when
 * the project cannot be found (for example: wrong owner, wrong number, or
 * insufficient scopes on the token).
 *
 * Behavior preserved from the prior per-file copies in `add` / `list` /
 * `today` / `done` / `link` / `triage` / `plan` / `review` / `standup`,
 * which were byte-identical.
 */
export async function resolveProjectNodeId(
  opts: ResolveProjectNodeIdOptions
): Promise<string | null> {
  const { client, scope, projectRef } = opts;
  if (scope === 'org') {
    const data = await client.request<GetOrgProjectV2Response>(GET_ORG_PROJECT_V2, {
      login: projectRef.owner,
      number: projectRef.number,
    });
    return data.organization?.projectV2?.id ?? null;
  }
  const data = await client.request<GetUserProjectV2Response>(GET_USER_PROJECT_V2, {
    login: projectRef.owner,
    number: projectRef.number,
  });
  return data.user?.projectV2?.id ?? null;
}

/**
 * Find the value of a Projects v2 single-select field conventionally named
 * "Status" (case-insensitive). Returns the option name (e.g. "Done", "In
 * Progress") when present, or `null` when the item has no Status set or
 * Status is not a single-select field on this project.
 */
export function findStatus(values: readonly ProjectV2FieldValue[]): string | null {
  for (const v of values) {
    if (
      v.__typename === 'ProjectV2ItemFieldSingleSelectValue' &&
      v.field.name.toLowerCase() === 'status'
    ) {
      return v.name;
    }
  }
  return null;
}

/**
 * Multi-line "list" rendering of a Projects v2 item.
 *
 * Format matches the prior `formatItem` in `list.ts` / `today.ts` /
 * `triage.ts` exactly:
 *
 * - Issue / PullRequest: `<prefix>#<n>  <title>[  [Status]]\n  <url>\n`
 *   where `<prefix>` is `"PR"` for PullRequest and empty string for Issue.
 * - DraftIssue (no number / url): `(draft)  <title>[  [Status]]\n`.
 * - No content: `(no content)[  [Status]]\n`.
 *
 * Trailing newlines are intentional — callers `stdout.write(formatItem(item))`
 * directly without adding their own newline.
 */
export function formatItem(item: ProjectV2ItemNode): string {
  const status = findStatus(item.fieldValues.nodes);
  const statusSuffix = status ? `  [${status}]` : '';
  const content = item.content;
  if (!content) {
    return `(no content)${statusSuffix}\n`;
  }
  if (content.__typename === 'Issue' || content.__typename === 'PullRequest') {
    const prefix = content.__typename === 'PullRequest' ? 'PR' : '';
    return `${prefix}#${content.number}  ${content.title}${statusSuffix}\n  ${content.url}\n`;
  }
  // DraftIssue: no number/url.
  return `(draft)  ${content.title}${statusSuffix}\n`;
}

/**
 * Single-line "compact" rendering of a Projects v2 item, used when the
 * caller embeds the line into a bulleted Markdown list and wants the URL
 * inline rather than on a second line.
 *
 * Format matches the prior `formatItemLine` in `review.ts` / `standup.ts`
 * exactly (no leading indent, no trailing newline, URL inline in
 * parens, no Status suffix):
 *
 * - Issue / PullRequest: `<prefix>#<n> <title> (<url>)`
 * - DraftIssue: `(draft) <title>`
 * - No content: `(no content)`
 *
 * NOTE: `plan.ts` has a *different* `formatItemLine` (2-space leading
 * indent, trailing newline, no URL) that is intentionally NOT replaced
 * here — that variant is local to the plan write-mode preview and
 * preserving its exact byte output is more important than dedup.
 */
export function formatItemLineCompact(item: ProjectV2ItemNode): string {
  const c = item.content;
  if (!c) return '(no content)';
  if (c.__typename === 'Issue' || c.__typename === 'PullRequest') {
    const prefix = c.__typename === 'PullRequest' ? 'PR' : '';
    return `${prefix}#${c.number} ${c.title} (${c.url})`;
  }
  return `(draft) ${c.title}`;
}
