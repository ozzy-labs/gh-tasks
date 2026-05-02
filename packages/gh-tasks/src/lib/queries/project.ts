/**
 * Add a draft item to a Projects v2 board (org / user scope).
 *
 * Variables:
 *   - $input: AddProjectV2DraftIssueInput { projectId, title, body?, assigneeIds? }
 */
export const ADD_PROJECT_V2_DRAFT_ISSUE = /* GraphQL */ `
  mutation AddProjectV2DraftIssue($input: AddProjectV2DraftIssueInput!) {
    addProjectV2DraftIssue(input: $input) {
      projectItem {
        id
      }
    }
  }
`;

export interface AddProjectV2DraftIssueResponse {
  addProjectV2DraftIssue: {
    projectItem: { id: string };
  };
}
