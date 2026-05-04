package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link <pr> <task>",
		Short: "Link a PR with an Issue / Project item",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet (phase 2)")
		},
	}
}
