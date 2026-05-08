//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestE2E_ProjectsInitTemplatesSmoke verifies the static
// `projects init-templates` command outputs the bundled YAML. No
// network, no scope dependency — included here so the projects sub-cmd
// has a place in the isolation matrix.
func TestE2E_ProjectsInitTemplatesSmoke(t *testing.T) {
	stdout, stderr, code := runBin(t, "projects", "init-templates")
	if code != 0 {
		t.Fatalf("projects init-templates: stderr=%q", stderr)
	}
	// Templates declare iteration field configuration — assert the
	// substring rather than full YAML to keep the test loose against
	// future template tweaks.
	if !strings.Contains(stdout, "iteration") && !strings.Contains(stdout, "Iteration") {
		t.Errorf("projects init-templates output missing iteration field: %q", stdout)
	}
}

// TestE2E_ProjectsInitSmoke_Org / _User verify `projects init` with
// --dry-run. The --dry-run path resolves the template, validates the
// auth, and prints the planned mutations without actually creating a
// Project. Live Project creation is deferred to Flow G in lifecycle
// tests because it requires post-test archival to keep the org/user
// project list clean.

func TestE2E_ProjectsInitSmoke_Org(t *testing.T) {
	_, stderr, code := runBin(t,
		"--lang", "en",
		"projects", "init",
		"--owner", "ozzy-labs",
		"--template", "org",
		"--title", uniqueTitle("ProjectsInitSmoke_Org"),
		"--dry-run",
	)
	if code != 0 {
		t.Fatalf("projects init --dry-run org: stderr=%q", stderr)
	}
}

func TestE2E_ProjectsInitSmoke_User(t *testing.T) {
	_, stderr, code := runBin(t,
		"--lang", "en",
		"projects", "init",
		"--owner", "@me",
		"--template", "user",
		"--title", uniqueTitle("ProjectsInitSmoke_User"),
		"--dry-run",
	)
	if code != 0 {
		t.Fatalf("projects init --dry-run user: stderr=%q", stderr)
	}
}
