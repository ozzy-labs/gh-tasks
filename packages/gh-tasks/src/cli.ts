#!/usr/bin/env bun
import { add } from './commands/add.ts';
import { list } from './commands/list.ts';
import { today } from './commands/today.ts';
import { resolveLocale, t } from './i18n/index.ts';

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

function printHelp(): void {
  const locale = resolveLocale(process.argv);
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

  if (args.length === 0 || args[0] === '--help' || args[0] === '-h') {
    printHelp();
    return 0;
  }

  if (args[0] === '--version' || args[0] === '-v') {
    process.stdout.write(`gh-tasks ${VERSION}\n`);
    return 0;
  }

  const cmd = args[0] as Command;
  if (!COMMANDS.includes(cmd)) {
    const locale = resolveLocale(argv);
    process.stderr.write(`${t(locale, 'error.unknownCommand')}: ${cmd}\n`);
    return 1;
  }

  // Subcommand dispatch — implementations land in src/commands/{cmd}.ts
  if (cmd === 'add') {
    return add(args.slice(1));
  }
  if (cmd === 'list') {
    return list(args.slice(1));
  }
  if (cmd === 'today') {
    return today(args.slice(1));
  }

  const locale = resolveLocale(argv);
  process.stderr.write(`${t(locale, 'error.notImplemented')}: gh tasks ${cmd}\n`);
  return 2;
}

const exitCode = await main(process.argv);
process.exit(exitCode);
