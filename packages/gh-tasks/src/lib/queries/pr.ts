/**
 * Resolve a PullRequest's id and current body from `<owner>/<name>` + number.
 * Required input for `updatePullRequest`.
 */
export const GET_PULL_REQUEST_BY_NUMBER = /* GraphQL */ `
  query GetPullRequestByNumber($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) {
      pullRequest(number: $number) {
        id
        number
        url
        body
      }
    }
  }
`;

export interface GetPullRequestByNumberResponse {
  repository: {
    pullRequest: {
      id: string;
      number: number;
      url: string;
      body: string;
    } | null;
  } | null;
}

/**
 * Update a PullRequest's body (and other mutable fields).
 *
 * Variables:
 *   - $input: UpdatePullRequestInput { pullRequestId, body? }
 */
export const UPDATE_PULL_REQUEST = /* GraphQL */ `
  mutation UpdatePullRequest($input: UpdatePullRequestInput!) {
    updatePullRequest(input: $input) {
      pullRequest {
        id
        number
        url
      }
    }
  }
`;

export interface UpdatePullRequestResponse {
  updatePullRequest: {
    pullRequest: {
      id: string;
      number: number;
      url: string;
    };
  };
}

/**
 * List recently MERGED PullRequests. The caller filters by `mergedAt`.
 */
export const LIST_MERGED_PRS = /* GraphQL */ `
  query ListMergedPRs($owner: String!, $name: String!, $first: Int!) {
    repository(owner: $owner, name: $name) {
      pullRequests(first: $first, states: MERGED, orderBy: { field: UPDATED_AT, direction: DESC }) {
        nodes {
          id
          number
          title
          url
          mergedAt
          author {
            login
          }
          assignees(first: 10) {
            nodes {
              login
            }
          }
        }
      }
    }
  }
`;

export interface MergedPRNode {
  id: string;
  number: number;
  title: string;
  url: string;
  mergedAt: string;
  author?: { login: string } | null;
  assignees?: { nodes: Array<{ login: string }> };
}

export interface ListMergedPRsResponse {
  repository: { pullRequests: { nodes: MergedPRNode[] } } | null;
}
