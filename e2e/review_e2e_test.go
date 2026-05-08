//go:build e2e

package e2e

import "testing"

// TestE2E_ReviewSmoke_* verifies `gh tasks review --period <p>` runs in
// each scope. Tests the daily / weekly / sprint code paths once each
// distributed across scopes to keep the smoke compact.
//
// `sprint` is only valid for org / user (Iteration field), so the repo
// case exercises `weekly` (Milestone-based).

func TestE2E_ReviewSmoke_Repo(t *testing.T) {
	_, stderr, code := runBin(t,
		"-s", "repo", "-r", testRepo,
		"review", "--period", "weekly",
	)
	if code != 0 {
		t.Fatalf("review -s repo --period weekly: stderr=%q", stderr)
	}
}

func TestE2E_ReviewSmoke_Org(t *testing.T) {
	_, stderr, code := runBin(t,
		"-s", "org", "-p", testOrgProject,
		"review", "--period", "sprint",
	)
	if code != 0 {
		t.Fatalf("review -s org --period sprint: stderr=%q", stderr)
	}
}

func TestE2E_ReviewSmoke_User(t *testing.T) {
	_, stderr, code := runBin(t,
		"-s", "user", "-p", testUserProject,
		"review", "--period", "daily",
	)
	if code != 0 {
		t.Fatalf("review -s user --period daily: stderr=%q", stderr)
	}
}
