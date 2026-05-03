/**
 * Mutations / queries for `gh tasks projects init`.
 *
 * Bootstraps a new Projects v2 board owned by `@me`, a user, or an
 * organization, and adds custom fields from a YAML template.
 */

/**
 * Resolve the authenticated viewer's node id. Used when `--owner @me` is
 * passed (the default). The id is required as the `ownerId` for
 * `createProjectV2`.
 */
export const GET_VIEWER_ID = /* GraphQL */ `
  query GetViewerId {
    viewer {
      id
      login
    }
  }
`;

export interface GetViewerIdResponse {
  viewer: { id: string; login: string };
}

/**
 * Resolve a user's or organization's node id from a login. `repositoryOwner`
 * is an interface implemented by both `User` and `Organization`, so a single
 * lookup covers either ownership type.
 */
export const GET_OWNER_ID = /* GraphQL */ `
  query GetOwnerId($login: String!) {
    repositoryOwner(login: $login) {
      __typename
      id
      login
    }
  }
`;

export interface GetOwnerIdResponse {
  repositoryOwner: {
    __typename: 'User' | 'Organization';
    id: string;
    login: string;
  } | null;
}

/**
 * Create a Projects v2 board owned by `ownerId`.
 *
 * Variables:
 *   - $input: CreateProjectV2Input { ownerId, title }
 */
export const CREATE_PROJECT_V2 = /* GraphQL */ `
  mutation CreateProjectV2($input: CreateProjectV2Input!) {
    createProjectV2(input: $input) {
      projectV2 {
        id
        number
        title
        url
      }
    }
  }
`;

export interface CreateProjectV2Response {
  createProjectV2: {
    projectV2: {
      id: string;
      number: number;
      title: string;
      url: string;
    };
  };
}

/**
 * Add a custom field to an existing Projects v2 board.
 *
 * `dataType` accepts the `ProjectV2CustomFieldType` enum:
 *   TEXT | NUMBER | DATE | SINGLE_SELECT | ITERATION
 *
 * `singleSelectOptions` is required for SINGLE_SELECT and ignored otherwise.
 * Each option needs a `color` per the GraphQL schema; we default to GRAY so
 * the YAML stays minimal.
 */
export const CREATE_PROJECT_V2_FIELD = /* GraphQL */ `
  mutation CreateProjectV2Field($input: CreateProjectV2FieldInput!) {
    createProjectV2Field(input: $input) {
      projectV2Field {
        ... on ProjectV2FieldCommon {
          id
          name
          dataType
        }
      }
    }
  }
`;

export interface CreateProjectV2FieldResponse {
  createProjectV2Field: {
    projectV2Field: {
      id: string;
      name: string;
      dataType: string;
    };
  };
}
