import type { RestClient } from '../github.ts';

/**
 * List open repo Issues with their current Milestone binding. `gh tasks plan`
 * uses this in write mode to skip Issues already attached to a different
 * Milestone (we don't want to silently re-route someone's planning state).
 */
export const LIST_REPO_ISSUES_WITH_MILESTONE = /* GraphQL */ `
  query ListRepoIssuesWithMilestone($owner: String!, $name: String!, $first: Int!) {
    repository(owner: $owner, name: $name) {
      issues(first: $first, states: OPEN, orderBy: { field: UPDATED_AT, direction: DESC }) {
        nodes {
          id
          number
          title
          url
          updatedAt
          milestone {
            id
            number
            title
          }
        }
      }
    }
  }
`;

export interface RepoIssueWithMilestoneNode {
  id: string;
  number: number;
  title: string;
  url: string;
  updatedAt: string;
  milestone: {
    id: string;
    number: number;
    title: string;
  } | null;
}

export interface ListRepoIssuesWithMilestoneResponse {
  repository: {
    issues: { nodes: RepoIssueWithMilestoneNode[] };
  } | null;
}

/**
 * List the most recently updated open Milestones in a repository. Used by
 * `gh tasks plan` to detect a same-titled Milestone before creating a new
 * one (the REST `POST /milestones` endpoint does not deduplicate).
 */
export const LIST_MILESTONES = /* GraphQL */ `
  query ListMilestones($owner: String!, $name: String!, $first: Int!) {
    repository(owner: $owner, name: $name) {
      milestones(
        first: $first
        states: OPEN
        orderBy: { field: UPDATED_AT, direction: DESC }
      ) {
        nodes {
          id
          number
          title
        }
      }
    }
  }
`;

export interface MilestoneNode {
  id: string;
  number: number;
  title: string;
}

export interface ListMilestonesResponse {
  repository: {
    milestones: { nodes: MilestoneNode[] };
  } | null;
}

/**
 * Bind an Issue to a Milestone (or clear the binding when `milestoneId` is
 * null). GraphQL's `updateIssue` mutation accepts `milestoneId` directly, so
 * no REST call is needed here.
 */
export const UPDATE_ISSUE_MILESTONE = /* GraphQL */ `
  mutation UpdateIssueMilestone($input: UpdateIssueInput!) {
    updateIssue(input: $input) {
      issue {
        id
        number
        url
        milestone {
          id
          number
          title
        }
      }
    }
  }
`;

export interface UpdateIssueMilestoneResponse {
  updateIssue: {
    issue: {
      id: string;
      number: number;
      url: string;
      milestone: {
        id: string;
        number: number;
        title: string;
      } | null;
    };
  };
}

export interface CreateMilestoneResult {
  /** GraphQL node id (`MI_*`), required for `updateIssue.milestoneId`. */
  node_id: string;
  /** REST numeric id (not used downstream but logged for debugging). */
  id: number;
  number: number;
  title: string;
}

/**
 * Create a Milestone via REST (`POST /repos/{owner}/{repo}/milestones`).
 *
 * GraphQL v4 has no `createMilestone` mutation as of 2026-05, so the CLI
 * falls back to REST. The response includes `node_id`, which is the
 * GraphQL-relay id required by `updateIssue.milestoneId`.
 */
export async function createMilestone(
  rest: RestClient,
  args: { owner: string; name: string; title: string; description?: string; dueOn?: string }
): Promise<CreateMilestoneResult> {
  const body: Record<string, unknown> = { title: args.title };
  if (args.description !== undefined) body.description = args.description;
  if (args.dueOn !== undefined) body.due_on = args.dueOn;
  return rest.request<CreateMilestoneResult>({
    method: 'POST',
    url: `/repos/${args.owner}/${args.name}/milestones`,
    body,
  });
}
