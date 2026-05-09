package jsonout

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/itchyny/gojq"
)

// JQError wraps a gojq parse or runtime failure so cmd/* can detect it via
// errors.As and emit a localized hint while preserving the underlying gojq
// message for debug output.
type JQError struct {
	Phase string // "parse" or "runtime"
	Err   error
}

func (e *JQError) Error() string {
	return fmt.Sprintf("jq %s: %v", e.Phase, e.Err)
}

func (e *JQError) Unwrap() error { return e.Err }

// runJQ applies query to data and writes the resulting values to w as JSON
// (one value per line for arrays/objects, with two-space indent matching
// `gh issue list --jq` formatting). string / number primitives are emitted
// in their JSON form (quoted / unquoted respectively).
func runJQ(w io.Writer, data any, query string) error {
	q, err := gojq.Parse(query)
	if err != nil {
		return &JQError{Phase: "parse", Err: err}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	iter := q.Run(data)
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if jqErr, isErr := v.(error); isErr {
			return &JQError{Phase: "runtime", Err: jqErr}
		}
		if err := enc.Encode(v); err != nil {
			return fmt.Errorf("encode jq output: %w", err)
		}
	}
}
