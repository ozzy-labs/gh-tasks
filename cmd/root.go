package cmd

import (
	"github.com/spf13/cobra"
)

// Version is overridden at build time via -ldflags.
var Version = "0.0.0-dev"

// Root constructs the gh-tasks cobra root command with all subcommands attached.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "gh-tasks",
		Short:         "GitHub Projects v2 / Issues / Milestone task management CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}

	root.PersistentFlags().String("scope", "", "scope to operate on (repo|org|user)")
	root.PersistentFlags().String("locale", "", "locale override (en|ja)")

	root.AddCommand(
		newAddCmd(),
		newListCmd(),
		newTodayCmd(),
		newDoneCmd(),
		newStandupCmd(),
		newReviewCmd(),
		newPlanCmd(),
		newTriageCmd(),
		newLinkCmd(),
		newProjectsCmd(),
		newBuildSkillsCmd(),
	)

	return root
}
