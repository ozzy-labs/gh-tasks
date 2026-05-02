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
