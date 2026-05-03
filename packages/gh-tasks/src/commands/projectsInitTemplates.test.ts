import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { BUNDLED_ORG_TEMPLATE, BUNDLED_USER_TEMPLATE } from './projectsInitTemplates.ts';

// Guards against drift between the YAML SSOT under packages/templates/ and the
// inlined copies in projectsInitTemplates.ts. The inlined copies exist because
// `bun --compile` does not embed sibling YAML files (see ADR-0001), so the
// `--template user|org` shortcut needs the content at compile time.
//
// If you intentionally update the YAML, copy the new content into
// projectsInitTemplates.ts (preserving the backtick-template-literal escapes
// for backslashes / `${`).

const TEMPLATES_DIR = resolve(__dirname, '../../../templates/projects-v2');

describe('bundled projects-v2 templates', () => {
  it('user.yaml matches BUNDLED_USER_TEMPLATE', () => {
    const onDisk = readFileSync(resolve(TEMPLATES_DIR, 'user.yaml'), 'utf8');
    expect(BUNDLED_USER_TEMPLATE).toBe(onDisk);
  });

  it('org.yaml matches BUNDLED_ORG_TEMPLATE', () => {
    const onDisk = readFileSync(resolve(TEMPLATES_DIR, 'org.yaml'), 'utf8');
    expect(BUNDLED_ORG_TEMPLATE).toBe(onDisk);
  });
});
