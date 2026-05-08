//go:build e2e

package e2e

import "testing"

// TestE2E_TodaySmoke_* verifies `gh tasks today` runs successfully across
// all three scopes. The output content depends on real test-project state
// (which iteration is current, what items are due) so the smoke only
// asserts exit 0 — output shape is pinned by cmd/today_flow_test.go
// against mocks.

func TestE2E_TodaySmoke_Repo(t *testing.T) {
	_, stderr, code := runBin(t, "-s", "repo", "-r", testRepo, "today")
	if code != 0 {
		t.Fatalf("today -s repo: stderr=%q", stderr)
	}
}

func TestE2E_TodaySmoke_Org(t *testing.T) {
	_, stderr, code := runBin(t, "-s", "org", "-p", testOrgProject, "today")
	if code != 0 {
		t.Fatalf("today -s org: stderr=%q", stderr)
	}
}

func TestE2E_TodaySmoke_User(t *testing.T) {
	_, stderr, code := runBin(t, "-s", "user", "-p", testUserProject, "today")
	if code != 0 {
		t.Fatalf("today -s user: stderr=%q", stderr)
	}
}
