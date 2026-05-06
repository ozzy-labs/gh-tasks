package install

import "strings"

// MarkerTag is the single-identifier provenance string wrapping every
// marker block gh-tasks contributes to consumer-owned files. Both
// codex-cli and gemini-cli adapters share the same tag deliberately: the
// AGENTS.md they target is a shared aggregator, and reusing one tag means
// installing both adapters does not produce two duplicate marker blocks.
// Reference-count cleanup in PR 7 (--uninstall) keys off this constant.
const MarkerTag = "@ozzylabs/gh-tasks"

// MarkerBeginLine and MarkerEndLine are the verbatim HTML comment lines
// that delimit the gh-tasks-owned region. They are intentionally identical
// to the snippet markers emitted by the build-side adapters package
// (internal/adapters/adapters.go) so a switch from Renovate-driven sync to
// install-skills (or vice versa) does not produce a spurious merge-block
// rewrite.
const (
	MarkerBeginLine = "<!-- begin: " + MarkerTag + " -->"
	MarkerEndLine   = "<!-- end: " + MarkerTag + " -->"
)

// MergeMarkerBlock returns the new content for a consumer-owned file
// after replacing (or appending) the gh-tasks marker block with body.
//
// Behavior:
//
//   - If existing already contains a Begin/End pair, the contents between
//     them are replaced with body. Content outside the markers is
//     preserved byte-for-byte.
//   - If existing is non-empty but has no markers, the marker block is
//     appended after a single blank line so the consumer's existing
//     content stays at the top.
//   - If existing is empty, the result is just the marker block.
//
// The body is wrapped with a single blank line on each side of its
// markers so the result remains idempotent under Prettier's Markdown
// formatter (matching the convention adopted by the adapters package).
//
// MergeMarkerBlock is content-pure: feeding its output back as `existing`
// with the same body yields the same result.
func MergeMarkerBlock(existing, body string) string {
	wrapped := wrapMarker(body)

	beginIdx := strings.Index(existing, MarkerBeginLine)
	endIdx := strings.Index(existing, MarkerEndLine)

	if beginIdx >= 0 && endIdx > beginIdx {
		// Replace the existing block in place. `before` retains the
		// original whitespace that preceded the begin marker so the
		// surrounding consumer content is byte-identical to the input.
		// `wrapped` is self-contained (terminates with a single "\n");
		// strip the immediately-following "\n" from `after` so we don't
		// double up on a separator newline that already belonged to the
		// old block.
		before := existing[:beginIdx]
		afterStart := endIdx + len(MarkerEndLine)
		after := strings.TrimPrefix(existing[afterStart:], "\n")
		return before + wrapped + after
	}

	if existing == "" {
		return wrapped
	}
	// No marker present — append. Ensure exactly one blank line between
	// existing content and our block.
	trimmed := strings.TrimRight(existing, "\n")
	return trimmed + "\n\n" + wrapped
}

// HasMarkerBlock reports whether s already contains a complete (begin,
// end) marker pair. Used by adapters / tests to short-circuit Plan when
// the marker block is up-to-date and content matches.
func HasMarkerBlock(s string) bool {
	beginIdx := strings.Index(s, MarkerBeginLine)
	endIdx := strings.Index(s, MarkerEndLine)
	return beginIdx >= 0 && endIdx > beginIdx
}

// wrapMarker wraps body with the begin/end markers, padding with one
// blank line on each side so re-running the formatter on the result is a
// no-op. Trailing newline ensures the block ends cleanly when appended
// to a file that has no trailing newline.
func wrapMarker(body string) string {
	trimmed := strings.TrimRight(body, "\n")
	return MarkerBeginLine + "\n\n" + trimmed + "\n\n" + MarkerEndLine + "\n"
}
