/**
 * Create an Issue under a given repository (repo scope).
 *
 * Variables:
 *   - $input: CreateIssueInput { repositoryId, title, body? }
 */
export const CREATE_ISSUE = /* GraphQL */ `
  mutation CreateIssue($input: CreateIssueInput!) {
    createIssue(input: $input) {
      issue {
        id
        number
        url
      }
    }
  }
`;

export interface CreateIssueResponse {
  createIssue: {
    issue: {
      id: string;
      number: number;
      url: string;
    };
  };
}

/**
 * List open Issues under a given repository, ordered by recently updated.
 *
 * Variables:
 *   - $owner / $name: repository identifier
 *   - $first: page size (max 100 per GitHub GraphQL spec)
 */
export const LIST_REPO_ISSUES = /* GraphQL */ `
  query ListRepoIssues($owner: String!, $name: String!, $first: Int!) {
    repository(owner: $owner, name: $name) {
      issues(first: $first, states: OPEN, orderBy: { field: UPDATED_AT, direction: DESC }) {
        nodes {
          id
          number
          title
          url
          updatedAt
        }
      }
    }
  }
`;

export interface RepoIssueNode {
  id: string;
  number: number;
  title: string;
  url: string;
  updatedAt: string;
}

export interface ListRepoIssuesResponse {
  repository: {
    issues: { nodes: RepoIssueNode[] };
  } | null;
}

/**
 * Resolve an Issue's node id from `<owner>/<name>` + issue number.
 * Required input for the `closeIssue` mutation.
 */
export const GET_ISSUE_BY_NUMBER = /* GraphQL */ `
  query GetIssueByNumber($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) {
      issue(number: $number) {
        id
        number
        url
        state
      }
    }
  }
`;

export interface GetIssueByNumberResponse {
  repository: {
    issue: {
      id: string;
      number: number;
      url: string;
      state: 'OPEN' | 'CLOSED';
    } | null;
  } | null;
}

/**
 * Close an Issue (state CLOSED, optional reason).
 *
 * Variables:
 *   - $input: CloseIssueInput { issueId, stateReason? }
 */
export const CLOSE_ISSUE = /* GraphQL */ `
  mutation CloseIssue($input: CloseIssueInput!) {
    closeIssue(input: $input) {
      issue {
        id
        number
        url
        state
      }
    }
  }
`;

export interface CloseIssueResponse {
  closeIssue: {
    issue: {
      id: string;
      number: number;
      url: string;
      state: 'CLOSED';
    };
  };
}

/**
 * List open Issues with their label set so the caller can filter for
 * untriaged items (label-less Issues).
 *
 * Variables:
 *   - $owner / $name / $first
 */
export const LIST_REPO_ISSUES_WITH_LABELS = /* GraphQL */ `
  query ListRepoIssuesWithLabels($owner: String!, $name: String!, $first: Int!) {
    repository(owner: $owner, name: $name) {
      issues(first: $first, states: OPEN, orderBy: { field: UPDATED_AT, direction: DESC }) {
        nodes {
          id
          number
          title
          url
          updatedAt
          labels(first: 20) {
            nodes {
              name
            }
          }
        }
      }
    }
  }
`;

export interface RepoIssueWithLabelsNode {
  id: string;
  number: number;
  title: string;
  url: string;
  updatedAt: string;
  labels: { nodes: Array<{ name: string }> };
}

export interface ListRepoIssuesWithLabelsResponse {
  repository: {
    issues: { nodes: RepoIssueWithLabelsNode[] };
  } | null;
}
