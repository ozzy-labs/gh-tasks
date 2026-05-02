/**
 * Resolve the authenticated user's login. Used by `gh tasks standup --mine`
 * to filter activity to the viewer.
 */
export const GET_VIEWER_LOGIN = /* GraphQL */ `
  query GetViewerLogin {
    viewer {
      login
    }
  }
`;

export interface GetViewerLoginResponse {
  viewer: { login: string };
}
