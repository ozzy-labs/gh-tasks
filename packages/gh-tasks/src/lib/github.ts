import { graphql } from '@octokit/graphql';
import { request } from '@octokit/request';
import { RequestError } from '@octokit/request-error';

export type I18nArgs = Readonly<Record<string, string | number>>;

export class AuthError extends Error {
  readonly i18nKey: string;
  readonly i18nArgs: I18nArgs;
  constructor(i18nKey: string, args: I18nArgs = {}) {
    super(i18nKey);
    this.name = 'AuthError';
    this.i18nKey = i18nKey;
    this.i18nArgs = args;
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
 * gh extensions inherit `gh auth token` via the `GH_TOKEN` env. We also
 * fall back to `GITHUB_TOKEN` so this works in GitHub Actions runners and
 * local shells where users export it directly.
 */
export function resolveToken(env: NodeJS.ProcessEnv = process.env): string {
  const token = env.GH_TOKEN ?? env.GITHUB_TOKEN;
  if (!token) {
    throw new AuthError('error.auth.tokenMissing');
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
