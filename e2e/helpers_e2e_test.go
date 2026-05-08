//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test fixtures: the canonical Projects and repo that E2E writes to.
// Changing these requires updating 7 files — see memory rule
// `feedback_e2e_project_id_change_rule`.
const (
	testOrgProject  = "ozzy-labs/3"
	testUserProject = "ozzy-3/5"
	testRepo        = "ozzy-labs/gh-tasks"
)

// sharedBin is the path of the gh-tasks binary built once per `go test` run
// in TestMain. Smoke tests reuse it instead of rebuilding for every case.
var sharedBin string

// repoRoot is the absolute path of the repo's git root, resolved once in
// TestMain. Tests use it to invoke the binary with a known cwd when a flow
// depends on git remote auto-detection.
var repoRoot string

func TestMain(m *testing.M) { os.Exit(setupAndRun(m)) }

// setupAndRun resolves repoRoot, builds the binary into a tempdir, and runs
// the suite. Wrapping in a non-Exit-ing function lets the deferred tempdir
// cleanup actually execute (os.Exit skips defers).
func setupAndRun(m *testing.M) int {
	rr, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "E2E setup: git rev-parse failed (run from inside the repo): %v\n", err)
		return 1
	}
	repoRoot = strings.TrimSpace(string(rr))

	dir, err := os.MkdirTemp("", "gh-tasks-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "E2E setup: mkdtemp: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()

	sharedBin = filepath.Join(dir, "gh-tasks")
	build := exec.Command("go", "build", "-o", sharedBin, ".")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "E2E setup: go build failed: %v\n%s\n", err, out)
		return 1
	}

	return m.Run()
}

// uniqueTitle generates a `[E2E] <tag> <unix-nano>` title so test
// resources are easy to spot in real Project listings and don't collide
// across parallel test runs.
func uniqueTitle(tag string) string {
	return fmt.Sprintf("[E2E] %s %d", tag, time.Now().UnixNano())
}

// addProjectDraft adds a draft item to the given project and returns
// the parsed project item ID. Forces --lang en for stable parsing.
func addProjectDraft(t *testing.T, scope, project, title string) string {
	t.Helper()
	stdout, stderr, code := runBin(t,
		"--lang", "en",
		"-s", scope, "-p", project,
		"add", title,
	)
	if code != 0 {
		t.Fatalf("addProjectDraft: stderr=%q stdout=%q", stderr, stdout)
	}
	id := extractTrailingID(t, stdout)
	if id == "" {
		t.Fatalf("addProjectDraft: empty id, stdout=%q", stdout)
	}
	return id
}

// markProjectItemDone runs `gh tasks done` with retry to absorb the
// eventual-consistency window between draft creation and the item
// becoming searchable in `node.items`. Fails the calling test if all
// attempts fail. Use `cleanupProjectItem` instead from inside
// t.Cleanup if a cleanup miss should not fail the surrounding test.
func markProjectItemDone(t *testing.T, scope, project, itemID string) {
	t.Helper()
	const maxAttempts = 6
	var lastStderr string
	for attempt := 0; attempt < maxAttempts; attempt++ {
		_, stderr, code := runBin(t,
			"--lang", "en",
			"-s", scope, "-p", project,
			"done", itemID,
		)
		if code == 0 {
			return
		}
		lastStderr = stderr
		if !strings.Contains(stderr, "Item not found in project") {
			t.Fatalf("done(%s, %s, %s): stderr=%q", scope, project, itemID, stderr)
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("done(%s, %s, %s): exhausted %d attempts: stderr=%q",
		scope, project, itemID, maxAttempts, lastStderr)
}

// cleanupProjectItem is the t.Cleanup-friendly variant of
// markProjectItemDone: it logs failures via t.Logf rather than failing
// the test. Use this when leaving an item in Status=Todo on a cleanup
// miss is preferable to failing an otherwise-passing test.
func cleanupProjectItem(t *testing.T, scope, project, itemID string) {
	t.Helper()
	const maxAttempts = 6
	for attempt := 0; attempt < maxAttempts; attempt++ {
		_, stderr, code := runBin(t,
			"--lang", "en",
			"-s", scope, "-p", project,
			"done", itemID,
		)
		if code == 0 {
			return
		}
		if !strings.Contains(stderr, "Item not found in project") {
			t.Logf("cleanupProjectItem(%s, %s, %s): stderr=%q", scope, project, itemID, stderr)
			return
		}
		time.Sleep(time.Second)
	}
	t.Logf("cleanupProjectItem(%s, %s, %s): exhausted retries (eventual consistency)",
		scope, project, itemID)
}

// addRepoIssue creates an Issue in the test repo and returns the issue
// number parsed from the URL printed on stdout. Forces --lang en.
func addRepoIssue(t *testing.T, repo, title string) int {
	t.Helper()
	stdout, stderr, code := runBin(t,
		"--lang", "en",
		"-s", "repo", "-r", repo,
		"add", title,
	)
	if code != 0 {
		t.Fatalf("addRepoIssue: stderr=%q stdout=%q", stderr, stdout)
	}
	url := strings.TrimSpace(extractTrailingID(t, stdout))
	if url == "" {
		t.Fatalf("addRepoIssue: empty url, stdout=%q", stdout)
	}
	num := 0
	if i := strings.LastIndex(url, "/"); i >= 0 {
		_, err := fmt.Sscanf(url[i+1:], "%d", &num)
		if err != nil || num == 0 {
			t.Fatalf("addRepoIssue: cannot parse issue number from %q", url)
		}
	}
	return num
}

// closeRepoIssue uses `gh tasks done` to close a repo-scope Issue.
// Cleanup-style helper: errors are logged, not raised.
func closeRepoIssue(t *testing.T, repo string, num int) {
	t.Helper()
	_, stderr, code := runBin(t,
		"--lang", "en",
		"-s", "repo", "-r", repo,
		"done", fmt.Sprintf("%d", num),
	)
	if code != 0 {
		t.Logf("closeRepoIssue(%s, #%d): stderr=%q", repo, num, stderr)
	}
}

// extractTrailingID parses an ID off the end of an i18n message line of
// the form `<localized prefix>: <id>`. Returns "" if no `: ` separator
// is found.
func extractTrailingID(t *testing.T, stdout string) string {
	t.Helper()
	line := strings.TrimSpace(stdout)
	idx := strings.LastIndex(line, ": ")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(line[idx+2:])
}

// runBin executes the shared binary with the given args, returning stdout,
// stderr, and the exit code. Non-ExitError failures (e.g. binary missing)
// fail the test directly.
func runBin(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	var so, se strings.Builder
	cmd := exec.Command(sharedBin, args...)
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("exec %s %v: %v", sharedBin, args, err)
		}
		code = exitErr.ExitCode()
	}
	return so.String(), se.String(), code
}
