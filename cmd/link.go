package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newLinkCmd(_ Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "link <pr> <task>",
		Short: "Link a PR with an Issue / Project item",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("not implemented yet (phase 2c)")
		},
	}
}
