// Package queries holds the GraphQL operations and Go response types used
// by the gh-tasks CLI.
//
// Two flavors coexist: most operations are now genqlient-generated typed
// functions in `genqlient.go` (sourced from `operations.graphql` SSOT +
// `schema.graphql` GitHub public SDL — see `generate.go`). A small set of
// hand-written GraphQL strings and Go response types remain in this file
// for the two read operations whose response shape goes through the
// `node(id: ID!)` Node interface, which genqlient would expand into a Go
// interface backed by every Node-implementing type in the schema:
//
//   - `ListProjectV2Items`
//   - `ListProjectV2Fields`
//
// Migrating these two also requires reshaping the shared
// `ProjectV2ItemNode` / `ProjectV2FieldValue` domain helpers in
// `internal/projectitem` and several cmd/*.go consumers; tracked under
// follow-up #234.
//
// New operations should be added to `operations.graphql` and consumed via
// the genqlient-generated functions.
package queries

// Login is a thin wrapper around { login: "..." } shared by hand-written
// shapes. Each genqlient-generated read operation has its own per-
// operation Login type; this is kept for the still-hand-written
// `ListProjectV2Items` ProjectV2ItemContent shape (deferred under #234).
type Login struct {
	Login string `json:"login"`
}

// Assignees wraps the paginated assignees list returned by hand-written
// queries (currently only `ListProjectV2Items.content.assignees`).
type Assignees struct {
	Nodes []Login `json:"nodes"`
}

// CreateMilestoneResult is the REST response of POST /repos/{o}/{r}/milestones.
// The plan command issues that REST call to create new milestones; the
// node id is then used as input to the genqlient-generated
// [UpdateIssueMilestone] mutation.
type CreateMilestoneResult struct {
	NodeID string `json:"node_id"`
	ID     int    `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// ProjectV2 read operations (deferred under #234) ----------------------------
//
// These two operations remain hand-written for now. Their response shape
// flows through the `node(id: ID!)` Node interface, which genqlient
// would model as a Go interface backed by ~200 generated types — and
// the shared `ProjectV2ItemNode` / `ProjectV2FieldValue` domain helpers
// in `internal/projectitem` plus several cmd/*.go consumers would all
// need to be reshaped. A follow-up issue tracks the migration.

// ListProjectV2Fields lists fields with their per-type extras (single-select
// options, iteration configurations).
const ListProjectV2Fields = `
query ListProjectV2Fields($projectId: ID!, $first: Int!) {
  node(id: $projectId) {
    ... on ProjectV2 {
      fields(first: $first) {
        nodes {
          ... on ProjectV2FieldCommon {
            id
            name
            dataType
          }
          ... on ProjectV2SingleSelectField {
            id
            name
            dataType
            options { id name }
          }
          ... on ProjectV2IterationField {
            id
            name
            dataType
            configuration {
              iterations { id title startDate duration }
              completedIterations { id title startDate duration }
            }
          }
        }
      }
    }
  }
}`

// ProjectV2IterationOption is a single iteration entry in a project's
// iteration field configuration.
type ProjectV2IterationOption struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	StartDate string `json:"startDate"`
	Duration  int    `json:"duration"`
}

// ProjectV2FieldNode represents a Projects v2 field definition. Different
// field types carry different extras; consumers branch on DataType.
//
// Invariants from the GraphQL schema and the inline-fragment selection set in
// [ListProjectV2Fields]:
//   - Options is non-nil only when DataType=SINGLE_SELECT (populated by the
//     ProjectV2SingleSelectField fragment).
//   - Configuration is non-nil only when DataType=ITERATION (populated by the
//     ProjectV2IterationField fragment).
type ProjectV2FieldNode struct {
	ID            string                    `json:"id"`
	Name          string                    `json:"name"`
	DataType      string                    `json:"dataType"`
	Options       []ProjectV2SelectOption   `json:"options,omitempty"`
	Configuration *ProjectV2IterationConfig `json:"configuration,omitempty"`
}

// ProjectV2SelectOption is a single-select field's option.
type ProjectV2SelectOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ProjectV2IterationConfig holds active and completed iterations.
type ProjectV2IterationConfig struct {
	Iterations          []ProjectV2IterationOption `json:"iterations"`
	CompletedIterations []ProjectV2IterationOption `json:"completedIterations"`
}

// ListProjectV2FieldsResponse is the response of [ListProjectV2Fields].
type ListProjectV2FieldsResponse struct {
	Node *struct {
		Fields struct {
			Nodes []ProjectV2FieldNode `json:"nodes"`
		} `json:"fields"`
	} `json:"node"`
}

// ListProjectV2Items lists items in a Projects v2 board.
const ListProjectV2Items = `
query ListProjectV2Items($projectId: ID!, $first: Int!) {
  node(id: $projectId) {
    ... on ProjectV2 {
      items(first: $first) {
        nodes {
          id
          updatedAt
          content {
            __typename
            ... on Issue {
              id
              number
              title
              url
              state
              updatedAt
              closedAt
              author { login }
              assignees(first: 10) { nodes { login } }
            }
            ... on PullRequest {
              id
              number
              title
              url
              state
              updatedAt
              mergedAt
              author { login }
              assignees(first: 10) { nodes { login } }
            }
            ... on DraftIssue {
              id
              title
              body
            }
          }
          fieldValues(first: 30) {
            nodes {
              __typename
              ... on ProjectV2ItemFieldSingleSelectValue {
                optionId
                name
                field { ... on ProjectV2FieldCommon { id name } }
              }
              ... on ProjectV2ItemFieldIterationValue {
                iterationId
                title
                startDate
                duration
                field { ... on ProjectV2FieldCommon { id name } }
              }
              ... on ProjectV2ItemFieldTextValue {
                text
                field { ... on ProjectV2FieldCommon { id name } }
              }
              ... on ProjectV2ItemFieldDateValue {
                date
                field { ... on ProjectV2FieldCommon { id name } }
              }
            }
          }
        }
      }
    }
  }
}`

// ProjectV2ItemContent is the union content returned for each Projects v2
// item. The Typename discriminator selects which fields are populated.
//
// The selection set in [ListProjectV2Items] requests:
//   - Issue        → id, number, title, url, state, updatedAt, closedAt,
//     author, assignees
//   - PullRequest  → id, number, title, url, state, updatedAt, mergedAt,
//     author, assignees
//   - DraftIssue   → id, title, body
//
// Fields shared by Issue and PullRequest (ID, Number, Title, URL, State,
// UpdatedAt, Author, Assignees) are always populated for those two variants
// and only absent for DraftIssue, which itself populates ID, Title, and
// Body.
type ProjectV2ItemContent struct {
	Typename  string     `json:"__typename"`
	ID        string     `json:"id"`
	Number    int        `json:"number,omitempty"`
	Title     string     `json:"title"`
	URL       string     `json:"url,omitempty"`
	State     string     `json:"state,omitempty"`
	UpdatedAt string     `json:"updatedAt,omitempty"`
	ClosedAt  *string    `json:"closedAt,omitempty"`
	MergedAt  *string    `json:"mergedAt,omitempty"`
	Body      *string    `json:"body,omitempty"`
	Author    *Login     `json:"author,omitempty"`
	Assignees *Assignees `json:"assignees,omitempty"`
}

// ProjectV2FieldRef is the field selector embedded in every field-value node.
type ProjectV2FieldRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ProjectV2ItemFieldValue is the union of single-select / iteration / text /
// date values on a Projects v2 item. The Typename selects which fields are
// populated.
//
// Note: the name `ProjectV2FieldValue` is reserved for the genqlient-
// generated *input* type (the schema's `input ProjectV2FieldValue`), so
// the *response* shape used by [ListProjectV2Items] carries the `Item`
// infix to disambiguate.
type ProjectV2ItemFieldValue struct {
	Typename    string            `json:"__typename"`
	OptionID    string            `json:"optionId,omitempty"`
	Name        string            `json:"name,omitempty"`
	IterationID string            `json:"iterationId,omitempty"`
	Title       string            `json:"title,omitempty"`
	StartDate   string            `json:"startDate,omitempty"`
	Duration    int               `json:"duration,omitempty"`
	Text        string            `json:"text,omitempty"`
	Date        string            `json:"date,omitempty"`
	Field       ProjectV2FieldRef `json:"field"`
}

// ProjectV2ItemNode is the per-item shape returned by [ListProjectV2Items].
type ProjectV2ItemNode struct {
	ID          string                `json:"id"`
	UpdatedAt   string                `json:"updatedAt"`
	Content     *ProjectV2ItemContent `json:"content"`
	FieldValues struct {
		Nodes []ProjectV2ItemFieldValue `json:"nodes"`
	} `json:"fieldValues"`
}

// ListProjectV2ItemsResponse is the response of [ListProjectV2Items].
type ListProjectV2ItemsResponse struct {
	Node *struct {
		Items struct {
			Nodes []ProjectV2ItemNode `json:"nodes"`
		} `json:"items"`
	} `json:"node"`
}
