#!/usr/bin/env bun
import { add } from './commands/add.ts';
import { done } from './commands/done.ts';
import { link } from './commands/link.ts';
import { list } from './commands/list.ts';
import { plan } from './commands/plan.ts';
import { review } from './commands/review.ts';
import { standup } from './commands/standup.ts';
import { today } from './commands/today.ts';
import { triage } from './commands/triage.ts';
import { resolveLocale, t } from './i18n/index.ts';
import { type AppConfig, ConfigError, loadConfig } from './lib/config.ts';

const VERSION = '0.0.0';

const COMMANDS = [
  'add',
  'list',
  'today',
  'plan',
  'triage',
  'done',
  'review',
  'standup',
  'link',
] as const;

type Command = (typeof COMMANDS)[number];

function printHelp(config: AppConfig): void {
  const locale = resolveLocale(process.argv, process.env, config);
  process.stdout.write(`${t(locale, 'help.header')}\n\n`);
  process.stdout.write(`${t(locale, 'help.usage')}\n\n`);
  process.stdout.write(`${t(locale, 'help.commands')}\n`);
  for (const cmd of COMMANDS) {
    process.stdout.write(`  ${cmd.padEnd(10)} ${t(locale, `help.cmd.${cmd}`)}\n`);
  }
  process.stdout.write(`\n${t(locale, 'help.flags')}\n`);
}

async function main(argv: string[]): Promise<number> {
  const [, , ...args] = argv;

  let config: AppConfig;
  try {
    config = loadConfig();
  } catch (err) {
    if (err instanceof ConfigError) {
      process.stderr.write(`${err.message}\n`);
      return 2;
    }
    throw err;
  }

  if (args.length === 0 || args[0] === '--help' || args[0] === '-h') {
    printHelp(config);
    return 0;
  }

  if (args[0] === '--version' || args[0] === '-v') {
    process.stdout.write(`gh-tasks ${VERSION}\n`);
    return 0;
  }

  const cmd = args[0] as Command;
  if (!COMMANDS.includes(cmd)) {
    const locale = resolveLocale(argv, process.env, config);
    process.stderr.write(`${t(locale, 'error.unknownCommand')}: ${cmd}\n`);
    return 1;
  }

  const rest = args.slice(1);
  // Subcommand dispatch — implementations land in src/commands/{cmd}.ts
  if (cmd === 'add') {
    return add(rest, { config });
  }
  if (cmd === 'list') {
    return list(rest, { config });
  }
  if (cmd === 'today') {
    return today(rest, { config });
  }
  if (cmd === 'done') {
    return done(rest, { config });
  }
  if (cmd === 'link') {
    return link(rest, { config });
  }
  if (cmd === 'triage') {
    return triage(rest, { config });
  }
  if (cmd === 'plan') {
    return plan(rest, { config });
  }
  if (cmd === 'review') {
    return review(rest, { config });
  }
  if (cmd === 'standup') {
    return standup(rest, { config });
  }

  const locale = resolveLocale(argv, process.env, config);
  process.stderr.write(`${t(locale, 'error.notImplemented')}: gh tasks ${cmd}\n`);
  return 2;
}

const exitCode = await main(process.argv);
process.exit(exitCode);
