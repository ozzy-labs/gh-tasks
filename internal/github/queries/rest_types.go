// Package queries: REST-side response shapes used by the gh-tasks CLI.
//
// Genqlient covers every GraphQL operation the CLI issues. A small REST
// surface remains for endpoints with no GraphQL counterpart — currently
// just `POST /repos/{owner}/{repo}/milestones`, used by `cmd/plan.go` to
// create a new repo-scope milestone before binding issues to it via the
// genqlient-generated [UpdateIssueMilestone] mutation.
//
// This file holds only the REST response shape; everything else in this
// package is generated from `operations.graphql` (see `genqlient.go`).
package queries

// CreateMilestoneResult is the response of POST /repos/{owner}/{repo}/milestones.
//
// Only the fields actually consumed by `cmd/plan.go` are decoded — the
// node id (used as the GraphQL input for [UpdateIssueMilestone]), the
// numeric id, the milestone number (rendered in the user-visible URL),
// and the title (echoed back for confirmation).
type CreateMilestoneResult struct {
	NodeID string `json:"node_id"`
	ID     int    `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}
