package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newPlanCmd(_ Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Plan tasks for a period (daily / weekly / sprint)",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("not implemented yet (phase 2d)")
		},
	}
}
