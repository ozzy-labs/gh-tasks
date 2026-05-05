// Cobra-rooted flow tests for the `projects` command group: `projects init`
// (template-driven board bootstrap), the `projects` subcommand wiring, and
// `projects init-templates` (bundled-YAML printer). Shared helpers live in
// `testhelpers_test.go`. See `docs/design/test-structure.md` for rationale
// and the `Test<Cmd>_<Scenario>` naming convention.
package cmd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ozzy-labs/gh-tasks/cmd"
	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/testfake"
)

// ===== projects init =======================================================

func TestProjectsInit_DryRunHeader(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "My Todo", "--dry-run")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "(--dry-run)") || !strings.Contains(got, "My Todo") {
		t.Errorf("expected dry-run header with title, got:\n%s", got)
	}
	// User template fields: Status (single_select with Triage/Todo/Done) +
	// Iteration. Verify both surface.
	if !strings.Contains(got, "Status") || !strings.Contains(got, "Iteration") {
		t.Errorf("expected user-template fields in output, got:\n%s", got)
	}
}

func TestProjectsInit_DryRunOrgTemplate(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "projects", "init", "--template", "org", "--title", "Team", "--dry-run")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Status", "Iteration", "Project"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected org-template field %q in output, got:\n%s", want, got)
		}
	}
}

func TestProjectsInit_TitleRequired(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--template", "user")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projectsInit.titleRequired")
}

func TestProjectsInit_TemplateRequired(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--title", "Foo")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projectsInit.templateRequired")
}

func TestProjectsInit_TemplateConflict(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "x", "/tmp/some.yaml")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projectsInit.templateConflict")
}

func TestProjectsInit_OwnerNotFound(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetOwnerID (", Data: map[string]any{"repositoryOwner": nil}},
	}}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "x", "--owner", "ghost")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projectsInit.ownerNotFound", "owner", "ghost")
}

// TestProjectsInit_CreatesNewFields drives the mutation-success half of
// runProjectsInit end-to-end with a captured GraphQL fake, asserting:
//   - GetViewerID is consulted for --owner @me
//   - CreateProjectV2 receives the title + viewer-resolved owner id
//   - CreateProjectV2Field is invoked once per template entry
//   - The 3 genqlient subtype payloads (Common / SingleSelect / Iteration)
//     all flow through projectV2FieldDescriptor without losing name/dataType
//
// The user-template ships 2 fields (Status[single_select], Iteration); we
// canon a CreateProjectV2Field response per call to stage one Common, one
// SingleSelectField, and (via the second mutation) one IterationField --
// that combination gives projectV2FieldDescriptor full type-switch coverage.
func TestProjectsInit_CreatesNewFields(t *testing.T) {
	t.Parallel()

	mutationInputs := []map[string]any{}
	inner := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerID", Data: map[string]any{"viewer": map[string]any{
			"id": "U_viewer", "login": "me",
		}}},
		{MatchSubstring: "mutation CreateProjectV2 (", Data: map[string]any{
			"createProjectV2": map[string]any{"projectV2": map[string]any{
				"id": "PVT_new", "number": 1, "title": "My Todo",
				"url": "https://example.test/p/1",
			}},
		}},
		// PaginateProjectV2Fields runs once after the project is created;
		// returning an empty connection ensures every template field hits
		// the create path (no `existingNames` skip).
		{MatchSubstring: "query ListProjectV2Fields (", Data: map[string]any{
			"node": map[string]any{
				"__typename": "ProjectV2",
				"fields": map[string]any{
					"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					"nodes":    []any{},
				},
			},
		}},
		// First field create: Status (SINGLE_SELECT). Stage the response
		// as a SingleSelectField so the type-switch hits that arm.
		{MatchSubstring: "mutation CreateProjectV2Field (", Data: map[string]any{
			"createProjectV2Field": map[string]any{"projectV2Field": map[string]any{
				"__typename": "ProjectV2SingleSelectField",
				"id":         "F_status", "name": "Status", "dataType": "SINGLE_SELECT",
			}},
		}},
		// Second field create: Iteration. Stage as IterationField.
		{MatchSubstring: "mutation CreateProjectV2Field (", Data: map[string]any{
			"createProjectV2Field": map[string]any{"projectV2Field": map[string]any{
				"__typename": "ProjectV2IterationField",
				"id":         "F_iter", "name": "Iteration", "dataType": "ITERATION",
			}},
		}},
	}}
	wrap := &captureGraphQL{inner: inner, capture: func(query string, vars map[string]any) {
		if strings.Contains(query, "mutation CreateProjectV2 (") ||
			strings.Contains(query, "mutation CreateProjectV2Field (") {
			if input, ok := vars["input"].(map[string]any); ok {
				// Defensive copy so subsequent mutations don't mutate
				// the captured snapshot through the shared map.
				snapshot := map[string]any{}
				for k, v := range input {
					snapshot[k] = v
				}
				snapshot["__op__"] = query
				mutationInputs = append(mutationInputs, snapshot)
			}
		}
	}}
	d := testDeps(inner, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "My Todo")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Project created") || !strings.Contains(got, "https://example.test/p/1") {
		t.Errorf("expected projectsInit.created with URL, got:\n%s", got)
	}
	if !strings.Contains(got, "Status") || !strings.Contains(got, "Iteration") {
		t.Errorf("expected each created field to surface in stdout, got:\n%s", got)
	}
	// CreateProjectV2 + 2x CreateProjectV2Field = 3 capture entries.
	if len(mutationInputs) != 3 {
		t.Fatalf("captured %d mutation inputs, want 3: %#v", len(mutationInputs), mutationInputs)
	}
	if !strings.Contains(mutationInputs[0]["__op__"].(string), "mutation CreateProjectV2 (") {
		t.Errorf("first mutation should be CreateProjectV2, got %q", mutationInputs[0]["__op__"])
	}
	if mutationInputs[0]["ownerId"] != "U_viewer" {
		t.Errorf("CreateProjectV2.ownerId = %v, want U_viewer", mutationInputs[0]["ownerId"])
	}
	if mutationInputs[0]["title"] != "My Todo" {
		t.Errorf("CreateProjectV2.title = %v, want My Todo", mutationInputs[0]["title"])
	}
	// Field mutations: first is Status (SINGLE_SELECT, with options).
	if mutationInputs[1]["projectId"] != "PVT_new" || mutationInputs[1]["name"] != "Status" {
		t.Errorf("Status mutation projectId/name mismatch: %#v", mutationInputs[1])
	}
	if mutationInputs[1]["dataType"] != "SINGLE_SELECT" {
		t.Errorf("Status dataType = %v, want SINGLE_SELECT", mutationInputs[1]["dataType"])
	}
	if opts, ok := mutationInputs[1]["singleSelectOptions"].([]any); !ok || len(opts) == 0 {
		t.Errorf("Status mutation should carry singleSelectOptions, got %#v", mutationInputs[1]["singleSelectOptions"])
	}
	// Second: Iteration (no options).
	if mutationInputs[2]["name"] != "Iteration" || mutationInputs[2]["dataType"] != "ITERATION" {
		t.Errorf("Iteration mutation = %#v", mutationInputs[2])
	}
}

// TestProjectsInit_SkipsExistingFields verifies the existingNames branch:
// when PaginateProjectV2Fields surfaces a field whose name (case-insensitive)
// matches one in the template, runProjectsInit emits projectsInit.fieldSkipped
// and does *not* issue CreateProjectV2Field for that entry.
func TestProjectsInit_SkipsExistingFields(t *testing.T) {
	t.Parallel()

	createdFieldNames := []string{}
	inner := &testfake.FakeGraphQL{Responses: []testfake.FakeResponse{
		{MatchSubstring: "query GetViewerID", Data: map[string]any{"viewer": map[string]any{
			"id": "U_viewer", "login": "me",
		}}},
		{MatchSubstring: "mutation CreateProjectV2 (", Data: map[string]any{
			"createProjectV2": map[string]any{"projectV2": map[string]any{
				"id": "PVT_skip", "number": 2, "title": "Reuse",
				"url": "https://example.test/p/2",
			}},
		}},
		// Pre-populate "status" (lowercase) to exercise the case-insensitive
		// skip — the template field is "Status".
		{MatchSubstring: "query ListProjectV2Fields (", Data: map[string]any{
			"node": map[string]any{
				"__typename": "ProjectV2",
				"fields": map[string]any{
					"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					"nodes": []any{
						map[string]any{
							"__typename": "ProjectV2SingleSelectField",
							"id":         "F_existing_status", "name": "status", "dataType": "SINGLE_SELECT",
							"options": []any{},
						},
					},
				},
			},
		}},
		// Only Iteration should hit CreateProjectV2Field after the skip.
		{MatchSubstring: "mutation CreateProjectV2Field (", Data: map[string]any{
			"createProjectV2Field": map[string]any{"projectV2Field": map[string]any{
				"__typename": "ProjectV2IterationField",
				"id":         "F_iter", "name": "Iteration", "dataType": "ITERATION",
			}},
		}},
	}}
	wrap := &captureGraphQL{inner: inner, capture: func(query string, vars map[string]any) {
		if strings.Contains(query, "mutation CreateProjectV2Field (") {
			if input, ok := vars["input"].(map[string]any); ok {
				if name, ok := input["name"].(string); ok {
					createdFieldNames = append(createdFieldNames, name)
				}
			}
		}
	}}
	d := testDeps(inner, func(d *cmd.Deps) {
		d.NewClients = func() (*github.Clients, error) {
			return &github.Clients{Host: "github.com", GraphQL: wrap, REST: fakeREST{}}, nil
		}
	})
	stdout, _, err := runCmd(t, d, "projects", "init", "--template", "user", "--title", "Reuse")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "skipped") || !strings.Contains(got, "Status") {
		t.Errorf("expected projectsInit.fieldSkipped for Status, got:\n%s", got)
	}
	if !strings.Contains(got, "Iteration") {
		t.Errorf("expected Iteration to still be created, got:\n%s", got)
	}
	if len(createdFieldNames) != 1 || createdFieldNames[0] != "Iteration" {
		t.Errorf("expected exactly one CreateProjectV2Field call for Iteration, saw %v", createdFieldNames)
	}
}

// TestProjectsInit_TemplateNotFound exercises the YAML-path-missing branch
// of loadTemplateRaw via the runProjectsInit error funnel: the user passes
// a yaml path that doesn't exist, so the command emits
// `error.projectsInit.yamlRead` and exits with ErrSilent before any
// GraphQL calls are issued.
func TestProjectsInit_TemplateNotFound(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{} // any GraphQL call would fall through with "no fake response matched"
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init", "--title", "x", "/path/does/not/exist.yaml")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	// reason is OS-dependent ("no such file or directory" on linux, etc.)
	// so we can't assert on the full rendered string. Instead we render
	// only the {path} placeholder and assert the prefix-up-to-{reason}
	// portion appears, which still detects key/wording mismatches.
	rendered := i18n.T(i18n.LocaleEN, "error.projectsInit.yamlRead",
		"path", "/path/does/not/exist.yaml")
	prefix := strings.SplitN(rendered, "{reason}", 2)[0]
	if !strings.Contains(stderr.String(), prefix) {
		t.Errorf("expected yamlRead prefix %q in stderr, got:\n%s", prefix, stderr.String())
	}
}

// ===== projects (group) + init-templates wiring ============================
//
// These tests pin the cobra wiring of the `projects` subcommand group and
// the small bundled-template printer. They do NOT exercise GitHub API
// surface — `runProjectsInit` already has full mutation/skip/error coverage
// in the TestProjectsInit_* block above. Their job is to lock the
// command-tree shape (groups, available subcommands, flag rejection) and
// the literal stdout that operators copy-paste into their own YAML files
// when bootstrapping a board.

// TestProjectsCmd_NoArgs pins that running `gh tasks projects` without any
// subcommand emits the i18n `error.projects.subcommandRequired` notice on
// stderr and exits with [cmd.ErrSilent] (via the underlying
// [cmd.ErrSilentArgs]). Stdout must stay empty because the parent agent
// pipes the output of `projects init-templates` into a YAML file — any
// stray group-level chatter would corrupt that pipe.
func TestProjectsCmd_NoArgs(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, stderr, err := runCmd(t, d, "projects")
	if !errors.Is(err, cmd.ErrSilent) {
		t.Fatalf("expected ErrSilent for no-subcommand projects, got %v", err)
	}
	assertI18nMessage(t, stderr.String(), i18n.LocaleEN,
		"error.projects.subcommandRequired")
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout when no subcommand is given, got:\n%s", stdout.String())
	}
}

// TestProjectsCmd_HelpFlag pins that `gh tasks projects --help` produces
// cobra's standard help layout and lists both `init` and `init-templates`
// under "Available Commands:". This guards against accidental subcommand
// removal / rename — the group wiring inside `newProjectsCmd` is exercised
// only when `AddCommand` is consulted by cobra during help rendering.
func TestProjectsCmd_HelpFlag(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "projects", "--help")
	if err != nil {
		t.Fatalf("Execute --help: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Available Commands:", "init", "init-templates"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in projects --help output, got:\n%s", want, got)
		}
	}
}

// TestProjectsInitTemplates_PrintsUserYaml pins that the `# --template
// user` section of `init-templates` carries the bundled user-scope YAML
// (name + fields + Status + Iteration). The literal copy that operators
// pipe into their own YAML files is part of the public CLI contract, so
// the assertion locks the structural markers (top-level keys + scope
// title + the two required fields) rather than the entire string body.
func TestProjectsInitTemplates_PrintsUserYaml(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "projects", "init-templates")
	if err != nil {
		t.Fatalf("Execute init-templates: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"# --template user",
		"name: gh-tasks user scope",
		"fields:",
		"- name: Status",
		"type: single_select",
		"- name: Iteration",
		"type: iteration",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("user template section missing %q, got:\n%s", want, got)
		}
	}
}

// TestProjectsInitTemplates_PrintsOrgYaml pins the `# --template org`
// section, which extends the user template with `Repository` (built-in
// field type) and a free-form `Project` single_select. Operators rely on
// these two extra fields to coordinate cross-repo work in a team Project,
// so any drift in the bundled YAML body is a customer-visible change.
func TestProjectsInitTemplates_PrintsOrgYaml(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	stdout, _, err := runCmd(t, d, "projects", "init-templates")
	if err != nil {
		t.Fatalf("Execute init-templates: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"# --template org",
		"name: gh-tasks org scope",
		"- name: Repository",
		"type: repository",
		"- name: Project",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("org template section missing %q, got:\n%s", want, got)
		}
	}
	// The org section must follow the user section (the printer emits
	// user first, then a blank line, then org). Verify the ordering so a
	// future refactor that swaps them can't slip through.
	userIdx := strings.Index(got, "# --template user")
	orgIdx := strings.Index(got, "# --template org")
	if userIdx < 0 || orgIdx < 0 || userIdx >= orgIdx {
		t.Errorf("expected user section before org section, indices user=%d org=%d:\n%s",
			userIdx, orgIdx, got)
	}
}

// TestProjectsInitTemplates_InvalidTemplate pins that `init-templates`
// rejects unknown flags such as `--template foo`. The command is a small
// stdout printer that takes no flags by design (it emits both bundled
// templates unconditionally), so cobra's unknown-flag error is the
// expected behaviour. This guards against a future refactor that
// silently absorbs / ignores unrecognised flags.
func TestProjectsInitTemplates_InvalidTemplate(t *testing.T) {
	t.Parallel()

	g := &testfake.FakeGraphQL{}
	d := testDeps(g)
	_, stderr, err := runCmd(t, d, "projects", "init-templates", "--template", "foo")
	if err == nil {
		t.Fatalf("expected error for --template flag on init-templates, got nil")
	}
	// cobra surfaces unknown flags via "unknown flag" in the error / stderr.
	combined := err.Error() + "\n" + stderr.String()
	if !strings.Contains(combined, "unknown flag") {
		t.Errorf("expected 'unknown flag' in error/stderr, got err=%v stderr=%s",
			err, stderr.String())
	}
}
