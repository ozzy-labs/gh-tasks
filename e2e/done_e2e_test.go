//go:build e2e

package e2e

import (
	"strconv"
	"testing"
)

// TestE2E_DoneSmoke_* verifies the `done` mutation roundtrip in each
// scope. Implemented as `add → done` because `done` requires a target
// (we don't want to depend on a long-lived fixture item that another
// test could close).

func TestE2E_DoneSmoke_Repo(t *testing.T) {
	num := addRepoIssue(t, testRepo, uniqueTitle("DoneSmoke_Repo"))
	_, stderr, code := runBin(t,
		"--lang", "en",
		"-s", "repo", "-r", testRepo,
		"done", strconv.Itoa(num),
	)
	if code != 0 {
		t.Fatalf("done #%d: stderr=%q", num, stderr)
	}
}

func TestE2E_DoneSmoke_Org(t *testing.T) {
	id := addProjectDraft(t, "org", testOrgProject, uniqueTitle("DoneSmoke_Org"))
	markProjectItemDone(t, "org", testOrgProject, id)
}

func TestE2E_DoneSmoke_User(t *testing.T) {
	id := addProjectDraft(t, "user", testUserProject, uniqueTitle("DoneSmoke_User"))
	markProjectItemDone(t, "user", testUserProject, id)
}
