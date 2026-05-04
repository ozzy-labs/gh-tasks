package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newDoneCmd(_ Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "done <task>",
		Short: "Mark a task as done",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("not implemented yet (phase 2c)")
		},
	}
}
