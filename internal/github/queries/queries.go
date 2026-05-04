// Package queries holds the hand-written GraphQL operations and Go response
// types used by the gh-tasks CLI. Each block is a 1:1 port of the previous
// TypeScript counterpart in packages/gh-tasks/src/lib/queries/.
//
// genqlient adoption is tracked separately; the migration plan calls for it
// (ADR-0006 / 0007), but the schema fetch step is deferred to a follow-up
// because the sandboxed environment cannot fetch the public schema. Until
// then, response types are hand-written and kept in sync with the queries
// below.
package queries

import "encoding/json"

// Issue queries / types ------------------------------------------------------

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

// ListRepoIssues lists open Issues under a repository, ordered by recently
// updated.
const ListRepoIssues = `
query ListRepoIssues($owner: String!, $name: String!, $first: Int!) {
  repository(owner: $owner, name: $name) {
    issues(first: $first, states: OPEN, orderBy: { field: UPDATED_AT, direction: DESC }) {
      nodes {
        id
        number
        title
        url
        updatedAt
        author { login }
        assignees(first: 10) { nodes { login } }
      }
    }
  }
}`

// RepoIssueNode is the per-item shape returned by [ListRepoIssues].
type RepoIssueNode struct {
	ID        string    `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	UpdatedAt string    `json:"updatedAt"`
	Author    *Login    `json:"author,omitempty"`
	Assignees Assignees `json:"assignees"`
}

// Assignees wraps the paginated assignees list.
type Assignees struct {
	Nodes []Login `json:"nodes"`
}

// Login is a thin wrapper around { login: "..." }.
type Login struct {
	Login string `json:"login"`
}

// ListRepoIssuesResponse is the response of [ListRepoIssues].
type ListRepoIssuesResponse struct {
	Repository *struct {
		Issues struct {
			Nodes []RepoIssueNode `json:"nodes"`
		} `json:"issues"`
	} `json:"repository"`
}

// GetIssueByNumber resolves an Issue's node id from owner/name + number.
const GetIssueByNumber = `
query GetIssueByNumber($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    issue(number: $number) {
      id
      number
      url
      state
    }
  }
}`

// GetIssueByNumberResponse is the response of [GetIssueByNumber].
type GetIssueByNumberResponse struct {
	Repository *struct {
		Issue *struct {
			ID     string `json:"id"`
			Number int    `json:"number"`
			URL    string `json:"url"`
			State  string `json:"state"`
		} `json:"issue"`
	} `json:"repository"`
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

// ListRepoIssuesWithLabels lists open Issues with their label set, used for
// triage filtering.
const ListRepoIssuesWithLabels = `
query ListRepoIssuesWithLabels($owner: String!, $name: String!, $first: Int!) {
  repository(owner: $owner, name: $name) {
    issues(first: $first, states: OPEN, orderBy: { field: UPDATED_AT, direction: DESC }) {
      nodes {
        id
        number
        title
        url
        updatedAt
        labels(first: 20) { nodes { name } }
      }
    }
  }
}`

// LabelNode is a label name wrapper.
type LabelNode struct {
	Name string `json:"name"`
}

// Labels is the paginated labels list.
type Labels struct {
	Nodes []LabelNode `json:"nodes"`
}

// RepoIssueWithLabelsNode is the per-item shape returned by
// [ListRepoIssuesWithLabels].
type RepoIssueWithLabelsNode struct {
	ID        string `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	UpdatedAt string `json:"updatedAt"`
	Labels    Labels `json:"labels"`
}

// ListRepoIssuesWithLabelsResponse is the response of
// [ListRepoIssuesWithLabels].
type ListRepoIssuesWithLabelsResponse struct {
	Repository *struct {
		Issues struct {
			Nodes []RepoIssueWithLabelsNode `json:"nodes"`
		} `json:"issues"`
	} `json:"repository"`
}

// ListClosedIssues lists recently CLOSED Issues; the caller filters by
// closedAt window.
const ListClosedIssues = `
query ListClosedIssues($owner: String!, $name: String!, $first: Int!) {
  repository(owner: $owner, name: $name) {
    issues(first: $first, states: CLOSED, orderBy: { field: UPDATED_AT, direction: DESC }) {
      nodes {
        id
        number
        title
        url
        closedAt
        author { login }
        assignees(first: 10) { nodes { login } }
      }
    }
  }
}`

// ClosedIssueNode is the per-item shape returned by [ListClosedIssues].
type ClosedIssueNode struct {
	ID        string    `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	ClosedAt  string    `json:"closedAt"`
	Author    *Login    `json:"author,omitempty"`
	Assignees Assignees `json:"assignees"`
}

// ListClosedIssuesResponse is the response of [ListClosedIssues].
type ListClosedIssuesResponse struct {
	Repository *struct {
		Issues struct {
			Nodes []ClosedIssueNode `json:"nodes"`
		} `json:"issues"`
	} `json:"repository"`
}

// Repository queries / types -------------------------------------------------

// GetRepositoryID resolves a repository's node id.
const GetRepositoryID = `
query GetRepositoryId($owner: String!, $name: String!) {
  repository(owner: $owner, name: $name) {
    id
  }
}`

// GetRepositoryIDResponse is the response of [GetRepositoryID].
type GetRepositoryIDResponse struct {
	Repository *struct {
		ID string `json:"id"`
	} `json:"repository"`
}

// Viewer queries / types -----------------------------------------------------

// GetViewerLogin resolves the authenticated user's login.
const GetViewerLogin = `
query GetViewerLogin {
  viewer { login }
}`

// GetViewerLoginResponse is the response of [GetViewerLogin].
type GetViewerLoginResponse struct {
	Viewer struct {
		Login string `json:"login"`
	} `json:"viewer"`
}

// GetViewerID resolves the viewer's node id (used for `--owner @me`).
const GetViewerID = `
query GetViewerId {
  viewer { id login }
}`

// GetViewerIDResponse is the response of [GetViewerID].
type GetViewerIDResponse struct {
	Viewer struct {
		ID    string `json:"id"`
		Login string `json:"login"`
	} `json:"viewer"`
}

// PR queries / types ---------------------------------------------------------

// GetPullRequestByNumber resolves a PR's id and current body.
const GetPullRequestByNumber = `
query GetPullRequestByNumber($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      id
      number
      url
      body
    }
  }
}`

// GetPullRequestByNumberResponse is the response of [GetPullRequestByNumber].
type GetPullRequestByNumberResponse struct {
	Repository *struct {
		PullRequest *struct {
			ID     string `json:"id"`
			Number int    `json:"number"`
			URL    string `json:"url"`
			Body   string `json:"body"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

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

// ListMergedPRs lists recently MERGED PRs; the caller filters by mergedAt.
const ListMergedPRs = `
query ListMergedPRs($owner: String!, $name: String!, $first: Int!) {
  repository(owner: $owner, name: $name) {
    pullRequests(first: $first, states: MERGED, orderBy: { field: UPDATED_AT, direction: DESC }) {
      nodes {
        id
        number
        title
        url
        mergedAt
        author { login }
        assignees(first: 10) { nodes { login } }
      }
    }
  }
}`

// MergedPRNode is the per-item shape returned by [ListMergedPRs].
type MergedPRNode struct {
	ID        string    `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	MergedAt  string    `json:"mergedAt"`
	Author    *Login    `json:"author,omitempty"`
	Assignees Assignees `json:"assignees"`
}

// ListMergedPRsResponse is the response of [ListMergedPRs].
type ListMergedPRsResponse struct {
	Repository *struct {
		PullRequests struct {
			Nodes []MergedPRNode `json:"nodes"`
		} `json:"pullRequests"`
	} `json:"repository"`
}

// Milestone queries / types --------------------------------------------------

// ListRepoIssuesWithMilestone lists open Issues with their milestone binding.
const ListRepoIssuesWithMilestone = `
query ListRepoIssuesWithMilestone($owner: String!, $name: String!, $first: Int!) {
  repository(owner: $owner, name: $name) {
    issues(first: $first, states: OPEN, orderBy: { field: UPDATED_AT, direction: DESC }) {
      nodes {
        id
        number
        title
        url
        updatedAt
        milestone {
          id
          number
          title
        }
      }
    }
  }
}`

// MilestoneRef points to an issue's currently-bound milestone.
type MilestoneRef struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// RepoIssueWithMilestoneNode is the per-item shape returned by
// [ListRepoIssuesWithMilestone].
type RepoIssueWithMilestoneNode struct {
	ID        string        `json:"id"`
	Number    int           `json:"number"`
	Title     string        `json:"title"`
	URL       string        `json:"url"`
	UpdatedAt string        `json:"updatedAt"`
	Milestone *MilestoneRef `json:"milestone"`
}

// ListRepoIssuesWithMilestoneResponse is the response of
// [ListRepoIssuesWithMilestone].
type ListRepoIssuesWithMilestoneResponse struct {
	Repository *struct {
		Issues struct {
			Nodes []RepoIssueWithMilestoneNode `json:"nodes"`
		} `json:"issues"`
	} `json:"repository"`
}

// ListMilestones lists recently updated open milestones.
const ListMilestones = `
query ListMilestones($owner: String!, $name: String!, $first: Int!) {
  repository(owner: $owner, name: $name) {
    milestones(first: $first, states: OPEN, orderBy: { field: UPDATED_AT, direction: DESC }) {
      nodes {
        id
        number
        title
      }
    }
  }
}`

// ListMilestonesResponse is the response of [ListMilestones].
type ListMilestonesResponse struct {
	Repository *struct {
		Milestones struct {
			Nodes []MilestoneRef `json:"nodes"`
		} `json:"milestones"`
	} `json:"repository"`
}

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

// Project queries / types ----------------------------------------------------

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

// GetUserProjectV2 resolves a user-scope Projects v2 node by login + number.
const GetUserProjectV2 = `
query GetUserProjectV2($login: String!, $number: Int!) {
  user(login: $login) {
    projectV2(number: $number) {
      id
      number
      title
    }
  }
}`

// ProjectV2Ref is a Projects v2 minimal reference returned by lookups.
type ProjectV2Ref struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// GetUserProjectV2Response is the response of [GetUserProjectV2].
type GetUserProjectV2Response struct {
	User *struct {
		ProjectV2 *ProjectV2Ref `json:"projectV2"`
	} `json:"user"`
}

// GetOrgProjectV2 resolves an org-scope Projects v2 node by login + number.
const GetOrgProjectV2 = `
query GetOrgProjectV2($login: String!, $number: Int!) {
  organization(login: $login) {
    projectV2(number: $number) {
      id
      number
      title
    }
  }
}`

// GetOrgProjectV2Response is the response of [GetOrgProjectV2].
type GetOrgProjectV2Response struct {
	Organization *struct {
		ProjectV2 *ProjectV2Ref `json:"projectV2"`
	} `json:"organization"`
}

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
//
// For every other DataType (TEXT / NUMBER / DATE / TITLE / ASSIGNEES / …)
// both fields are absent and the response carries only the
// ProjectV2FieldCommon shape (id / name / dataType).
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
// Body. The remaining fields (ClosedAt, MergedAt, Body, Number, URL, State,
// UpdatedAt, Author, Assignees) are variant-specific and modeled as
// pointers / value types whose zero value is meaningful — branch on
// Typename before reading them. omitempty is intentionally omitted for the
// always-present fields on the Issue/PullRequest path so the struct
// signature reflects the schema invariant; this struct is decode-only and
// never marshaled back, so the tag has no runtime effect.
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
//
// Field is always populated for every union variant since the GraphQL
// selection set in [ListProjectV2Items] requests
// `field { ... on ProjectV2FieldCommon { id name } }` on every branch. It
// is therefore kept as a value type rather than a pointer; a zero
// {ID:"", Name:""} would only appear if GitHub silently omitted the
// selection, which would be a schema-level breakage rather than a
// per-item nil. genqlient adoption (tracked separately) may revisit this
// when each variant gets its own generated struct.
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

// Project init queries / types ----------------------------------------------

// GetOwnerID resolves a user/org's node id from a login.
const GetOwnerID = `
query GetOwnerId($login: String!) {
  repositoryOwner(login: $login) {
    __typename
    id
    login
  }
}`

// GetOwnerIDResponse is the response of [GetOwnerID].
type GetOwnerIDResponse struct {
	RepositoryOwner *struct {
		Typename string `json:"__typename"`
		ID       string `json:"id"`
		Login    string `json:"login"`
	} `json:"repositoryOwner"`
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
