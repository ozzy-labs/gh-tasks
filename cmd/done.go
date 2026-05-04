package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <task>",
		Short: "Mark a task as done",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet (phase 2)")
		},
	}
}
