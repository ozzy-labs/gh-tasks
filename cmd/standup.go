package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newStandupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "standup",
		Short: "Generate standup summary of recent activity",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet (phase 2)")
		},
	}
}
