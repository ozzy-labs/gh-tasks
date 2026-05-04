package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newProjectsCmd(_ Deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "projects",
		Short: "Manage Projects v2",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("not implemented yet (phase 2e)")
		},
	}

	c.AddCommand(
		&cobra.Command{
			Use:   "init",
			Short: "Initialize Projects v2 fields and templates",
			RunE: func(_ *cobra.Command, _ []string) error {
				return errors.New("not implemented yet (phase 2e)")
			},
		},
		&cobra.Command{
			Use:   "init-templates",
			Short: "Sync Issue templates",
			RunE: func(_ *cobra.Command, _ []string) error {
				return errors.New("not implemented yet (phase 2e)")
			},
		},
	)

	return c
}
