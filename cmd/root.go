package cmd

import (
	"github.com/spf13/cobra"
)

// Version reflects the current release tag. release-please rewrites the literal
// on each release via the x-release-please-version annotation below; the value
// committed in main between releases is the prior tag (i.e. matches what `gh
// extension install` would have fetched at that point).
var Version = "0.3.0" // x-release-please-version

// Root constructs the gh-tasks cobra root command using DefaultDeps.
func Root() *cobra.Command {
	return RootWithDeps(DefaultDeps())
}

// RootWithDeps constructs the root command with the provided dependencies.
// Tests use this to inject fakes for the GraphQL client, env, time source,
// etc.
func RootWithDeps(deps Deps) *cobra.Command {
	root := &cobra.Command{
		Use:           "gh-tasks",
		Short:         "GitHub Projects v2 / Issues / Milestone task management CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}

	// Persistent flags consumed by every subcommand. cobra is the single
	// authoritative source for these values; commands read them via
	// cmd.Flags().GetString. Help text is intentionally plain ASCII so
	// gh tasks check-i18n stays green without needing flag.* keys in the
	// catalog (i18n-ization of help text is tracked as follow-up work).
	root.PersistentFlags().StringP("scope", "s", "", "scope to operate on (repo|org|user)")
	root.PersistentFlags().StringP("repo", "r", "", "repository (<owner>/<name>) override")
	root.PersistentFlags().StringP("project", "p", "", "project (<owner>/<number>) override")
	root.PersistentFlags().String("lang", "", "output locale override (en|ja)")

	root.AddCommand(
		newAddCmd(deps),
		newListCmd(deps),
		newTodayCmd(deps),
		newDoneCmd(deps),
		newStandupCmd(deps),
		newReviewCmd(deps),
		newPlanCmd(deps),
		newTriageCmd(deps),
		newLinkCmd(deps),
		newProjectsCmd(deps),
		newBuildSkillsCmd(deps),
		newCheckI18nCmd(deps),
		newInstallSkillsCmd(deps),
	)

	return root
}
