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

/**
 * Resolve a Projects v2 node id from `<user-login> + number`. Used for the
 * `user` scope, where the project belongs to a user account.
 *
 * `viewer` (the authenticated user) and `user(login: ...)` both return the
 * same `User` type and expose `projectV2(number)`. We always resolve via
 * `user(login)` so the same query covers viewer and shared projects.
 */
export const GET_USER_PROJECT_V2 = /* GraphQL */ `
  query GetUserProjectV2($login: String!, $number: Int!) {
    user(login: $login) {
      projectV2(number: $number) {
        id
        number
        title
      }
    }
  }
`;

export interface GetUserProjectV2Response {
  user: {
    projectV2: {
      id: string;
      number: number;
      title: string;
    } | null;
  } | null;
}

/**
 * Resolve a Projects v2 node id from `<org-login> + number`. Used for the
 * `org` scope.
 */
export const GET_ORG_PROJECT_V2 = /* GraphQL */ `
  query GetOrgProjectV2($login: String!, $number: Int!) {
    organization(login: $login) {
      projectV2(number: $number) {
        id
        number
        title
      }
    }
  }
`;

export interface GetOrgProjectV2Response {
  organization: {
    projectV2: {
      id: string;
      number: number;
      title: string;
    } | null;
  } | null;
}

/**
 * List Projects v2 fields with type-specific extras (single-select options,
 * iteration configurations). Commands use this to map a `Status` / `Iteration`
 * name → field id + option id pair before issuing
 * `updateProjectV2ItemFieldValue`.
 */
export const LIST_PROJECT_V2_FIELDS = /* GraphQL */ `
  query ListProjectV2Fields($projectId: ID!, $first: Int!) {
    node(id: $projectId) {
      ... on ProjectV2 {
        fields(first: $first) {
          nodes {
            ... on ProjectV2FieldCommon {
              id
              name
              dataType
            }
            ... on ProjectV2SingleSelectField {
              id
              name
              dataType
              options {
                id
                name
              }
            }
            ... on ProjectV2IterationField {
              id
              name
              dataType
              configuration {
                iterations {
                  id
                  title
                  startDate
                  duration
                }
                completedIterations {
                  id
                  title
                  startDate
                  duration
                }
              }
            }
          }
        }
      }
    }
  }
`;

export interface ProjectV2IterationOption {
  id: string;
  title: string;
  startDate: string;
  duration: number;
}

export type ProjectV2FieldNode =
  | {
      id: string;
      name: string;
      dataType: 'TEXT' | 'NUMBER' | 'DATE' | 'TITLE' | 'ASSIGNEES' | 'LABELS' | 'REPOSITORY';
    }
  | {
      id: string;
      name: string;
      dataType: 'SINGLE_SELECT';
      options: Array<{ id: string; name: string }>;
    }
  | {
      id: string;
      name: string;
      dataType: 'ITERATION';
      configuration: {
        iterations: ProjectV2IterationOption[];
        completedIterations: ProjectV2IterationOption[];
      };
    };

export interface ListProjectV2FieldsResponse {
  node: {
    fields: { nodes: ProjectV2FieldNode[] };
  } | null;
}

/**
 * List items in a Projects v2 board. The `content` union covers Issue / PR
 * (linked to a repo) and DraftIssue (project-internal). Field values are
 * returned per-item with their containing field id, so callers can index by
 * field name client-side.
 */
export const LIST_PROJECT_V2_ITEMS = /* GraphQL */ `
  query ListProjectV2Items($projectId: ID!, $first: Int!) {
    node(id: $projectId) {
      ... on ProjectV2 {
        items(first: $first) {
          nodes {
            id
            updatedAt
            content {
              __typename
              ... on Issue {
                id
                number
                title
                url
                state
                updatedAt
                closedAt
                author {
                  login
                }
                assignees(first: 10) {
                  nodes {
                    login
                  }
                }
              }
              ... on PullRequest {
                id
                number
                title
                url
                state
                updatedAt
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
              ... on DraftIssue {
                id
                title
                body
              }
            }
            fieldValues(first: 30) {
              nodes {
                __typename
                ... on ProjectV2ItemFieldSingleSelectValue {
                  optionId
                  name
                  field {
                    ... on ProjectV2FieldCommon {
                      id
                      name
                    }
                  }
                }
                ... on ProjectV2ItemFieldIterationValue {
                  iterationId
                  title
                  startDate
                  duration
                  field {
                    ... on ProjectV2FieldCommon {
                      id
                      name
                    }
                  }
                }
                ... on ProjectV2ItemFieldTextValue {
                  text
                  field {
                    ... on ProjectV2FieldCommon {
                      id
                      name
                    }
                  }
                }
                ... on ProjectV2ItemFieldDateValue {
                  date
                  field {
                    ... on ProjectV2FieldCommon {
                      id
                      name
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
`;

export type ProjectV2ItemContent =
  | {
      __typename: 'Issue';
      id: string;
      number: number;
      title: string;
      url: string;
      state: 'OPEN' | 'CLOSED';
      updatedAt: string;
      closedAt: string | null;
      author?: { login: string } | null;
      assignees?: { nodes: Array<{ login: string }> };
    }
  | {
      __typename: 'PullRequest';
      id: string;
      number: number;
      title: string;
      url: string;
      state: 'OPEN' | 'CLOSED' | 'MERGED';
      updatedAt: string;
      mergedAt: string | null;
      author?: { login: string } | null;
      assignees?: { nodes: Array<{ login: string }> };
    }
  | {
      __typename: 'DraftIssue';
      id: string;
      title: string;
      body: string | null;
    };

export type ProjectV2FieldValue =
  | {
      __typename: 'ProjectV2ItemFieldSingleSelectValue';
      optionId: string;
      name: string;
      field: { id: string; name: string };
    }
  | {
      __typename: 'ProjectV2ItemFieldIterationValue';
      iterationId: string;
      title: string;
      startDate: string;
      duration: number;
      field: { id: string; name: string };
    }
  | {
      __typename: 'ProjectV2ItemFieldTextValue';
      text: string;
      field: { id: string; name: string };
    }
  | {
      __typename: 'ProjectV2ItemFieldDateValue';
      date: string;
      field: { id: string; name: string };
    };

export interface ProjectV2ItemNode {
  id: string;
  updatedAt: string;
  content: ProjectV2ItemContent | null;
  fieldValues: { nodes: ProjectV2FieldValue[] };
}

export interface ListProjectV2ItemsResponse {
  node: {
    items: { nodes: ProjectV2ItemNode[] };
  } | null;
}

/**
 * Update a single field value on a Projects v2 item. The `value` shape
 * varies by field type:
 *   - single-select: `{ singleSelectOptionId }`
 *   - iteration: `{ iterationId }`
 *   - text: `{ text }`
 *   - date: `{ date }`
 *   - number: `{ number }`
 *
 * Callers construct the input variant matching the target field's `dataType`.
 */
export const UPDATE_PROJECT_V2_ITEM_FIELD_VALUE = /* GraphQL */ `
  mutation UpdateProjectV2ItemFieldValue($input: UpdateProjectV2ItemFieldValueInput!) {
    updateProjectV2ItemFieldValue(input: $input) {
      projectV2Item {
        id
      }
    }
  }
`;

export interface UpdateProjectV2ItemFieldValueResponse {
  updateProjectV2ItemFieldValue: {
    projectV2Item: { id: string };
  };
}
