package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
)

// Inlined copies of packages/templates/projects-v2/{user,org}.yaml. They are
// embedded as string constants so the single-binary build stays self-contained
// even though the YAML files remain the documentation SSOT.

const bundledUserTemplate = `# GitHub Projects v2 field template - user (personal) scope
#
# Used by ` + "`gh-tasks`" + ` user scope (personal Project v2). Defines the minimum
# fields required for ` + "`gh tasks plan`" + ` (Iteration) and standup/review
# (Status) to operate.
#
# Iteration field duration / start date are intentionally omitted: the
# GitHub UI is the authoritative configuration surface for those (the
# REST/GraphQL API only accepts the iteration list, not the cadence).
# Configure them under Project settings -> Iteration after creation.
name: gh-tasks user scope
description: Personal Project v2 fields for gh-tasks (user scope)
fields:
  - name: Status
    type: single_select
    options:
      - Triage
      - Todo
      - In Progress
      - Done
  - name: Iteration
    type: iteration
`

const bundledOrgTemplate = `# GitHub Projects v2 field template - org (team) scope
#
# Used by ` + "`gh-tasks`" + ` org scope (organization-wide Project v2).
# Extends the user template with cross-repo coordination fields:
#
# - Repository: built-in field type for the owning repo of an item.
# - Project: free-form single_select identifying a logical project that
#   spans multiple repos (e.g. "Platform", "Docs", "Infra"). Edit the
#   options list to match your org's project taxonomy.
#
# Iteration cadence is configured in the GitHub UI (see user.yaml note).
name: gh-tasks org scope
description: Team Project v2 fields for gh-tasks (org scope)
fields:
  - name: Status
    type: single_select
    options:
      - Triage
      - Todo
      - In Progress
      - Done
  - name: Iteration
    type: iteration
  - name: Repository
    type: repository
  - name: Project
    type: single_select
    options:
      - Platform
      - Docs
      - Infra
`

func newProjectsCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:          "projects",
		Short:        "Manage Projects v2",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, _ []string) error {
			r, err := deps.Resolve()
			if err != nil {
				return localizedError(c, r, err)
			}
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.projects.subcommandRequired"))
			return ErrSilent
		},
	}
	c.AddCommand(newProjectsInitCmd(deps), newProjectsInitTemplatesCmd(deps))
	return c
}

func newProjectsInitCmd(deps Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "init [yaml-path]",
		Short: "Initialize a Projects v2 board with field templates",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			yamlPath := ""
			if len(args) > 0 {
				yamlPath = args[0]
			}
			return runProjectsInit(c.Context(), c, deps, yamlPath)
		},
	}
	c.Flags().String("template", "", "bundled template name: user | org")
	c.Flags().String("owner", "@me", "owner login (or @me for the viewer)")
	c.Flags().String("title", "", "Project v2 title (required)")
	c.Flags().Bool("dry-run", false, "print the planned creation without mutating GitHub")
	return c
}

func newProjectsInitTemplatesCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "init-templates",
		Short: "Print the bundled `user` / `org` Project v2 field templates",
		RunE: func(c *cobra.Command, _ []string) error {
			r, err := deps.Resolve()
			if err != nil {
				return localizedError(c, r, err)
			}
			out := c.OutOrStdout()
			fmt.Fprintln(out, "# --template user")
			fmt.Fprint(out, bundledUserTemplate)
			fmt.Fprintln(out)
			fmt.Fprintln(out, "# --template org")
			fmt.Fprint(out, bundledOrgTemplate)
			return nil
		},
	}
}

type templateField struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Options []string `yaml:"options,omitempty"`
}

type parsedTemplate struct {
	Fields []templateField `yaml:"fields"`
}

type fieldInput struct {
	Name                string
	DataType            string
	SingleSelectOptions []map[string]string
}

func runProjectsInit(ctx context.Context, c *cobra.Command, deps Deps, yamlPath string) error {
	r, err := deps.Resolve()
	if err != nil {
		return localizedError(c, r, err)
	}
	title, _ := c.Flags().GetString("title")
	if title == "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.projectsInit.titleRequired"))
		return ErrSilent
	}
	tpl, _ := c.Flags().GetString("template")
	owner, _ := c.Flags().GetString("owner")
	dryRun, _ := c.Flags().GetBool("dry-run")
	if yamlPath == "" && tpl == "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.projectsInit.templateRequired"))
		return ErrSilent
	}
	if yamlPath != "" && tpl != "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.projectsInit.templateConflict"))
		return ErrSilent
	}
	raw, source, err := loadTemplateRaw(yamlPath, tpl)
	if err != nil {
		fmt.Fprintln(c.ErrOrStderr(),
			r.T("error.projectsInit.yamlRead", "path", source, "reason", err.Error()))
		return ErrSilent
	}
	parsed, err := parseTemplateBytes(raw)
	if err != nil {
		fmt.Fprintln(c.ErrOrStderr(),
			r.T("error.projectsInit.yamlRead", "path", source, "reason", err.Error()))
		return ErrSilent
	}
	fields := []fieldInput{}
	for _, f := range parsed.Fields {
		if f.Type == "repository" {
			continue
		}
		input, err := toFieldInput(f)
		if err != nil {
			fmt.Fprintln(c.ErrOrStderr(),
				r.T("error.projectsInit.yamlRead", "path", source, "reason", err.Error()))
			return ErrSilent
		}
		fields = append(fields, input)
	}

	if dryRun {
		fmt.Fprintln(c.OutOrStdout(),
			r.T("projectsInit.dryRunHeader", "title", title, "owner", owner))
		for _, f := range fields {
			suffix := ""
			if len(f.SingleSelectOptions) > 0 {
				names := make([]string, len(f.SingleSelectOptions))
				for i, o := range f.SingleSelectOptions {
					names[i] = o["name"]
				}
				suffix = " [" + strings.Join(names, ", ") + "]"
			}
			fmt.Fprintf(c.OutOrStdout(), "  - %s (%s)%s\n", f.Name, f.DataType, suffix)
		}
		return nil
	}

	clients, err := deps.NewClients()
	if err != nil {
		return localizedError(c, r, err)
	}
	ownerID, err := resolveOwnerID(ctx, clients.GraphQL, owner)
	if err != nil {
		return err
	}
	if ownerID == "" {
		fmt.Fprintln(c.ErrOrStderr(),
			r.T("error.projectsInit.ownerNotFound", "owner", owner))
		return ErrSilent
	}

	var pResp queries.CreateProjectV2Response
	if err := clients.GraphQL.Do(ctx, queries.CreateProjectV2, map[string]any{
		"input": map[string]any{"ownerId": ownerID, "title": title},
	}, &pResp); err != nil {
		return err
	}
	project := pResp.CreateProjectV2.ProjectV2
	fmt.Fprintln(c.OutOrStdout(),
		r.T("projectsInit.created", "url", project.URL))

	var existing queries.ListProjectV2FieldsResponse
	if err := clients.GraphQL.Do(ctx, queries.ListProjectV2Fields, map[string]any{
		"projectId": project.ID, "first": 100,
	}, &existing); err != nil {
		return err
	}
	existingNames := map[string]bool{}
	if existing.Node != nil {
		for _, f := range existing.Node.Fields.Nodes {
			existingNames[strings.ToLower(f.Name)] = true
		}
	}

	for _, f := range fields {
		if existingNames[strings.ToLower(f.Name)] {
			fmt.Fprintln(c.OutOrStdout(),
				r.T("projectsInit.fieldSkipped", "name", f.Name))
			continue
		}
		input := map[string]any{
			"projectId": project.ID,
			"name":      f.Name,
			"dataType":  f.DataType,
		}
		if len(f.SingleSelectOptions) > 0 {
			input["singleSelectOptions"] = f.SingleSelectOptions
		}
		var created queries.CreateProjectV2FieldResponse
		if err := clients.GraphQL.Do(ctx, queries.CreateProjectV2Field, map[string]any{
			"input": input,
		}, &created); err != nil {
			return err
		}
		fmt.Fprintln(c.OutOrStdout(),
			r.T("projectsInit.fieldCreated",
				"name", created.CreateProjectV2Field.ProjectV2Field.Name,
				"dataType", created.CreateProjectV2Field.ProjectV2Field.DataType))
	}
	return nil
}

func loadTemplateRaw(yamlPath, tpl string) ([]byte, string, error) {
	switch tpl {
	case "user":
		return []byte(bundledUserTemplate), "--template user", nil
	case "org":
		return []byte(bundledOrgTemplate), "--template org", nil
	}
	if yamlPath == "" {
		return nil, "", errors.New("no source")
	}
	// Defense-in-depth: clean the path before opening it so that any
	// traversal sequences are normalized away. The CLI is operator-trusted
	// (the user supplies the path interactively), so the residual gosec
	// G304 finding is suppressed below.
	yamlPath = filepath.Clean(yamlPath)
	raw, err := os.ReadFile(yamlPath) //nolint:gosec // operator-trusted CLI; user supplies the path interactively
	if err != nil {
		return nil, yamlPath, err
	}
	return raw, yamlPath, nil
}

func parseTemplateBytes(raw []byte) (parsedTemplate, error) {
	var doc parsedTemplate
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return parsedTemplate{}, err
	}
	for _, f := range doc.Fields {
		if f.Name == "" || f.Type == "" {
			return parsedTemplate{}, errors.New("each field must have name and type")
		}
		switch f.Type {
		case "text", "number", "date", "single_select", "iteration", "repository":
			// ok
		default:
			return parsedTemplate{}, fmt.Errorf("unsupported field type: %q (field %q)", f.Type, f.Name)
		}
		if f.Type == "single_select" && len(f.Options) == 0 {
			return parsedTemplate{}, fmt.Errorf("single_select field %q requires options[]", f.Name)
		}
	}
	return doc, nil
}

func toFieldInput(f templateField) (fieldInput, error) {
	dataType, err := templateTypeToDataType(f.Type)
	if err != nil {
		return fieldInput{}, err
	}
	if dataType == "SINGLE_SELECT" {
		opts := make([]map[string]string, len(f.Options))
		for i, name := range f.Options {
			opts[i] = map[string]string{
				"name":        name,
				"color":       "GRAY",
				"description": "",
			}
		}
		return fieldInput{Name: f.Name, DataType: dataType, SingleSelectOptions: opts}, nil
	}
	return fieldInput{Name: f.Name, DataType: dataType}, nil
}

func templateTypeToDataType(t string) (string, error) {
	switch t {
	case "text":
		return "TEXT", nil
	case "number":
		return "NUMBER", nil
	case "date":
		return "DATE", nil
	case "single_select":
		return "SINGLE_SELECT", nil
	case "iteration":
		return "ITERATION", nil
	case "repository":
		return "", errors.New("unreachable: repository is built-in")
	}
	return "", fmt.Errorf("unsupported field type: %q", t)
}

func resolveOwnerID(ctx context.Context, gql interface {
	Do(context.Context, string, map[string]any, any) error
}, owner string,
) (string, error) {
	if owner == "@me" {
		var resp queries.GetViewerIDResponse
		if err := gql.Do(ctx, queries.GetViewerID, nil, &resp); err != nil {
			return "", err
		}
		return resp.Viewer.ID, nil
	}
	var resp queries.GetOwnerIDResponse
	if err := gql.Do(ctx, queries.GetOwnerID, map[string]any{"login": owner}, &resp); err != nil {
		return "", err
	}
	if resp.RepositoryOwner == nil {
		return "", nil
	}
	return resp.RepositoryOwner.ID, nil
}
