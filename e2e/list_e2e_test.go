//go:build e2e

package e2e

import "testing"

// TestE2E_ListSmoke_Repo / _Org / _User exercise `gh tasks list` in each
// scope with a small --limit, verifying read-side end-to-end including
// scope detection, project / repo resolution, GraphQL pagination, and
// localized output. No mutations.

func TestE2E_ListSmoke_Repo(t *testing.T) {
	_, stderr, code := runBin(t, "-s", "repo", "-r", testRepo, "list", "--limit", "1")
	if code != 0 {
		t.Fatalf("list -s repo -r %s: stderr=%q", testRepo, stderr)
	}
}

func TestE2E_ListSmoke_Org(t *testing.T) {
	_, stderr, code := runBin(t, "-s", "org", "-p", testOrgProject, "list", "--limit", "1")
	if code != 0 {
		t.Fatalf("list -s org -p %s: stderr=%q", testOrgProject, stderr)
	}
}

func TestE2E_ListSmoke_User(t *testing.T) {
	_, stderr, code := runBin(t, "-s", "user", "-p", testUserProject, "list", "--limit", "1")
	if code != 0 {
		t.Fatalf("list -s user -p %s: stderr=%q", testUserProject, stderr)
	}
}
