#!/usr/bin/env bun
import { add } from './commands/add.ts';
import { done } from './commands/done.ts';
import { link } from './commands/link.ts';
import { list } from './commands/list.ts';
import { plan } from './commands/plan.ts';
import { projects } from './commands/projects.ts';
import { review } from './commands/review.ts';
import { standup } from './commands/standup.ts';
import { today } from './commands/today.ts';
import { triage } from './commands/triage.ts';
import { resolveLocale, t } from './i18n/index.ts';
import { type AppConfig, ConfigError, loadConfig } from './lib/config.ts';
import { AuthError } from './lib/github.ts';
import { PeriodError } from './lib/period.ts';
import { ProjectError } from './lib/project.ts';
import { RepoError } from './lib/repo.ts';
import { ScopeError } from './lib/scope.ts';

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
  'projects',
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
      // Locale cannot be drawn from config (config load failed). Resolve
      // from argv + env only.
      const locale = resolveLocale(argv, process.env);
      process.stderr.write(`${err.name}: ${t(locale, err.i18nKey, err.i18nArgs)}\n`);
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
  try {
    // Subcommand dispatch — implementations land in src/commands/{cmd}.ts
    if (cmd === 'add') {
      return await add(rest, { config });
    }
    if (cmd === 'list') {
      return await list(rest, { config });
    }
    if (cmd === 'today') {
      return await today(rest, { config });
    }
    if (cmd === 'done') {
      return await done(rest, { config });
    }
    if (cmd === 'link') {
      return await link(rest, { config });
    }
    if (cmd === 'triage') {
      return await triage(rest, { config });
    }
    if (cmd === 'plan') {
      return await plan(rest, { config });
    }
    if (cmd === 'review') {
      return await review(rest, { config });
    }
    if (cmd === 'standup') {
      return await standup(rest, { config });
    }
    if (cmd === 'projects') {
      return await projects(rest, { config });
    }
  } catch (err) {
    if (
      err instanceof AuthError ||
      err instanceof RepoError ||
      err instanceof ScopeError ||
      err instanceof ProjectError ||
      err instanceof PeriodError
    ) {
      const locale = resolveLocale(argv, process.env, config);
      process.stderr.write(`${err.name}: ${t(locale, err.i18nKey, err.i18nArgs)}\n`);
      return 2;
    }
    throw err;
  }

  const locale = resolveLocale(argv, process.env, config);
  process.stderr.write(`${t(locale, 'error.notImplemented')}: gh tasks ${cmd}\n`);
  return 2;
}

const exitCode = await main(process.argv);
process.exit(exitCode);
