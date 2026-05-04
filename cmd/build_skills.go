package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newBuildSkillsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:    "build-skills",
		Short:  "Build skill bundles for adapter agents",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet (phase 3)")
		},
	}

	c.Flags().Bool("check-diff", false, "fail if dist/ output differs from source SSOT")

	return c
}
