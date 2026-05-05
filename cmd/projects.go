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

	"github.com/ozzy-labs/gh-tasks/internal/github"
	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
	"github.com/ozzy-labs/gh-tasks/internal/projectitem"
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
			r, err := deps.Resolve(c)
			if err != nil {
				return localizedError(c, r, err)
			}
			fmt.Fprintln(c.ErrOrStderr(), r.T("error.projects.subcommandRequired"))
			return ErrSilentArgs
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
			r, err := deps.Resolve(c)
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
	r, err := deps.Resolve(c)
	if err != nil {
		return localizedError(c, r, err)
	}
	title, _ := c.Flags().GetString("title")
	if title == "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.projectsInit.titleRequired"))
		return ErrSilentArgs
	}
	tpl, _ := c.Flags().GetString("template")
	owner, _ := c.Flags().GetString("owner")
	dryRun, _ := c.Flags().GetBool("dry-run")
	if yamlPath == "" && tpl == "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.projectsInit.templateRequired"))
		return ErrSilentArgs
	}
	if yamlPath != "" && tpl != "" {
		fmt.Fprintln(c.ErrOrStderr(), r.T("error.projectsInit.templateConflict"))
		return ErrSilentArgs
	}
	raw, source, err := loadTemplateRaw(yamlPath, tpl)
	if err != nil {
		fmt.Fprintln(c.ErrOrStderr(),
			r.T("error.projectsInit.yamlRead", "path", source, "reason", err.Error()))
		return ErrSilentArgs
	}
	parsed, err := parseTemplateBytes(raw)
	if err != nil {
		fmt.Fprintln(c.ErrOrStderr(),
			r.T("error.projectsInit.yamlRead", "path", source, "reason", err.Error()))
		return ErrSilentArgs
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
			return ErrSilentArgs
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
		return ErrSilentRuntime
	}

	gqlClient := clients.AsGenqlientClient()
	pResp, err := queries.CreateProjectV2(ctx, gqlClient, &queries.CreateProjectV2Input{
		OwnerId: ownerID,
		Title:   title,
	})
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	project := pResp.CreateProjectV2.ProjectV2
	fmt.Fprintln(c.OutOrStdout(),
		r.T("projectsInit.created", "url", project.Url))

	existing, err := queries.ListProjectV2Fields(ctx, gqlClient, project.Id, 100)
	if err != nil {
		return fmt.Errorf("list project fields: %w", err)
	}
	existingNames := map[string]bool{}
	for _, f := range projectitem.FieldsOf(projectitem.FieldsFromResponse(existing)) {
		existingNames[strings.ToLower(f.Name)] = true
	}

	for _, f := range fields {
		if existingNames[strings.ToLower(f.Name)] {
			fmt.Fprintln(c.OutOrStdout(),
				r.T("projectsInit.fieldSkipped", "name", f.Name))
			continue
		}
		input := &queries.CreateProjectV2FieldInput{
			ProjectId: project.Id,
			Name:      f.Name,
			DataType:  queries.ProjectV2CustomFieldType(f.DataType),
		}
		if len(f.SingleSelectOptions) > 0 {
			input.SingleSelectOptions = make([]*queries.ProjectV2SingleSelectFieldOptionInput, len(f.SingleSelectOptions))
			for i, opt := range f.SingleSelectOptions {
				input.SingleSelectOptions[i] = &queries.ProjectV2SingleSelectFieldOptionInput{
					Name:        opt["name"],
					Color:       queries.ProjectV2SingleSelectFieldOptionColor(opt["color"]),
					Description: opt["description"],
				}
			}
		}
		created, err := queries.CreateProjectV2Field(ctx, gqlClient, input)
		if err != nil {
			return fmt.Errorf("create project field: %w", err)
		}
		name, dataType := projectV2FieldDescriptor(created.CreateProjectV2Field.ProjectV2Field)
		fmt.Fprintln(c.OutOrStdout(),
			r.T("projectsInit.fieldCreated",
				"name", name,
				"dataType", dataType))
	}
	return nil
}

// projectV2FieldDescriptor extracts the (name, dataType) of a newly-created
// Projects v2 field from the genqlient-generated `ProjectV2FieldConfiguration`
// interface return shape. The selection set in `operations.graphql` only
// requests `... on ProjectV2FieldCommon { id name dataType }`, but
// genqlient still synthesises one wrapper struct per concrete subtype
// (`ProjectV2Field`, `ProjectV2IterationField`, `ProjectV2SingleSelectField`).
// All three carry the same `Name` / `DataType` shape, so a simple type
// switch surfaces the values.
func projectV2FieldDescriptor(v *queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2FieldProjectV2FieldConfiguration) (string, string) {
	if v == nil {
		return "", ""
	}
	switch f := (*v).(type) {
	case *queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2Field:
		return f.Name, string(f.DataType)
	case *queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2FieldProjectV2IterationField:
		return f.Name, string(f.DataType)
	case *queries.CreateProjectV2FieldCreateProjectV2FieldCreateProjectV2FieldPayloadProjectV2FieldProjectV2SingleSelectField:
		return f.Name, string(f.DataType)
	}
	return "", ""
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

func resolveOwnerID(ctx context.Context, gql github.GraphQLClient, owner string) (string, error) {
	gqlClient := github.AsGenqlientClientFor(gql)
	if owner == "@me" {
		resp, err := queries.GetViewerID(ctx, gqlClient)
		if err != nil {
			return "", err
		}
		return resp.Viewer.Id, nil
	}
	resp, err := queries.GetOwnerID(ctx, gqlClient, owner)
	if err != nil {
		return "", err
	}
	if resp.RepositoryOwner == nil || *resp.RepositoryOwner == nil {
		return "", nil
	}
	return (*resp.RepositoryOwner).GetId(), nil
}
