package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newTriageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "triage",
		Short: "Triage untriaged Issues / Project draft items",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet (phase 2)")
		},
	}
}
