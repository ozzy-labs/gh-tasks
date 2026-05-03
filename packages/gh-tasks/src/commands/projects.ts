import { resolveLocale, t } from '../i18n/index.ts';
import type { AppConfig } from '../lib/config.ts';
import { type ProjectsInitDeps, projectsInit } from './projectsInit.ts';

export interface ProjectsCommandDeps extends ProjectsInitDeps {
  config?: AppConfig;
}

/**
 * Dispatcher for the two-word `gh tasks projects <subcmd>` namespace.
 *
 * Currently only `init` is implemented; the dispatcher exists so future
 * subcommands (e.g. `projects update`, `projects show`) can be added without
 * threading them through `cli.ts`.
 */
export async function projects(
  argv: readonly string[],
  deps: ProjectsCommandDeps = {}
): Promise<number> {
  const stderr = deps.stderr ?? process.stderr;
  const locale = resolveLocale(argv, process.env, deps.config);

  // Find the first positional argument (skipping flags). This lets users put
  // global flags like --lang before the subcommand, e.g.
  //   gh tasks projects --lang=en init --template user --title ...
  const subIndex = argv.findIndex((a) => !a.startsWith('-'));
  const sub = subIndex === -1 ? undefined : argv[subIndex];
  const subRest =
    subIndex === -1 ? argv : [...argv.slice(0, subIndex), ...argv.slice(subIndex + 1)];

  if (sub === 'init') {
    return await projectsInit(subRest, deps);
  }

  if (sub === undefined) {
    stderr.write(`${t(locale, 'error.projects.subcommandRequired')}\n`);
    return 2;
  }
  stderr.write(`${t(locale, 'error.projects.unknownSubcommand', { sub })}\n`);
  return 2;
}
