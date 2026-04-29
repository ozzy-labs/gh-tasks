import en from './en.json' with { type: 'json' };
import ja from './ja.json' with { type: 'json' };

type Locale = 'ja' | 'en';
type Messages = Record<string, string>;

const TABLES: Record<Locale, Messages> = { ja, en };

export function resolveLocale(argv: readonly string[]): Locale {
  const langFlag = argv.find((a) => a.startsWith('--lang='))?.split('=')[1];
  if (langFlag === 'ja' || langFlag === 'en') return langFlag;

  const env = process.env.LC_ALL ?? process.env.LANG ?? '';
  if (env.startsWith('ja')) return 'ja';

  return 'en';
}

export function t(locale: Locale, key: string): string {
  return TABLES[locale][key] ?? TABLES.en[key] ?? key;
}
