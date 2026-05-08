//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestE2E_PlanSmoke_* runs `gh tasks plan` with --dry-run in each scope.
// --dry-run is mandatory for the smoke because the unrestricted form
// creates Milestones (repo) or rebinds items into Iterations
// (org / user), neither of which we want as a side effect of a smoke
// test. The dry-run path still exercises the full pre-flight: scope
// detection, period parsing, GraphQL paginate of items, and the
// localized output of the plan summary.

func TestE2E_PlanSmoke_Repo(t *testing.T) {
	stdout, stderr, code := runBin(t,
		"--lang", "en",
		"-s", "repo", "-r", testRepo,
		"plan", "--period", "weekly", "--dry-run",
	)
	if code != 0 {
		t.Fatalf("plan -s repo --dry-run: stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "Proposed") {
		t.Errorf("plan stdout missing 'Proposed' header: %q", stdout)
	}
}

func TestE2E_PlanSmoke_Org(t *testing.T) {
	stdout, stderr, code := runBin(t,
		"--lang", "en",
		"-s", "org", "-p", testOrgProject,
		"plan", "--period", "sprint", "--dry-run",
	)
	if code != 0 {
		t.Fatalf("plan -s org --period sprint --dry-run: stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "Proposed") {
		t.Errorf("plan stdout missing 'Proposed' header: %q", stdout)
	}
}

func TestE2E_PlanSmoke_User(t *testing.T) {
	stdout, stderr, code := runBin(t,
		"--lang", "en",
		"-s", "user", "-p", testUserProject,
		"plan", "--period", "sprint", "--dry-run",
	)
	if code != 0 {
		t.Fatalf("plan -s user --period sprint --dry-run: stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "Proposed") {
		t.Errorf("plan stdout missing 'Proposed' header: %q", stdout)
	}
}
