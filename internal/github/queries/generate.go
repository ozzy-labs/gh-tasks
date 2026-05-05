package queries

// genqlient code generation. Run from the repository root:
//
//	go generate ./internal/github/queries/...
//
// The generator (registered as a tool dependency in go.mod per ADR-0006)
// reads `genqlient.yaml` from the repo root, which points at
// `internal/github/queries/operations.graphql` and the SDL schema in
// `internal/github/queries/schema.graphql`, and writes the typed bindings
// to `internal/github/queries/genqlient.go`. The generated file is
// committed; CI verifies the checked-in output matches the source.

//go:generate go run -mod=mod github.com/Khan/genqlient ../../../genqlient.yaml
