import { graphql } from '@octokit/graphql';
import { RequestError } from '@octokit/request-error';

export class AuthError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AuthError';
  }
}

export class GraphQLClientError extends Error {
  constructor(
    message: string,
    public readonly status: number | undefined,
    public override readonly cause: unknown
  ) {
    super(message);
    this.name = 'GraphQLClientError';
  }
}

export interface GraphQLClient {
  request<T>(query: string, vars?: Record<string, unknown>): Promise<T>;
}

/**
 * Resolve the GitHub token from the environment.
 *
 * gh extensions inherit `gh auth token` via the `GH_TOKEN` env per
 * handbook ADR-0022. We also fall back to `GITHUB_TOKEN` so this works in
 * GitHub Actions runners and local shells where users export it directly.
 */
export function resolveToken(env: NodeJS.ProcessEnv = process.env): string {
  const token = env.GH_TOKEN ?? env.GITHUB_TOKEN;
  if (!token) {
    throw new AuthError('GH_TOKEN / GITHUB_TOKEN が未設定。`gh auth login` を実行してください。');
  }
  return token;
}

export function createClient(token: string): GraphQLClient {
  const client = graphql.defaults({
    headers: { authorization: `token ${token}` },
  });
  return {
    async request<T>(query: string, vars: Record<string, unknown> = {}): Promise<T> {
      try {
        return (await client<T>(query, vars)) as T;
      } catch (err) {
        if (err instanceof RequestError) {
          throw new GraphQLClientError(err.message, err.status, err);
        }
        throw err;
      }
    },
  };
}
