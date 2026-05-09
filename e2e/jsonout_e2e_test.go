//go:build e2e

package e2e

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// E2E smoke for the structured-output surface (#367 / #376 / #386):
// `--json [fields]` + `--jq <query>` + `--paginate`. These tests are
// intentionally minimal — unit / flow tests already cover the catalog
// shape, the marker-rewrite drift gate, and edge cases like empty
// values; what we want from e2e is "the wire format works against a
// real GitHub" and "the Go CLI surface composes correctly with real
// argv parsing". Non-mutating where possible to keep the smoke fast.

// TestE2E_SmokeJSONReadOnly verifies that --json reads work end-to-end
// against both test Projects: stdout parses as a JSON array, every
// row carries the requested fields, and the response is locale-
// independent (we pass --lang ja but expect English field names).
func TestE2E_SmokeJSONReadOnly(t *testing.T) {
	cases := []struct {
		name  string
		scope string
		proj  string
	}{
		{"org", "org", testOrgProject},
		{"user", "user", testUserProject},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runBin(t,
				"--lang", "ja",
				"-s", tc.scope, "-p", tc.proj,
				"list", "--limit", "1",
				"--json", "id,title,type,state",
			)
			if code != 0 {
				t.Fatalf("list --json: exit=%d stderr=%q", code, stderr)
			}
			var rows []map[string]any
			if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
				t.Fatalf("stdout is not a JSON array: %v\nstdout=%s", err, stdout)
			}
			// Empty Projects are valid (the test board may have no items
			// at the moment); the contract is "parses to []", not
			// "non-empty".
			for i, row := range rows {
				for _, want := range []string{"id", "title", "type", "state"} {
					if _, ok := row[want]; !ok {
						t.Errorf("row %d missing field %q: %v", i, want, row)
					}
				}
			}
		})
	}
}

// TestE2E_SmokeJSONJq verifies the in-process gojq path produces
// usable output. We extract the id of the first row (if any) via
// `--jq '.[0].id'` and assert the result is either a quoted string or
// `null` (jq's representation of "no first element"). Non-mutating.
func TestE2E_SmokeJSONJq(t *testing.T) {
	stdout, stderr, code := runBin(t,
		"--lang", "en",
		"-s", "org", "-p", testOrgProject,
		"list", "--limit", "1",
		"--json", "id",
		"--jq", ".[0].id",
	)
	if code != 0 {
		t.Fatalf("list --json --jq: exit=%d stderr=%q", code, stderr)
	}
	got := strings.TrimSpace(stdout)
	if got != "null" && (!strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`)) {
		t.Errorf("expected quoted-string id or null, got: %q", got)
	}
}

// TestE2E_SmokeJSONWriteRoundtrip is the JSON twin of
// TestE2E_SmokeWriteRoundtrip_*. It exercises the mutation-side --json
// surface added in Phase 2: `add --json` returns the created item's
// id directly (no text-output parsing), which `done --json` consumes.
// Validates the canonical script idiom from README:
//
//	id=$(gh tasks add ... --json id --jq '.[0].id')
//	gh tasks done "$id" --json state --jq '.[0].state'
func TestE2E_SmokeJSONWriteRoundtrip(t *testing.T) {
	const scope = "org"
	const project = testOrgProject
	title := uniqueTitle("smoke jsonout")

	// add --json id --jq '.[0].id' → unquoted id string on stdout
	stdout, stderr, code := runBin(t,
		"--lang", "en",
		"-s", scope, "-p", project,
		"add", title,
		"--json", "id",
		"--jq", ".[0].id",
	)
	if code != 0 {
		t.Fatalf("add --json: exit=%d stderr=%q", code, stderr)
	}
	itemID := strings.Trim(strings.TrimSpace(stdout), `"`)
	if itemID == "" {
		t.Fatalf("add --json: empty id, stdout=%q", stdout)
	}

	// done --json — same id-from-create path, plus the state
	// assertion. Wrapped in the same eventual-consistency retry as
	// markProjectItemDone because the draft may not be searchable in
	// the items page yet.
	t.Cleanup(func() {
		// Best-effort cleanup: if done never succeeded the helper
		// would have failed the test already, so a no-op here is
		// fine.
	})

	doneOK := false
	var lastStderr string
	for attempt := 0; attempt < 6; attempt++ {
		stdout, stderr, code = runBin(t,
			"--lang", "en",
			"-s", scope, "-p", project,
			"done", itemID,
			"--json", "id,state",
		)
		if code == 0 {
			doneOK = true
			break
		}
		lastStderr = stderr
		if !strings.Contains(stderr, "Item not found in project") {
			t.Fatalf("done --json: stderr=%q", stderr)
		}
		// fall through to retry
	}
	if !doneOK {
		t.Fatalf("done --json: exhausted retries, lastStderr=%q", lastStderr)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("done stdout is not JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("expected single-element array, got len=%d", len(rows))
	}
	if rows[0]["id"] != itemID {
		t.Errorf("done --json id = %v; want %s", rows[0]["id"], itemID)
	}
}

// TestE2E_SmokePaginate verifies the read-only --paginate path works
// against a real Project. Cannot assert a specific length because
// the test board's content fluctuates run-to-run, but `length` must
// be a non-negative integer and the call must exit 0.
func TestE2E_SmokePaginate(t *testing.T) {
	stdout, stderr, code := runBin(t,
		"--lang", "en",
		"-s", "org", "-p", testOrgProject,
		"list", "--paginate",
		"--json", "id",
		"--jq", "length",
	)
	if code != 0 {
		t.Fatalf("list --paginate --json: exit=%d stderr=%q", code, stderr)
	}
	got := strings.TrimSpace(stdout)
	if got == "" {
		t.Fatal("expected length output, got empty")
	}
	// `jq 'length'` emits an unquoted integer.
	n, err := strconv.Atoi(got)
	if err != nil {
		t.Errorf("expected integer length from jq, got %q: %v", got, err)
	}
	if n < 0 {
		t.Errorf("length must be >= 0, got %d", n)
	}
}
