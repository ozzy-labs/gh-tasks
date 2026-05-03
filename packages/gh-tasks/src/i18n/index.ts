import en from './en.json' with { type: 'json' };
import ja from './ja.json' with { type: 'json' };

export type Locale = 'ja' | 'en';
type Messages = Record<string, string>;

const TABLES: Record<Locale, Messages> = { ja, en };

export interface LocaleConfig {
  lang?: Locale;
}

/**
 * Resolve the output locale.
 *
 * Order:
 *   1. `--lang ja|en` / `--lang=ja|en` flag (both forms)
 *   2. `~/.config/ozzylabs/gh-tasks.toml` `lang` (passed via `config`)
 *   3. `LC_ALL` env (`ja*` → ja)
 *   4. `LANG` env (`ja*` → ja)
 *   5. fallback → en
 *
 * Unknown `--lang` values are silently ignored and fall through to the env.
 */
export function resolveLocale(
  argv: readonly string[] = [],
  env: NodeJS.ProcessEnv = process.env,
  config: LocaleConfig = {}
): Locale {
  const flag = parseLangFlag(argv);
  if (flag) return flag;

  if (config.lang) return config.lang;

  // LC_ALL outranks LANG per POSIX: when LC_ALL is set, it is the only env
  // consulted (LANG is not used as a further fallback).
  const lcAll = env.LC_ALL;
  if (typeof lcAll === 'string' && lcAll.length > 0) {
    return lcAll.toLowerCase().startsWith('ja') ? 'ja' : 'en';
  }
  const lang = env.LANG;
  if (typeof lang === 'string' && lang.toLowerCase().startsWith('ja')) {
    return 'ja';
  }
  return 'en';
}

function parseLangFlag(argv: readonly string[]): Locale | null {
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === undefined) continue;
    if (arg.startsWith('--lang=')) {
      return toLocale(arg.slice('--lang='.length));
    }
    if (arg === '--lang') {
      const next = argv[i + 1];
      return next ? toLocale(next) : null;
    }
  }
  return null;
}

function toLocale(value: string): Locale | null {
  return value === 'ja' || value === 'en' ? value : null;
}

export function t(
  locale: Locale,
  key: string,
  args?: Readonly<Record<string, string | number>>
): string {
  let msg = TABLES[locale][key] ?? TABLES.en[key] ?? key;
  if (args) {
    for (const [k, v] of Object.entries(args)) {
      msg = msg.replaceAll(`{${k}}`, String(v));
    }
  }
  return msg;
}
