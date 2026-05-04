package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/cmd"
)

func main() {
	if err := cmd.Root().Execute(); err != nil {
		if errors.Is(err, cmd.ErrSilent) {
			os.Exit(1)
		}
		cobra.CheckErr(err)
	}
}
