package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet (phase 2)")
		},
	}
}
