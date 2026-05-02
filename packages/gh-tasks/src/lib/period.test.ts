import { describe, expect, it } from 'vitest';
import { PeriodError, parsePeriodFlag, rangeOf, suggestMilestoneTitle } from './period.ts';

describe('parsePeriodFlag', () => {
  it('returns null when --period is absent', () => {
    expect(parsePeriodFlag([])).toBeNull();
    expect(parsePeriodFlag(['--scope=repo'])).toBeNull();
  });

  it('parses --period=value form', () => {
    expect(parsePeriodFlag(['--period=daily'])).toBe('daily');
    expect(parsePeriodFlag(['--period=weekly'])).toBe('weekly');
    expect(parsePeriodFlag(['--period=sprint'])).toBe('sprint');
  });

  it('parses --period value form', () => {
    expect(parsePeriodFlag(['--period', 'weekly'])).toBe('weekly');
  });

  it('throws on unknown period', () => {
    expect(() => parsePeriodFlag(['--period=monthly'])).toThrow(PeriodError);
  });

  it('throws when --period has no value', () => {
    expect(() => parsePeriodFlag(['--period'])).toThrow(PeriodError);
  });
});

describe('rangeOf', () => {
  // 2026-05-03 is a Sunday.
  const SUNDAY = new Date('2026-05-03T12:00:00Z');

  it('daily covers exactly one UTC day', () => {
    const r = rangeOf('daily', SUNDAY);
    expect(r.start.toISOString()).toBe('2026-05-03T00:00:00.000Z');
    expect(r.end.toISOString()).toBe('2026-05-04T00:00:00.000Z');
  });

  it('weekly anchors on Monday', () => {
    // Sunday → previous Monday (2026-04-27).
    const r = rangeOf('weekly', SUNDAY);
    expect(r.start.toISOString()).toBe('2026-04-27T00:00:00.000Z');
    expect(r.end.toISOString()).toBe('2026-05-04T00:00:00.000Z');
  });

  it('sprint covers 14 days from today', () => {
    const r = rangeOf('sprint', SUNDAY);
    expect(r.start.toISOString()).toBe('2026-05-03T00:00:00.000Z');
    expect(r.end.toISOString()).toBe('2026-05-17T00:00:00.000Z');
  });
});

describe('suggestMilestoneTitle', () => {
  const NOW = new Date('2026-05-03T12:00:00Z');

  it('formats a daily title', () => {
    expect(suggestMilestoneTitle('daily', NOW)).toBe('Daily 2026-05-03');
  });

  it('formats a weekly title from Monday anchor', () => {
    expect(suggestMilestoneTitle('weekly', NOW)).toBe('Week of 2026-04-27');
  });

  it('formats a sprint title from today anchor', () => {
    expect(suggestMilestoneTitle('sprint', NOW)).toBe('Sprint 2026-05-03');
  });
});
