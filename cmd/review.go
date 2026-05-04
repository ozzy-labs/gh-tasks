package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review",
		Short: "Generate retrospective summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet (phase 2)")
		},
	}
}
