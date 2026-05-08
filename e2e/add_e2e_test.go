//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestE2E_AddSmoke_* exercises `gh tasks add` in isolation. Each test
// creates a single `[E2E]`-prefixed resource and registers cleanup so
// the project / repo doesn't accumulate Todo items if the test passes.
//
// Per the test plan §6 cleanup policy, items are closed (Status=Done /
// Issue closed) rather than physically deleted, so the audit trail
// survives.

func TestE2E_AddSmoke_Repo(t *testing.T) {
	title := uniqueTitle("AddSmoke_Repo")
	num := addRepoIssue(t, testRepo, title)
	t.Cleanup(func() { closeRepoIssue(t, testRepo, num) })
	if num <= 0 {
		t.Fatalf("expected positive issue number, got %d", num)
	}
}

func TestE2E_AddSmoke_Org(t *testing.T) {
	title := uniqueTitle("AddSmoke_Org")
	id := addProjectDraft(t, "org", testOrgProject, title)
	t.Cleanup(func() { cleanupProjectItem(t, "org", testOrgProject, id) })
	if !strings.HasPrefix(id, "PVTI_") {
		t.Errorf("expected PVTI_-prefixed item id, got %q", id)
	}
}

func TestE2E_AddSmoke_User(t *testing.T) {
	title := uniqueTitle("AddSmoke_User")
	id := addProjectDraft(t, "user", testUserProject, title)
	t.Cleanup(func() { cleanupProjectItem(t, "user", testUserProject, id) })
	if !strings.HasPrefix(id, "PVTI_") {
		t.Errorf("expected PVTI_-prefixed item id, got %q", id)
	}
}
