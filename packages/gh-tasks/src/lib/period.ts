/**
 * Period helpers for `gh tasks plan` / `gh tasks review`.
 *
 * Periods are anchored at UTC midnight so behavior is identical across local
 * dev (often JST) and CI runners (UTC). Higher-precision local-time anchoring
 * (e.g. respect the user's TZ for "weekly" boundaries) is deferred.
 */

export type Period = 'daily' | 'weekly' | 'sprint';

export const PERIODS: readonly Period[] = ['daily', 'weekly', 'sprint'];

export class PeriodError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'PeriodError';
  }
}

export interface DateRange {
  /** Inclusive start (UTC midnight). */
  start: Date;
  /** Exclusive end (UTC midnight). */
  end: Date;
}

export function parsePeriodFlag(argv: readonly string[]): Period | null {
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;
    if (arg.startsWith('--period=')) {
      return toPeriod(arg.slice('--period='.length));
    }
    if (arg === '--period') {
      return toPeriod(argv[i + 1]);
    }
  }
  return null;
}

function toPeriod(value: string | undefined): Period {
  if (value === undefined) {
    throw new PeriodError('--period フラグに値が指定されていません');
  }
  if ((PERIODS as readonly string[]).includes(value)) {
    return value as Period;
  }
  throw new PeriodError(`不正な --period 値: '${value}' (有効値: ${PERIODS.join(' | ')})`);
}

export function rangeOf(period: Period, now: Date): DateRange {
  const start = utcMidnight(now);
  switch (period) {
    case 'daily': {
      const end = new Date(start);
      end.setUTCDate(end.getUTCDate() + 1);
      return { start, end };
    }
    case 'weekly': {
      // Anchor on Monday of the current week (UTC). getUTCDay returns 0..6
      // with Sunday = 0. Compute days since the most recent Monday.
      const day = start.getUTCDay();
      const daysSinceMonday = (day + 6) % 7;
      const monday = new Date(start);
      monday.setUTCDate(monday.getUTCDate() - daysSinceMonday);
      const end = new Date(monday);
      end.setUTCDate(end.getUTCDate() + 7);
      return { start: monday, end };
    }
    case 'sprint': {
      // Two-week window ending tomorrow (i.e. next 14 days from start of today).
      const end = new Date(start);
      end.setUTCDate(end.getUTCDate() + 14);
      return { start, end };
    }
  }
}

function utcMidnight(d: Date): Date {
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()));
}

export function suggestMilestoneTitle(period: Period, now: Date): string {
  const { start } = rangeOf(period, now);
  const iso = start.toISOString().slice(0, 10); // YYYY-MM-DD
  switch (period) {
    case 'daily':
      return `Daily ${iso}`;
    case 'weekly':
      return `Week of ${iso}`;
    case 'sprint':
      return `Sprint ${iso}`;
  }
}
