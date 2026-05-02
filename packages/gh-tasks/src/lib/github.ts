import { graphql } from '@octokit/graphql';
import { request } from '@octokit/request';
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

export interface RestRequestOptions {
  method: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE';
  url: string;
  body?: Record<string, unknown>;
}

export interface RestClient {
  request<T>(options: RestRequestOptions): Promise<T>;
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

/**
 * REST client for endpoints not exposed by the GraphQL v4 schema.
 *
 * GitHub's GraphQL API has no `createMilestone` mutation, so `gh tasks plan`
 * falls back to REST `POST /repos/{owner}/{repo}/milestones`. We keep this
 * surface minimal — only what existing commands need.
 */
export function createRestClient(token: string): RestClient {
  const req = request.defaults({
    headers: { authorization: `token ${token}` },
  });
  return {
    async request<T>(options: RestRequestOptions): Promise<T> {
      try {
        const response = await req({
          method: options.method,
          url: options.url,
          ...(options.body ?? {}),
        });
        return response.data as T;
      } catch (err) {
        if (err instanceof RequestError) {
          throw new GraphQLClientError(err.message, err.status, err);
        }
        throw err;
      }
    },
  };
}
