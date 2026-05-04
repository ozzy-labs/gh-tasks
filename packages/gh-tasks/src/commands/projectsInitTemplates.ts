// Inlined copies of `packages/templates/projects-v2/{user,org}.yaml`.
//
// `gh tasks projects init --template user|org` needs the template content at
// runtime, but `bun --compile` produces a single-file binary that does not
// embed sibling YAML files (cf. repo-internal ADR-0001). Inlining as string
// constants keeps the convenience flag working from a compiled binary.
//
// **Stay in sync** with `packages/templates/projects-v2/{user,org}.yaml` —
// the YAML files are still the SSOT for documentation and any consumer that
// reads the templates directly.

export const BUNDLED_USER_TEMPLATE = `# GitHub Projects v2 field template - user (personal) scope
#
# Used by \`gh-tasks\` user scope (personal Project v2). Defines the minimum
# fields required for \`gh tasks plan\` (Iteration) and standup/review
# (Status) to operate.
#
# Iteration field duration / start date are intentionally omitted: the
# GitHub UI is the authoritative configuration surface for those (the
# REST/GraphQL API only accepts the iteration list, not the cadence).
# Configure them under Project settings -> Iteration after creation.
name: gh-tasks user scope
description: Personal Project v2 fields for gh-tasks (user scope)
fields:
  - name: Status
    type: single_select
    options:
      - Triage
      - Todo
      - In Progress
      - Done
  - name: Iteration
    type: iteration
`;

export const BUNDLED_ORG_TEMPLATE = `# GitHub Projects v2 field template - org (team) scope
#
# Used by \`gh-tasks\` org scope (organization-wide Project v2).
# Extends the user template with cross-repo coordination fields:
#
# - Repository: built-in field type for the owning repo of an item.
# - Project: free-form single_select identifying a logical project that
#   spans multiple repos (e.g. "Platform", "Docs", "Infra"). Edit the
#   options list to match your org's project taxonomy.
#
# Iteration cadence is configured in the GitHub UI (see user.yaml note).
name: gh-tasks org scope
description: Team Project v2 fields for gh-tasks (org scope)
fields:
  - name: Status
    type: single_select
    options:
      - Triage
      - Todo
      - In Progress
      - Done
  - name: Iteration
    type: iteration
  - name: Repository
    type: repository
  - name: Project
    type: single_select
    options:
      - Platform
      - Docs
      - Infra
`;
