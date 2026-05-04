package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newProjectsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "projects",
		Short: "Manage Projects v2",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet (phase 2)")
		},
	}

	c.AddCommand(
		&cobra.Command{
			Use:   "init",
			Short: "Initialize Projects v2 fields and templates",
			RunE: func(cmd *cobra.Command, args []string) error {
				return errors.New("not implemented yet (phase 2)")
			},
		},
		&cobra.Command{
			Use:   "init-templates",
			Short: "Sync Issue templates",
			RunE: func(cmd *cobra.Command, args []string) error {
				return errors.New("not implemented yet (phase 2)")
			},
		},
	)

	return c
}
