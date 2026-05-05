package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// warnIfTruncated emits a stderr warning when a list query returned exactly
// the page limit, signalling that older entries may have been silently
// dropped. Pagination is not yet wired through the GraphQL operations
// (operations.graphql lacks pageInfo), so the most we can do is surface
// the truncation to the user. kind is a stable identifier (ASCII slug)
// embedded in the i18n message so the warning text can be localized
// without baking command-specific strings into the catalog.
func warnIfTruncated(c *cobra.Command, r Resolved, kind string, fetched, limit int) {
	if fetched < limit {
		return
	}
	fmt.Fprintln(c.ErrOrStderr(), r.T("warn.results.truncated", "kind", kind, "limit", limit))
}
