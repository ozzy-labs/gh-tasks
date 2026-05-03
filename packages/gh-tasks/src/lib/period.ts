/**
 * Period helpers for `gh tasks plan` / `gh tasks review` / `gh tasks standup`.
 *
 * Periods are anchored at local-midnight in the user's IANA timezone. The
 * timezone is resolved (in priority order) from the `tz` argument, the `TZ`
 * environment variable, then `Intl.DateTimeFormat().resolvedOptions().timeZone`.
 * If none of these yield a usable IANA name (e.g. CI runners with `TZ=UTC` or
 * unset), boundaries fall back to UTC midnight, preserving the previous
 * behavior. Returned `Date` objects are absolute UTC instants representing
 * those local midnights.
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
  /** Inclusive start (local midnight, expressed as a UTC instant). */
  start: Date;
  /** Exclusive end (local midnight, expressed as a UTC instant). */
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

/**
 * Resolve the IANA timezone to use for boundary computation.
 *
 * - When an explicit `tz` is passed, use it if valid; if invalid, fall back
 *   directly to UTC (do NOT silently substitute the system tz — that would
 *   surprise the caller).
 * - When `tz` is undefined, try `process.env.TZ`, then the system default,
 *   and finally fall back to UTC.
 *
 * Returns `null` to signal "use UTC fallback" so callers can keep their
 * legacy UTC-anchored code path.
 */
function resolveTz(tz?: string): string | null {
  if (typeof tz === 'string') {
    return isValidTz(tz) ? tz : null;
  }
  for (const candidate of [process.env.TZ, systemTz()]) {
    if (candidate && isValidTz(candidate)) return candidate;
  }
  return null;
}

function systemTz(): string | undefined {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch {
    return undefined;
  }
}

function isValidTz(tz: string): boolean {
  try {
    // Throws RangeError when the timezone is not a valid IANA name.
    new Intl.DateTimeFormat('en-CA', { timeZone: tz });
    return true;
  } catch {
    return false;
  }
}

interface LocalDate {
  year: number;
  month: number; // 1..12
  day: number; // 1..31
  weekday: number; // 0..6, Sunday=0 (matches JS Date#getUTCDay)
}

const WEEKDAY_INDEX: Record<string, number> = {
  Sun: 0,
  Mon: 1,
  Tue: 2,
  Wed: 3,
  Thu: 4,
  Fri: 5,
  Sat: 6,
};

/**
 * Project a UTC instant onto its local civil date in `tz`. Uses
 * `Intl.DateTimeFormat` so DST shifts are handled correctly.
 */
function localDateIn(d: Date, tz: string): LocalDate {
  const fmt = new Intl.DateTimeFormat('en-CA', {
    timeZone: tz,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    weekday: 'short',
  });
  const parts = fmt.formatToParts(d);
  let year = 0;
  let month = 0;
  let day = 0;
  let weekday = 0;
  for (const p of parts) {
    if (p.type === 'year') year = Number.parseInt(p.value, 10);
    else if (p.type === 'month') month = Number.parseInt(p.value, 10);
    else if (p.type === 'day') day = Number.parseInt(p.value, 10);
    else if (p.type === 'weekday') weekday = WEEKDAY_INDEX[p.value] ?? 0;
  }
  return { year, month, day, weekday };
}

/**
 * Render a UTC instant as `[year, month, day, hour, minute, second]` in `tz`.
 * Used to compute the UTC↔tz offset by comparing the same instant rendered
 * in `tz` vs `'UTC'`.
 */
function wallClockMs(d: Date, tz: string): number {
  const fmt = new Intl.DateTimeFormat('en-US', {
    timeZone: tz,
    hourCycle: 'h23',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
  const parts = fmt.formatToParts(d);
  const m: Record<string, number> = {};
  for (const p of parts) {
    if (p.type === 'literal') continue;
    m[p.type] = Number.parseInt(p.value, 10);
  }
  return Date.UTC(
    m.year ?? 0,
    (m.month ?? 1) - 1,
    m.day ?? 1,
    m.hour ?? 0,
    m.minute ?? 0,
    m.second ?? 0
  );
}

/**
 * Return the UTC instant matching `YYYY-MM-DD 00:00:00` in `tz`.
 *
 * Compute the offset between `tz` and `UTC` at the candidate instant by
 * rendering both views and taking the wall-clock delta. Subtracting that
 * offset from the naïve UTC midnight gives the actual UTC instant for local
 * midnight. A second pass stabilises the answer across DST transitions
 * where the offset depends on the (initially unknown) target instant.
 */
function localMidnightUtc(year: number, month: number, day: number, tz: string): Date {
  const naive = Date.UTC(year, month - 1, day);
  let candidate = naive;
  for (let i = 0; i < 2; i++) {
    const offset = wallClockMs(new Date(candidate), tz) - candidate;
    candidate = naive - offset;
  }
  return new Date(candidate);
}

/**
 * Add `days` calendar days in `tz` and return the UTC instant of that local
 * midnight. Crosses DST cleanly because we re-anchor on local midnight after
 * advancing the civil date.
 */
function addDaysLocal(start: Date, days: number, tz: string): Date {
  const local = localDateIn(start, tz);
  // Use a UTC date as a calendar arithmetic helper, then re-anchor.
  const tmp = new Date(Date.UTC(local.year, local.month - 1, local.day));
  tmp.setUTCDate(tmp.getUTCDate() + days);
  return localMidnightUtc(tmp.getUTCFullYear(), tmp.getUTCMonth() + 1, tmp.getUTCDate(), tz);
}

export function rangeOf(period: Period, now: Date, tz?: string): DateRange {
  const resolved = resolveTz(tz);
  if (resolved === null) {
    return rangeOfUtc(period, now);
  }
  const local = localDateIn(now, resolved);
  const startOfToday = localMidnightUtc(local.year, local.month, local.day, resolved);
  switch (period) {
    case 'daily': {
      const end = addDaysLocal(startOfToday, 1, resolved);
      return { start: startOfToday, end };
    }
    case 'weekly': {
      // Anchor on Monday of the current week. weekday 0=Sun..6=Sat.
      const daysSinceMonday = (local.weekday + 6) % 7;
      const monday = addDaysLocal(startOfToday, -daysSinceMonday, resolved);
      const end = addDaysLocal(monday, 7, resolved);
      return { start: monday, end };
    }
    case 'sprint': {
      const end = addDaysLocal(startOfToday, 14, resolved);
      return { start: startOfToday, end };
    }
  }
}

function rangeOfUtc(period: Period, now: Date): DateRange {
  const start = utcMidnight(now);
  switch (period) {
    case 'daily': {
      const end = new Date(start);
      end.setUTCDate(end.getUTCDate() + 1);
      return { start, end };
    }
    case 'weekly': {
      const day = start.getUTCDay();
      const daysSinceMonday = (day + 6) % 7;
      const monday = new Date(start);
      monday.setUTCDate(monday.getUTCDate() - daysSinceMonday);
      const end = new Date(monday);
      end.setUTCDate(end.getUTCDate() + 7);
      return { start: monday, end };
    }
    case 'sprint': {
      const end = new Date(start);
      end.setUTCDate(end.getUTCDate() + 14);
      return { start, end };
    }
  }
}

function utcMidnight(d: Date): Date {
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()));
}

export function suggestMilestoneTitle(period: Period, now: Date, tz?: string): string {
  const { start } = rangeOf(period, now, tz);
  const iso = formatLocalIsoDate(start, tz);
  switch (period) {
    case 'daily':
      return `Daily ${iso}`;
    case 'weekly':
      return `Week of ${iso}`;
    case 'sprint':
      return `Sprint ${iso}`;
  }
}

/**
 * Format a `Date` as `YYYY-MM-DD` in the chosen timezone. Used by command
 * UIs that display the range bounds — those bounds are absolute UTC instants,
 * but we render them as the local civil date so the displayed string matches
 * the boundary the user specified (e.g. "Week of 2026-04-27" for the JST week
 * starting Mon 04/27 at JST 00:00, which is `2026-04-26T15:00Z`).
 *
 * Falls back to the UTC date string when no usable IANA tz is available.
 */
export function formatLocalIsoDate(d: Date, tz?: string): string {
  const resolved = resolveTz(tz);
  if (resolved === null) return d.toISOString().slice(0, 10);
  const local = localDateIn(d, resolved);
  const mm = String(local.month).padStart(2, '0');
  const dd = String(local.day).padStart(2, '0');
  return `${local.year}-${mm}-${dd}`;
}
