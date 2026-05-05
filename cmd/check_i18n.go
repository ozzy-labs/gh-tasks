package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/internal/i18n"
	"github.com/ozzy-labs/gh-tasks/internal/i18ncheck"
)

func newCheckI18nCmd(_ Deps) *cobra.Command {
	var refs bool
	c := &cobra.Command{
		Use:    "check-i18n [paths...]",
		Short:  "Detect hardcoded non-ASCII string literals in Go source",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if refs {
				return runCheckI18nRefs(cmd, args)
			}
			return runCheckI18n(cmd, args)
		},
	}
	c.Flags().BoolVar(&refs, "refs", false,
		"Verify every r.T(...) / i18n.NewPayload(...) literal key exists in the en/ja catalog")
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
		return ErrSilentRuntime
	}
	files, err := i18ncheck.FindFiles(roots)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.OutOrStdout(),
		"OK: scanned %d files, no hardcoded non-ASCII literals\n", len(files))
	return nil
}

func runCheckI18nRefs(c *cobra.Command, args []string) error {
	roots := args
	if len(roots) == 0 {
		roots = []string{"cmd", "internal"}
	}
	catalogs := map[string]map[string]struct{}{
		string(i18n.LocaleEN): i18n.Keys(i18n.LocaleEN),
		string(i18n.LocaleJA): i18n.Keys(i18n.LocaleJA),
	}
	missing, dynamic, err := i18ncheck.CheckCatalogReferences(roots, catalogs)
	if err != nil {
		return err
	}
	for _, d := range dynamic {
		fmt.Fprintln(c.ErrOrStderr(), "warning: "+i18ncheck.FormatDynamicRef(d))
	}
	for _, m := range missing {
		fmt.Fprintln(c.ErrOrStderr(), i18ncheck.FormatMissingRef(m))
	}
	if len(missing) > 0 {
		fmt.Fprintln(c.ErrOrStderr())
		fmt.Fprintf(c.ErrOrStderr(), "%d undefined i18n key reference(s) detected.\n", len(missing))
		fmt.Fprintln(c.ErrOrStderr(),
			"Define these keys in internal/i18n/{en,ja}.json or remove the stale references.")
		return ErrSilentRuntime
	}
	files, err := i18ncheck.FindFiles(roots)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.OutOrStdout(),
		"OK: scanned %d files, every r.T / NewPayload literal key resolves in en/ja catalog\n",
		len(files))
	return nil
}
