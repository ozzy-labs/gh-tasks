//go:build e2e

package e2e

import "testing"

// TestE2E_StandupSmoke_* verifies `gh tasks standup` reaches GraphQL and
// returns without error in each scope. Default 24h window is used; the
// `--mine` flag adds an extra filter pass and is exercised once in the
// org scope to keep the smoke quick.

func TestE2E_StandupSmoke_Repo(t *testing.T) {
	_, stderr, code := runBin(t, "-s", "repo", "-r", testRepo, "standup")
	if code != 0 {
		t.Fatalf("standup -s repo: stderr=%q", stderr)
	}
}

func TestE2E_StandupSmoke_Org(t *testing.T) {
	_, stderr, code := runBin(t, "-s", "org", "-p", testOrgProject, "standup", "--mine")
	if code != 0 {
		t.Fatalf("standup -s org --mine: stderr=%q", stderr)
	}
}

func TestE2E_StandupSmoke_User(t *testing.T) {
	_, stderr, code := runBin(t, "-s", "user", "-p", testUserProject, "standup")
	if code != 0 {
		t.Fatalf("standup -s user: stderr=%q", stderr)
	}
}
