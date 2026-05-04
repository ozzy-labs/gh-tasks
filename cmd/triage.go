package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newTriageCmd(_ Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "triage",
		Short: "Triage untriaged Issues / Project draft items",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("not implemented yet (phase 2d)")
		},
	}
}
