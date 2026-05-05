// Package queries holds the GraphQL operations and Go response types used
// by the gh-tasks CLI.
//
// Two flavors coexist while the genqlient migration (#229 / #230 / #231)
// is in flight:
//
//   - genqlient-generated typed operations in `genqlient.go`, sourced
//     from `operations.graphql` (SSOT) + `schema.graphql` (GitHub public
//     SDL). See `generate.go` for the `go generate` invocation. Read-side
//     operations were migrated under #230.
//   - Hand-written mutation operations and response types in this file.
//     Each block is a 1:1 port of the previous TypeScript counterpart in
//     packages/gh-tasks/src/lib/queries/. Mutations are tracked for
//     migration under #231.
//
// New operations should be added to `operations.graphql` and consumed via
// the genqlient-generated functions.
package queries

import "encoding/json"

// Login is a thin wrapper around { login: "..." } shared by hand-written
// shapes. Each genqlient-generated read operation has its own per-
// operation Login type; this is kept for the still-hand-written
// `ListProjectV2Items` ProjectV2ItemContent shape (deferred from #230).
type Login struct {
	Login string `json:"login"`
}

// Assignees wraps the paginated assignees list returned by hand-written
// queries (currently only `ListProjectV2Items.content.assignees`).
type Assignees struct {
	Nodes []Login `json:"nodes"`
}

// Issue mutations / types ----------------------------------------------------

// CreateIssue mutates a new Issue under a given repository.
const CreateIssue = `
mutation CreateIssue($input: CreateIssueInput!) {
  createIssue(input: $input) {
    issue {
      id
      number
      url
    }
  }
}`

// CreateIssueResponse is the response of [CreateIssue].
type CreateIssueResponse struct {
	CreateIssue struct {
		Issue struct {
			ID     string `json:"id"`
			Number int    `json:"number"`
			URL    string `json:"url"`
		} `json:"issue"`
	} `json:"createIssue"`
}

// CloseIssue mutates an Issue to state CLOSED.
const CloseIssue = `
mutation CloseIssue($input: CloseIssueInput!) {
  closeIssue(input: $input) {
    issue {
      id
      number
      url
      state
    }
  }
}`

// CloseIssueResponse is the response of [CloseIssue].
type CloseIssueResponse struct {
	CloseIssue struct {
		Issue struct {
			ID     string `json:"id"`
			Number int    `json:"number"`
			URL    string `json:"url"`
			State  string `json:"state"`
		} `json:"issue"`
	} `json:"closeIssue"`
}

// PR mutations / types -------------------------------------------------------

// UpdatePullRequest mutates a PR's body.
const UpdatePullRequest = `
mutation UpdatePullRequest($input: UpdatePullRequestInput!) {
  updatePullRequest(input: $input) {
    pullRequest {
      id
      number
      url
    }
  }
}`

// UpdatePullRequestResponse is the response of [UpdatePullRequest].
type UpdatePullRequestResponse struct {
	UpdatePullRequest struct {
		PullRequest struct {
			ID     string `json:"id"`
			Number int    `json:"number"`
			URL    string `json:"url"`
		} `json:"pullRequest"`
	} `json:"updatePullRequest"`
}

// Milestone mutations / REST helpers -----------------------------------------

// UpdateIssueMilestone binds (or clears) a milestone on an Issue.
const UpdateIssueMilestone = `
mutation UpdateIssueMilestone($input: UpdateIssueInput!) {
  updateIssue(input: $input) {
    issue {
      id
      number
      url
      milestone {
        id
        number
        title
      }
    }
  }
}`

// MilestoneRef points to an issue's currently-bound milestone. Retained
// here as a value type so the [UpdateIssueMilestone] mutation response
// can embed it without depending on the genqlient-generated milestone
// shapes (each generated type is operation-scoped).
type MilestoneRef struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// UpdateIssueMilestoneResponse is the response of [UpdateIssueMilestone].
type UpdateIssueMilestoneResponse struct {
	UpdateIssue struct {
		Issue struct {
			ID        string        `json:"id"`
			Number    int           `json:"number"`
			URL       string        `json:"url"`
			Milestone *MilestoneRef `json:"milestone"`
		} `json:"issue"`
	} `json:"updateIssue"`
}

// CreateMilestoneResult is the REST response of POST /repos/{o}/{r}/milestones.
type CreateMilestoneResult struct {
	NodeID string `json:"node_id"`
	ID     int    `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// Project mutations / types --------------------------------------------------

// AddProjectV2DraftIssue adds a draft item to a Projects v2 board.
const AddProjectV2DraftIssue = `
mutation AddProjectV2DraftIssue($input: AddProjectV2DraftIssueInput!) {
  addProjectV2DraftIssue(input: $input) {
    projectItem { id }
  }
}`

// AddProjectV2DraftIssueResponse is the response of [AddProjectV2DraftIssue].
type AddProjectV2DraftIssueResponse struct {
	AddProjectV2DraftIssue struct {
		ProjectItem struct {
			ID string `json:"id"`
		} `json:"projectItem"`
	} `json:"addProjectV2DraftIssue"`
}

// AddProjectV2ItemByID adds an existing Issue or PR to a Projects v2 board.
const AddProjectV2ItemByID = `
mutation AddProjectV2ItemById($input: AddProjectV2ItemByIdInput!) {
  addProjectV2ItemById(input: $input) {
    item { id }
  }
}`

// AddProjectV2ItemByIDResponse is the response of [AddProjectV2ItemByID].
type AddProjectV2ItemByIDResponse struct {
	AddProjectV2ItemByID struct {
		Item struct {
			ID string `json:"id"`
		} `json:"item"`
	} `json:"addProjectV2ItemById"`
}

// UpdateProjectV2ItemFieldValue updates a single field value on a project
// item. The value shape is constructed by the caller per the target field's
// dataType.
const UpdateProjectV2ItemFieldValue = `
mutation UpdateProjectV2ItemFieldValue($input: UpdateProjectV2ItemFieldValueInput!) {
  updateProjectV2ItemFieldValue(input: $input) {
    projectV2Item { id }
  }
}`

// UpdateProjectV2ItemFieldValueResponse is the response of
// [UpdateProjectV2ItemFieldValue].
type UpdateProjectV2ItemFieldValueResponse struct {
	UpdateProjectV2ItemFieldValue struct {
		ProjectV2Item struct {
			ID string `json:"id"`
		} `json:"projectV2Item"`
	} `json:"updateProjectV2ItemFieldValue"`
}

// CreateProjectV2 creates a Projects v2 board owned by ownerId.
const CreateProjectV2 = `
mutation CreateProjectV2($input: CreateProjectV2Input!) {
  createProjectV2(input: $input) {
    projectV2 {
      id
      number
      title
      url
    }
  }
}`

// CreateProjectV2Response is the response of [CreateProjectV2].
type CreateProjectV2Response struct {
	CreateProjectV2 struct {
		ProjectV2 struct {
			ID     string `json:"id"`
			Number int    `json:"number"`
			Title  string `json:"title"`
			URL    string `json:"url"`
		} `json:"projectV2"`
	} `json:"createProjectV2"`
}

// CreateProjectV2Field adds a custom field to an existing Projects v2 board.
const CreateProjectV2Field = `
mutation CreateProjectV2Field($input: CreateProjectV2FieldInput!) {
  createProjectV2Field(input: $input) {
    projectV2Field {
      ... on ProjectV2FieldCommon {
        id
        name
        dataType
      }
    }
  }
}`

// CreateProjectV2FieldResponse is the response of [CreateProjectV2Field].
type CreateProjectV2FieldResponse struct {
	CreateProjectV2Field struct {
		ProjectV2Field struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			DataType string `json:"dataType"`
		} `json:"projectV2Field"`
	} `json:"createProjectV2Field"`
}

// ProjectV2 read operations (deferred from #230) ----------------------------
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

// ProjectV2FieldValue is the union of single-select / iteration / text / date
// values on a Projects v2 item. The Typename selects which fields are
// populated.
type ProjectV2FieldValue struct {
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
		Nodes []ProjectV2FieldValue `json:"nodes"`
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

// MustMarshal is a helper for callers that need raw JSON pass-through (e.g.
// nested input payloads passed verbatim to mutations). Panics on a marshal
// error to keep call sites concise — the inputs constructed in this package
// are always marshalable.
func MustMarshal(v any) json.RawMessage {
	out, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return out
}
