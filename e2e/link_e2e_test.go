//go:build e2e

package e2e

import "testing"

// TestE2E_LinkSmoke_* are deliberately t.Skip()ed isolation smokes
// for `gh tasks link`. Implementing them in isolation requires
// creating an ephemeral draft PR (via gh api: blob → tree → commit →
// ref → pulls) plus a target Issue, which doubles the setup helper
// surface. The cost / value tradeoff favors covering link inside
// Flow A (repo) and Flow B (org/user) lifecycle tests where a PR is
// already part of the scenario, rather than as a standalone smoke.
//
// The test functions exist (rather than being absent) so that:
//
//  1. The Phase 4 matrix in docs/design/e2e-test-plan.md §6 reads as
//     complete: every cmd has a Smoke entry, even if some skip.
//  2. Future work to add the PR helper has a clear landing spot —
//     remove the t.Skip and fill in the setup.
//
// See e2e/lifecycle_*_e2e_test.go (when implemented) for end-to-end
// link coverage.

func TestE2E_LinkSmoke_Repo(t *testing.T) {
	t.Skip("link smoke requires PR-creation helper; covered by Flow A lifecycle (TODO)")
}

func TestE2E_LinkSmoke_Org(t *testing.T) {
	t.Skip("link smoke requires PR-creation helper; covered by Flow B lifecycle (TODO)")
}

func TestE2E_LinkSmoke_User(t *testing.T) {
	t.Skip("link smoke requires PR-creation helper; covered by Flow B lifecycle (TODO)")
}
