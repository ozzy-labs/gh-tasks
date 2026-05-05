package queries_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/Khan/genqlient/graphql"

	"github.com/ozzy-labs/gh-tasks/internal/github/queries"
)

func jsonMarshalImpl(v any) ([]byte, error)   { return json.Marshal(v) }
func jsonUnmarshalImpl(b []byte, v any) error { return json.Unmarshal(b, v) }

// ===== Response builders =====================================================

// repoIssuesPage builds a Repository envelope with the supplied issues and
// a pageInfo controlled by hasNext / endCursor.
func repoIssuesPage(nodes []*queries.RepoIssue, hasNext bool, endCursor *string) *queries.ListRepoIssuesRepository {
	return &queries.ListRepoIssuesRepository{
		Issues: &queries.ListRepoIssuesRepositoryIssuesIssueConnection{
			PageInfo: &queries.ListRepoIssuesRepositoryIssuesIssueConnectionPageInfo{
				HasNextPage: hasNext,
				EndCursor:   endCursor,
			},
			Nodes: nodes,
		},
	}
}

// makeRepoIssues generates `count` synthetic issue nodes numbered from
// `start` upward.
func makeRepoIssues(start, count int) []*queries.RepoIssue {
	out := make([]*queries.RepoIssue, count)
	for i := 0; i < count; i++ {
		num := start + i
		out[i] = &queries.RepoIssue{Id: fmt.Sprintf("I_%d", num), Number: num}
	}
	return out
}

// projectItemsNode builds a ProjectV2 narrowed-node holding the supplied
// project items + pageInfo.
func projectItemsNode(nodes []*queries.ProjectV2ItemNode, hasNext bool, endCursor *string) *queries.ProjectV2ItemsNodeProjectV2 {
	return &queries.ProjectV2ItemsNodeProjectV2{
		Items: &queries.ProjectV2ItemsNodeItemsProjectV2ItemConnection{
			PageInfo: &queries.ProjectV2ItemsNodeItemsProjectV2ItemConnectionPageInfo{
				HasNextPage: hasNext,
				EndCursor:   endCursor,
			},
			Nodes: nodes,
		},
	}
}

// projectFieldsNode builds a ProjectV2 narrowed-node holding the supplied
// fields + pageInfo. Each `nodes` element is wrapped in `*ProjectV2FieldNode`
// (a pointer to interface) to match genqlient's generated schema.
func projectFieldsNode(nodes []queries.ProjectV2FieldNode, hasNext bool, endCursor *string) *queries.ProjectV2FieldsNodeProjectV2 {
	wrapped := make([]*queries.ProjectV2FieldNode, len(nodes))
	for i := range nodes {
		n := nodes[i]
		wrapped[i] = &n
	}
	return &queries.ProjectV2FieldsNodeProjectV2{
		Fields: &queries.ProjectV2FieldsNodeFieldsProjectV2FieldConfigurationConnection{
			PageInfo: &queries.ProjectV2FieldsNodeFieldsProjectV2FieldConfigurationConnectionPageInfo{
				HasNextPage: hasNext,
				EndCursor:   endCursor,
			},
			Nodes: wrapped,
		},
	}
}

// scriptedClient implements graphql.Client by returning canned responses
// keyed by (operation name, after-cursor) per call. The script is consumed
// in registration order: each MakeRequest call pops the head of the queue
// for the matching OpName, asserts the cursor argument matches the
// recorded one, and copies the canned response into resp.Data.
//
// The fake exists in the queries package's external test (queries_test) so
// it gets to use the exported genqlient `MakeRequest` shape without
// circular imports. Tests live alongside pagination.go to keep their
// invariants close to the implementation they pin.
type scriptedClient struct {
	t      *testing.T
	steps  []scriptStep
	cursor int
}

type scriptStep struct {
	op       string
	wantSize int
	// wantAfter == nil asserts that the request's `after` variable is nil
	// or absent. Otherwise the request must carry exactly *wantAfter.
	wantAfter *string
	respond   func(out any)
	err       error
}

func (s *scriptedClient) MakeRequest(_ context.Context, req *graphql.Request, resp *graphql.Response) error {
	if s.cursor >= len(s.steps) {
		s.t.Fatalf("scripted client: unexpected extra request OpName=%q (script exhausted at %d)", req.OpName, s.cursor)
	}
	step := s.steps[s.cursor]
	s.cursor++
	if req.OpName != step.op {
		s.t.Fatalf("scripted client: step %d expected op=%q, got %q", s.cursor-1, step.op, req.OpName)
	}
	gotSize, gotAfter := extractPageVars(req)
	if step.wantSize != 0 && gotSize != step.wantSize {
		s.t.Fatalf("scripted client: step %d expected first=%d, got %d", s.cursor-1, step.wantSize, gotSize)
	}
	switch {
	case step.wantAfter == nil && gotAfter != nil:
		s.t.Fatalf("scripted client: step %d expected nil after, got %q", s.cursor-1, *gotAfter)
	case step.wantAfter != nil && gotAfter == nil:
		s.t.Fatalf("scripted client: step %d expected after=%q, got nil", s.cursor-1, *step.wantAfter)
	case step.wantAfter != nil && gotAfter != nil && *step.wantAfter != *gotAfter:
		s.t.Fatalf("scripted client: step %d expected after=%q, got %q", s.cursor-1, *step.wantAfter, *gotAfter)
	}
	if step.err != nil {
		return step.err
	}
	if step.respond != nil {
		step.respond(resp.Data)
	}
	return nil
}

// extractPageVars peeks at the variables struct passed by the
// genqlient-generated wrapper. Each Listxxx wrapper passes a struct with
// First / After fields; we use a small typed switch to read them without
// depending on the genqlient-internal name.
func extractPageVars(req *graphql.Request) (int, *string) {
	type pageReader interface{ getFirstAfter() (int, *string) }
	if r, ok := req.Variables.(pageReader); ok {
		return r.getFirstAfter()
	}
	// fall through to reflection-free generic struct probe via type assertion
	type firstAfterGetter interface {
		GetFirst() int
		GetAfter() *string
	}
	if v, ok := req.Variables.(firstAfterGetter); ok {
		return v.GetFirst(), v.GetAfter()
	}
	// As a backstop, hit the small set of input types directly. Each
	// generated `__ListXxxInput` carries First / After fields with the
	// same JSON tags but no shared interface, so extend this list when a
	// new query is added.
	if v, ok := readListInputFields(req.Variables); ok {
		return v.first, v.after
	}
	return 0, nil
}

// listInputView is the stripped-down view of any genqlient-generated
// `__ListXxxInput` struct that we care about for pagination tests.
type listInputView struct {
	first int
	after *string
}

// readListInputFields reads the First / After fields out of a value via
// JSON round-trip, which avoids importing reflect-style helpers and works
// for any genqlient input regardless of the generated unexported name.
func readListInputFields(v any) (listInputView, bool) {
	type pageInput struct {
		First int     `json:"first"`
		After *string `json:"after"`
	}
	// Use Marshal/Unmarshal to copy whatever the generator produced into
	// our shared view. Variables structs are pointer-receiver, but
	// json.Marshal handles both via reflect anyway.
	bs, err := jsonMarshal(v)
	if err != nil {
		return listInputView{}, false
	}
	var pi pageInput
	if err := jsonUnmarshal(bs, &pi); err != nil {
		return listInputView{}, false
	}
	return listInputView{first: pi.First, after: pi.After}, true
}

// jsonMarshal / jsonUnmarshal indirection keeps the test's import block
// short — encoding/json is the only stdlib dep and we forward.
var (
	jsonMarshal   = jsonMarshalImpl
	jsonUnmarshal = jsonUnmarshalImpl
)

// =============================================================================
// PaginateRepoIssues — basic pagination, error propagation, ErrRepoNotFound
// =============================================================================

func TestPaginateRepoIssues_SinglePage(t *testing.T) {
	t.Parallel()
	step := scriptStep{
		op:        "ListRepoIssues",
		wantSize:  30,
		wantAfter: nil,
		respond: func(out any) {
			r := out.(*queries.ListRepoIssuesResponse)
			r.Repository = repoIssuesPage([]*queries.RepoIssue{
				{Id: "I_1", Number: 1},
				{Id: "I_2", Number: 2},
			}, false, nil)
		},
	}
	client := &scriptedClient{t: t, steps: []scriptStep{step}}
	got, err := queries.PaginateRepoIssues(context.Background(), client, "o", "n", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].Id != "I_1" || got[1].Id != "I_2" {
		t.Errorf("unexpected items: %+v", got)
	}
}

func TestPaginateRepoIssues_MultiPage(t *testing.T) {
	t.Parallel()
	cur := "C1"
	steps := []scriptStep{
		{
			op: "ListRepoIssues", wantSize: 100, wantAfter: nil,
			respond: func(out any) {
				r := out.(*queries.ListRepoIssuesResponse)
				r.Repository = repoIssuesPage(makeRepoIssues(0, 100), true, &cur)
			},
		},
		{
			op: "ListRepoIssues", wantSize: 50, wantAfter: &cur,
			respond: func(out any) {
				r := out.(*queries.ListRepoIssuesResponse)
				r.Repository = repoIssuesPage(makeRepoIssues(100, 50), false, nil)
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateRepoIssues(context.Background(), client, "o", "n", 150)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 150 {
		t.Fatalf("expected 150 items, got %d", len(got))
	}
	if got[0].Number != 0 || got[149].Number != 149 {
		t.Errorf("unexpected page boundary: first=%d last=%d", got[0].Number, got[149].Number)
	}
}

func TestPaginateRepoIssues_LimitCap(t *testing.T) {
	t.Parallel()
	// limit=50, page1 returns 100 (which won't happen in practice because we
	// request first=50, but the paginator must defensively cap).
	steps := []scriptStep{
		{
			op: "ListRepoIssues", wantSize: 50, wantAfter: nil,
			respond: func(out any) {
				r := out.(*queries.ListRepoIssuesResponse)
				r.Repository = repoIssuesPage(makeRepoIssues(0, 100), false, nil)
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateRepoIssues(context.Background(), client, "o", "n", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 50 {
		t.Errorf("limit=50 should cap to 50 even when server returned 100, got %d", len(got))
	}
}

func TestPaginateRepoIssues_MaxPagesSafetyValve(t *testing.T) {
	t.Parallel()
	// Pretend the server keeps saying hasNextPage=true forever; paginator
	// must stop after maxPages (=10) at maxPageSize (=100), capping at 1000.
	steps := make([]scriptStep, 10)
	for i := range steps {
		i := i
		want := fmt.Sprintf("C%d", i)
		var afterPtr *string
		if i > 0 {
			prev := fmt.Sprintf("C%d", i-1)
			afterPtr = &prev
		}
		steps[i] = scriptStep{
			op: "ListRepoIssues", wantSize: 100, wantAfter: afterPtr,
			respond: func(out any) {
				r := out.(*queries.ListRepoIssuesResponse)
				r.Repository = repoIssuesPage(makeRepoIssues(i*100, 100), true, &want)
			},
		}
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateRepoIssues(context.Background(), client, "o", "n", 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1000 {
		t.Errorf("expected maxPages*maxPageSize=1000 items, got %d", len(got))
	}
	if client.cursor != 10 {
		t.Errorf("expected exactly 10 requests, got %d", client.cursor)
	}
}

func TestPaginateRepoIssues_ErrorMidStreamDiscardsAccumulated(t *testing.T) {
	t.Parallel()
	cur := "C1"
	steps := []scriptStep{
		{
			op: "ListRepoIssues", wantSize: 100, wantAfter: nil,
			respond: func(out any) {
				r := out.(*queries.ListRepoIssuesResponse)
				r.Repository = repoIssuesPage(makeRepoIssues(0, 100), true, &cur)
			},
		},
		{
			op: "ListRepoIssues", wantSize: 100, wantAfter: &cur,
			err: errors.New("boom"),
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateRepoIssues(context.Background(), client, "o", "n", 200)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected raw 'boom' error, got %v", err)
	}
	if got != nil {
		t.Errorf("partial success must be discarded; got %d items", len(got))
	}
}

func TestPaginateRepoIssues_RepoNotFound(t *testing.T) {
	t.Parallel()
	steps := []scriptStep{
		{
			op: "ListRepoIssues", wantSize: 30,
			respond: func(out any) {
				r := out.(*queries.ListRepoIssuesResponse)
				r.Repository = nil
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	_, err := queries.PaginateRepoIssues(context.Background(), client, "o", "n", 30)
	if !errors.Is(err, queries.ErrRepoNotFound) {
		t.Errorf("expected ErrRepoNotFound, got %v", err)
	}
}

// =============================================================================
// PaginateProjectV2Items — node-variant detection
// =============================================================================

func TestPaginateProjectV2Items_ProjectNotFoundOnWrongVariant(t *testing.T) {
	t.Parallel()
	// Simulate node(id:) resolving to something that isn't ProjectV2 by
	// leaving Node nil — UnmarshalJSON on the response only constructs
	// ProjectV2ItemsNodeProjectV2 when the typed payload says so.
	steps := []scriptStep{
		{
			op: "ListProjectV2Items", wantSize: 30,
			respond: func(out any) {
				r := out.(*queries.ListProjectV2ItemsResponse)
				r.Node = nil
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	_, err := queries.PaginateProjectV2Items(context.Background(), client, "PVT_1", 30)
	if !errors.Is(err, queries.ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

func TestPaginateProjectV2Items_HappyPath(t *testing.T) {
	t.Parallel()
	steps := []scriptStep{
		{
			op: "ListProjectV2Items", wantSize: 30,
			respond: func(out any) {
				r := out.(*queries.ListProjectV2ItemsResponse)
				node := projectItemsNode([]*queries.ProjectV2ItemNode{
					{Id: "I_1"}, {Id: "I_2"},
				}, false, nil)
				var asIface queries.ProjectV2ItemsNode = node
				r.Node = &asIface
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateProjectV2Items(context.Background(), client, "PVT_1", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 items, got %d", len(got))
	}
}

// =============================================================================
// PaginateProjectV2Fields — interface-variant accumulation across pages
// =============================================================================

func TestPaginateProjectV2Fields_AccumulatesVariantsAcrossPages(t *testing.T) {
	t.Parallel()
	cur := "C1"
	page1Common := &queries.ProjectV2FieldNodeProjectV2Field{Id: "F_C", Name: "Common", DataType: "TEXT"}
	page1SS := &queries.ProjectV2FieldNodeProjectV2SingleSelectField{Id: "F_S", Name: "Status", DataType: "SINGLE_SELECT"}
	page2Iter := &queries.ProjectV2FieldNodeProjectV2IterationField{Id: "F_I", Name: "Iter", DataType: "ITERATION"}
	steps := []scriptStep{
		{
			op: "ListProjectV2Fields", wantSize: 100, wantAfter: nil,
			respond: func(out any) {
				r := out.(*queries.ListProjectV2FieldsResponse)
				node := projectFieldsNode([]queries.ProjectV2FieldNode{
					page1Common, page1SS,
				}, true, &cur)
				var asIface queries.ProjectV2FieldsNode = node
				r.Node = &asIface
			},
		},
		{
			// Page 1 returned 2 items, so page 2 requests the
			// remaining capacity (100 - 2 = 98).
			op: "ListProjectV2Fields", wantSize: 98, wantAfter: &cur,
			respond: func(out any) {
				r := out.(*queries.ListProjectV2FieldsResponse)
				node := projectFieldsNode([]queries.ProjectV2FieldNode{
					page2Iter,
				}, false, nil)
				var asIface queries.ProjectV2FieldsNode = node
				r.Node = &asIface
			},
		},
	}
	client := &scriptedClient{t: t, steps: steps}
	got, err := queries.PaginateProjectV2Fields(context.Background(), client, "PVT_1", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 accumulated nodes, got %d", len(got))
	}
	// Verify each variant kept its concrete genqlient type.
	if _, ok := got[0].(*queries.ProjectV2FieldNodeProjectV2Field); !ok {
		t.Errorf("got[0] expected *ProjectV2FieldNodeProjectV2Field, got %T", got[0])
	}
	if _, ok := got[1].(*queries.ProjectV2FieldNodeProjectV2SingleSelectField); !ok {
		t.Errorf("got[1] expected *ProjectV2FieldNodeProjectV2SingleSelectField, got %T", got[1])
	}
	if _, ok := got[2].(*queries.ProjectV2FieldNodeProjectV2IterationField); !ok {
		t.Errorf("got[2] expected *ProjectV2FieldNodeProjectV2IterationField, got %T", got[2])
	}
}
