import { describe, expect, it, vi } from 'vitest';
import type { GraphQLClient } from './github.ts';
import {
  findStatus,
  formatItem,
  formatItemLineCompact,
  resolveProjectNodeId,
} from './projectItem.ts';
import type { ProjectV2FieldValue, ProjectV2ItemNode } from './queries/index.ts';

describe('findStatus', () => {
  it('returns the option name when a Status single-select value exists', () => {
    const values: ProjectV2FieldValue[] = [
      {
        __typename: 'ProjectV2ItemFieldSingleSelectValue',
        optionId: 'opt1',
        name: 'In Progress',
        field: { id: 'f1', name: 'Status' },
      },
    ];
    expect(findStatus(values)).toBe('In Progress');
  });

  it('matches the Status field name case-insensitively', () => {
    const values: ProjectV2FieldValue[] = [
      {
        __typename: 'ProjectV2ItemFieldSingleSelectValue',
        optionId: 'opt1',
        name: 'Done',
        field: { id: 'f1', name: 'STATUS' },
      },
    ];
    expect(findStatus(values)).toBe('Done');
  });

  it('returns null when Status is not present', () => {
    const values: ProjectV2FieldValue[] = [
      {
        __typename: 'ProjectV2ItemFieldTextValue',
        text: 'note',
        field: { id: 'f1', name: 'Notes' },
      },
    ];
    expect(findStatus(values)).toBeNull();
  });

  it('ignores non-single-select fields named "Status"', () => {
    const values: ProjectV2FieldValue[] = [
      {
        __typename: 'ProjectV2ItemFieldTextValue',
        text: 'free-form',
        field: { id: 'f1', name: 'Status' },
      },
    ];
    expect(findStatus(values)).toBeNull();
  });

  it('returns null for an empty values array', () => {
    expect(findStatus([])).toBeNull();
  });
});

describe('formatItem', () => {
  it('renders an Issue with status suffix and URL on its own line', () => {
    const item: ProjectV2ItemNode = {
      id: 'i1',
      updatedAt: '2026-05-03T00:00:00Z',
      content: {
        __typename: 'Issue',
        id: 'gh1',
        number: 42,
        title: 'Fix bug',
        url: 'https://github.com/o/r/issues/42',
        state: 'OPEN',
        updatedAt: '2026-05-03T00:00:00Z',
        closedAt: null,
        author: null,
        assignees: { nodes: [] },
      },
      fieldValues: {
        nodes: [
          {
            __typename: 'ProjectV2ItemFieldSingleSelectValue',
            optionId: 'opt1',
            name: 'Todo',
            field: { id: 'f1', name: 'Status' },
          },
        ],
      },
    };
    expect(formatItem(item)).toBe('#42  Fix bug  [Todo]\n  https://github.com/o/r/issues/42\n');
  });

  it('renders a PullRequest with a "PR" prefix', () => {
    const item: ProjectV2ItemNode = {
      id: 'i2',
      updatedAt: '2026-05-03T00:00:00Z',
      content: {
        __typename: 'PullRequest',
        id: 'gh2',
        number: 7,
        title: 'Add feature',
        url: 'https://github.com/o/r/pull/7',
        state: 'OPEN',
        updatedAt: '2026-05-03T00:00:00Z',
        mergedAt: null,
        author: null,
        assignees: { nodes: [] },
      },
      fieldValues: { nodes: [] },
    };
    expect(formatItem(item)).toBe('PR#7  Add feature\n  https://github.com/o/r/pull/7\n');
  });

  it('renders a DraftIssue without number/url', () => {
    const item: ProjectV2ItemNode = {
      id: 'i3',
      updatedAt: '2026-05-03T00:00:00Z',
      content: {
        __typename: 'DraftIssue',
        id: 'gh3',
        title: 'Idea',
        body: null,
      },
      fieldValues: { nodes: [] },
    };
    expect(formatItem(item)).toBe('(draft)  Idea\n');
  });

  it('renders "(no content)" when content is null', () => {
    const item: ProjectV2ItemNode = {
      id: 'i4',
      updatedAt: '2026-05-03T00:00:00Z',
      content: null,
      fieldValues: {
        nodes: [
          {
            __typename: 'ProjectV2ItemFieldSingleSelectValue',
            optionId: 'opt1',
            name: 'Done',
            field: { id: 'f1', name: 'Status' },
          },
        ],
      },
    };
    expect(formatItem(item)).toBe('(no content)  [Done]\n');
  });
});

describe('formatItemLineCompact', () => {
  it('renders an Issue inline with URL in parens', () => {
    const item: ProjectV2ItemNode = {
      id: 'i1',
      updatedAt: '2026-05-03T00:00:00Z',
      content: {
        __typename: 'Issue',
        id: 'gh1',
        number: 12,
        title: 'Fix bug',
        url: 'https://github.com/o/r/issues/12',
        state: 'OPEN',
        updatedAt: '2026-05-03T00:00:00Z',
        closedAt: null,
        author: null,
        assignees: { nodes: [] },
      },
      fieldValues: { nodes: [] },
    };
    expect(formatItemLineCompact(item)).toBe('#12 Fix bug (https://github.com/o/r/issues/12)');
  });

  it('renders a PullRequest with a "PR" prefix and no trailing newline', () => {
    const item: ProjectV2ItemNode = {
      id: 'i2',
      updatedAt: '2026-05-03T00:00:00Z',
      content: {
        __typename: 'PullRequest',
        id: 'gh2',
        number: 9,
        title: 'Refactor',
        url: 'https://github.com/o/r/pull/9',
        state: 'OPEN',
        updatedAt: '2026-05-03T00:00:00Z',
        mergedAt: null,
        author: null,
        assignees: { nodes: [] },
      },
      fieldValues: { nodes: [] },
    };
    expect(formatItemLineCompact(item)).toBe('PR#9 Refactor (https://github.com/o/r/pull/9)');
  });

  it('renders a DraftIssue without URL', () => {
    const item: ProjectV2ItemNode = {
      id: 'i3',
      updatedAt: '2026-05-03T00:00:00Z',
      content: {
        __typename: 'DraftIssue',
        id: 'gh3',
        title: 'Plan something',
        body: null,
      },
      fieldValues: { nodes: [] },
    };
    expect(formatItemLineCompact(item)).toBe('(draft) Plan something');
  });

  it('returns "(no content)" when content is null', () => {
    const item: ProjectV2ItemNode = {
      id: 'i4',
      updatedAt: '2026-05-03T00:00:00Z',
      content: null,
      fieldValues: { nodes: [] },
    };
    expect(formatItemLineCompact(item)).toBe('(no content)');
  });
});

describe('resolveProjectNodeId', () => {
  it('queries the org project for scope=org and returns its id', async () => {
    const request = vi.fn().mockResolvedValue({
      organization: { projectV2: { id: 'PVT_org_1' } },
    });
    const client: GraphQLClient = { request } as unknown as GraphQLClient;

    const id = await resolveProjectNodeId({
      client,
      scope: 'org',
      projectRef: { owner: 'octo', number: 3 },
    });

    expect(id).toBe('PVT_org_1');
    expect(request).toHaveBeenCalledTimes(1);
    const firstCall = request.mock.calls[0];
    expect(firstCall).toBeDefined();
    expect(firstCall?.[1]).toEqual({ login: 'octo', number: 3 });
  });

  it('queries the user project for scope=user and returns its id', async () => {
    const request = vi.fn().mockResolvedValue({
      user: { projectV2: { id: 'PVT_user_1' } },
    });
    const client: GraphQLClient = { request } as unknown as GraphQLClient;

    const id = await resolveProjectNodeId({
      client,
      scope: 'user',
      projectRef: { owner: 'alice', number: 5 },
    });

    expect(id).toBe('PVT_user_1');
    expect(request).toHaveBeenCalledTimes(1);
  });

  it('returns null when the project is missing', async () => {
    const request = vi.fn().mockResolvedValue({ user: null });
    const client: GraphQLClient = { request } as unknown as GraphQLClient;

    const id = await resolveProjectNodeId({
      client,
      scope: 'user',
      projectRef: { owner: 'ghost', number: 99 },
    });

    expect(id).toBeNull();
  });
});
