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
