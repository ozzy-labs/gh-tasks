package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newTodayCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "today",
		Short: "Show today's tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet (phase 2)")
		},
	}
}
