#!/usr/bin/env node
// Build the gh-tasks CLI for all supported platforms via Bun --compile.
// Outputs to packages/gh-tasks/bin/gh-tasks-{os}-{arch}.
import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const ROOT = resolve(dirname(__filename), '..');
const ENTRY = resolve(ROOT, 'packages/gh-tasks/src/cli.ts');
const OUT_DIR = resolve(ROOT, 'packages/gh-tasks/bin');

mkdirSync(OUT_DIR, { recursive: true });

const TARGETS = [
  { triple: 'bun-darwin-x64', name: 'gh-tasks-darwin-amd64' },
  { triple: 'bun-darwin-arm64', name: 'gh-tasks-darwin-arm64' },
  { triple: 'bun-linux-x64', name: 'gh-tasks-linux-amd64' },
  { triple: 'bun-linux-arm64', name: 'gh-tasks-linux-arm64' },
  { triple: 'bun-windows-x64', name: 'gh-tasks-windows-amd64.exe' },
];

for (const { triple, name } of TARGETS) {
  const out = resolve(OUT_DIR, name);
  console.log(`[build-cli] ${triple} → ${out}`);
  execFileSync('bun', ['build', ENTRY, '--compile', `--target=${triple}`, `--outfile=${out}`], {
    stdio: 'inherit',
  });
}
