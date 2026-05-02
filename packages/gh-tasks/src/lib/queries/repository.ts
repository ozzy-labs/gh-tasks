/**
 * Look up a repository's node id (required as input to `createIssue`).
 *
 * Variables:
 *   - $owner: repository owner login (`<owner>` of `<owner>/<name>`)
 *   - $name:  repository name
 */
export const GET_REPOSITORY_ID = /* GraphQL */ `
  query GetRepositoryId($owner: String!, $name: String!) {
    repository(owner: $owner, name: $name) {
      id
    }
  }
`;

export interface GetRepositoryIdResponse {
  repository: { id: string } | null;
}
