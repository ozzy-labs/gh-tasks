package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/i18ncheck"
)

func newCheckI18nCmd(_ Deps) *cobra.Command {
	c := &cobra.Command{
		Use:    "check-i18n [paths...]",
		Short:  "Detect hardcoded non-ASCII string literals in Go source",
		Hidden: true,
		RunE:   runCheckI18n,
	}
	return c
}

func runCheckI18n(c *cobra.Command, args []string) error {
	roots := args
	if len(roots) == 0 {
		roots = []string{"cmd", "internal"}
	}
	hits, err := i18ncheck.Scan(roots)
	if err != nil {
		return err
	}
	for _, h := range hits {
		fmt.Fprintln(c.ErrOrStderr(), i18ncheck.FormatHit(h))
	}
	if len(hits) > 0 {
		fmt.Fprintln(c.ErrOrStderr())
		fmt.Fprintf(c.ErrOrStderr(), "%d hardcoded non-ASCII literal(s) detected.\n", len(hits))
		fmt.Fprintln(c.ErrOrStderr(),
			"Move these strings to internal/i18n/{en,ja}.json and retrieve via i18n.T (repo-internal ADR-0005).")
		return ErrSilent
	}
	files, err := i18ncheck.FindFiles(roots)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.OutOrStdout(),
		"OK: scanned %d files, no hardcoded non-ASCII literals\n", len(files))
	return nil
}
