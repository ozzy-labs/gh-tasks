package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/cmd"
)

func main() {
	if err := cmd.Root().Execute(); err != nil {
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
