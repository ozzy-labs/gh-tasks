package main

import (
	"embed"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/cmd"
)

// embeddedSkills bundles the canonical skill SSOT into the released
// binary. Commands that need to read skills without a working tree
// (e.g. `gh tasks install-skills`, run from a consumer repo where the
// gh-tasks source is not present) consume it via Deps.EmbeddedSkills.
// `all:` includes dot-prefixed entries so any future `.metadata` files
// under skills/ are not silently dropped.
//
//go:embed all:skills
var embeddedSkills embed.FS

func main() {
	deps := cmd.DefaultDeps()
	deps.EmbeddedSkills = embeddedSkills
	if err := cmd.RootWithDeps(deps).Execute(); err != nil {
		// Match the legacy TS implementation: arg-validation failures
		// (invalid flags, malformed config, missing required positional
		// args) exit with code 2; other "silent" runtime failures
		// (auth / API / not-found responses) exit with code 1. ErrSilentArgs
		// is checked first because it also satisfies errors.Is(err, ErrSilent).
		if errors.Is(err, cmd.ErrSilentArgs) {
			os.Exit(2)
		}
		if errors.Is(err, cmd.ErrSilent) {
			os.Exit(1)
		}
		cobra.CheckErr(err)
	}
}
