//go:build e2e

package e2e

import "testing"

// TestE2E_TriageSmoke_* verifies `gh tasks triage` runs in each scope.
// Triage is read-only — it lists items needing attention. Output content
// is project-state-dependent; smoke only asserts exit 0.

func TestE2E_TriageSmoke_Repo(t *testing.T) {
	_, stderr, code := runBin(t,
		"-s", "repo", "-r", testRepo,
		"triage", "--limit", "5",
	)
	if code != 0 {
		t.Fatalf("triage -s repo: stderr=%q", stderr)
	}
}

func TestE2E_TriageSmoke_Org(t *testing.T) {
	_, stderr, code := runBin(t,
		"-s", "org", "-p", testOrgProject,
		"triage", "--limit", "5",
	)
	if code != 0 {
		t.Fatalf("triage -s org: stderr=%q", stderr)
	}
}

func TestE2E_TriageSmoke_User(t *testing.T) {
	_, stderr, code := runBin(t,
		"-s", "user", "-p", testUserProject,
		"triage", "--limit", "5",
	)
	if code != 0 {
		t.Fatalf("triage -s user: stderr=%q", stderr)
	}
}
