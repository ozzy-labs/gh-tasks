import { describe, expect, it } from 'vitest';
import { AuthError, resolveToken } from './github.ts';

describe('resolveToken', () => {
  it('prefers GH_TOKEN', () => {
    expect(resolveToken({ GH_TOKEN: 'gh-token', GITHUB_TOKEN: 'gha-token' })).toBe('gh-token');
  });

  it('falls back to GITHUB_TOKEN', () => {
    expect(resolveToken({ GITHUB_TOKEN: 'gha-token' })).toBe('gha-token');
  });

  it('throws AuthError when neither is set', () => {
    expect(() => resolveToken({})).toThrow(AuthError);
  });
});
