package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newAddCmd(_ Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "add <title>",
		Short: "Add a task (Issue / Project draft / Milestone)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("not implemented yet (phase 2c)")
		},
	}
}
