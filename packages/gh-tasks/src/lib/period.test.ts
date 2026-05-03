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

describe('rangeOf — UTC (backward-compatible default)', () => {
  // 2026-05-03 is a Sunday.
  const SUNDAY = new Date('2026-05-03T12:00:00Z');

  it('daily covers exactly one UTC day', () => {
    const r = rangeOf('daily', SUNDAY, 'UTC');
    expect(r.start.toISOString()).toBe('2026-05-03T00:00:00.000Z');
    expect(r.end.toISOString()).toBe('2026-05-04T00:00:00.000Z');
  });

  it('weekly anchors on Monday', () => {
    const r = rangeOf('weekly', SUNDAY, 'UTC');
    expect(r.start.toISOString()).toBe('2026-04-27T00:00:00.000Z');
    expect(r.end.toISOString()).toBe('2026-05-04T00:00:00.000Z');
  });

  it('sprint covers 14 days from today', () => {
    const r = rangeOf('sprint', SUNDAY, 'UTC');
    expect(r.start.toISOString()).toBe('2026-05-03T00:00:00.000Z');
    expect(r.end.toISOString()).toBe('2026-05-17T00:00:00.000Z');
  });
});

describe('rangeOf — Asia/Tokyo (UTC+9, no DST)', () => {
  // 2026-05-03T12:00Z = JST Sun 2026-05-03 21:00.
  const SUNDAY = new Date('2026-05-03T12:00:00Z');

  it('daily anchors at JST midnight', () => {
    const r = rangeOf('daily', SUNDAY, 'Asia/Tokyo');
    expect(r.start.toISOString()).toBe('2026-05-02T15:00:00.000Z'); // JST 5/3 00:00
    expect(r.end.toISOString()).toBe('2026-05-03T15:00:00.000Z'); // JST 5/4 00:00
  });

  it('weekly anchors on JST Monday', () => {
    const r = rangeOf('weekly', SUNDAY, 'Asia/Tokyo');
    expect(r.start.toISOString()).toBe('2026-04-26T15:00:00.000Z'); // JST 4/27 (Mon) 00:00
    expect(r.end.toISOString()).toBe('2026-05-03T15:00:00.000Z'); // JST 5/4 (Mon) 00:00
  });

  it('sprint covers 14 JST days from today', () => {
    const r = rangeOf('sprint', SUNDAY, 'Asia/Tokyo');
    expect(r.start.toISOString()).toBe('2026-05-02T15:00:00.000Z'); // JST 5/3 00:00
    expect(r.end.toISOString()).toBe('2026-05-16T15:00:00.000Z'); // JST 5/17 00:00
  });

  it('an instant just after JST midnight is in the new JST day', () => {
    // 2026-05-02T15:30Z = JST 5/3 00:30 (so "today" = JST 5/3, not JST 5/2).
    const justAfterJstMidnight = new Date('2026-05-02T15:30:00Z');
    const r = rangeOf('daily', justAfterJstMidnight, 'Asia/Tokyo');
    expect(r.start.toISOString()).toBe('2026-05-02T15:00:00.000Z');
    expect(r.end.toISOString()).toBe('2026-05-03T15:00:00.000Z');
  });
});

describe('rangeOf — DST transitions', () => {
  it('America/New_York spring-forward keeps daily as 24h-ish (23h on the spring DST day)', () => {
    // Spring-forward 2026: clocks jump 02:00 → 03:00 EST→EDT on Sun 2026-03-08.
    // Anchor on the previous Saturday so the daily range crosses the gap.
    const beforeDst = new Date('2026-03-07T20:00:00Z'); // EST Sat 15:00
    const r = rangeOf('daily', beforeDst, 'America/New_York');
    expect(r.start.toISOString()).toBe('2026-03-07T05:00:00.000Z'); // EST midnight
    expect(r.end.toISOString()).toBe('2026-03-08T05:00:00.000Z'); // next EST midnight (still EST since DST starts at 02:00 local)
  });

  it('America/New_York weekly straddles the spring-forward', () => {
    // Use a Monday before spring-forward (2026-03-02) to anchor.
    const monday = new Date('2026-03-02T15:00:00Z'); // EST Mon 10:00
    const r = rangeOf('weekly', monday, 'America/New_York');
    expect(r.start.toISOString()).toBe('2026-03-02T05:00:00.000Z'); // EST Mon 00:00
    // Following Monday is after spring-forward, so EDT (UTC-4): 04:00 UTC = EDT 00:00.
    expect(r.end.toISOString()).toBe('2026-03-09T04:00:00.000Z');
  });
});

describe('rangeOf — TZ resolution priority', () => {
  it('honors an explicit `tz` argument over process.env.TZ', () => {
    const original = process.env.TZ;
    process.env.TZ = 'Asia/Tokyo';
    try {
      const r = rangeOf('daily', new Date('2026-05-03T12:00:00Z'), 'UTC');
      expect(r.start.toISOString()).toBe('2026-05-03T00:00:00.000Z');
    } finally {
      if (original === undefined) delete process.env.TZ;
      else process.env.TZ = original;
    }
  });

  it('falls back to UTC when given an invalid IANA name', () => {
    const r = rangeOf('daily', new Date('2026-05-03T12:00:00Z'), 'Not/A_Real_Zone');
    expect(r.start.toISOString()).toBe('2026-05-03T00:00:00.000Z');
    expect(r.end.toISOString()).toBe('2026-05-04T00:00:00.000Z');
  });
});

describe('suggestMilestoneTitle', () => {
  const NOW = new Date('2026-05-03T12:00:00Z');

  it('formats a daily title (UTC)', () => {
    expect(suggestMilestoneTitle('daily', NOW, 'UTC')).toBe('Daily 2026-05-03');
  });

  it('formats a weekly title from Monday anchor (UTC)', () => {
    expect(suggestMilestoneTitle('weekly', NOW, 'UTC')).toBe('Week of 2026-04-27');
  });

  it('formats a sprint title from today anchor (UTC)', () => {
    expect(suggestMilestoneTitle('sprint', NOW, 'UTC')).toBe('Sprint 2026-05-03');
  });

  it('uses the JST civil date when tz is Asia/Tokyo', () => {
    // NOW is JST 5/3 21:00 — same civil date as UTC, so the title is unchanged.
    expect(suggestMilestoneTitle('daily', NOW, 'Asia/Tokyo')).toBe('Daily 2026-05-03');
  });

  it('uses the JST civil date even when UTC is the previous day', () => {
    // 2026-05-02T16:00Z = JST 5/3 01:00 — UTC says 5/2, JST says 5/3.
    const lateUtc = new Date('2026-05-02T16:00:00Z');
    expect(suggestMilestoneTitle('daily', lateUtc, 'Asia/Tokyo')).toBe('Daily 2026-05-03');
    expect(suggestMilestoneTitle('daily', lateUtc, 'UTC')).toBe('Daily 2026-05-02');
  });
});
