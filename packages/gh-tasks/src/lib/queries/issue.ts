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
