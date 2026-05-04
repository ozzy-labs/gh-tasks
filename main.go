package main

import (
	"github.com/spf13/cobra"

	"github.com/ozzy-labs/gh-tasks/cmd"
)

func main() {
	cobra.CheckErr(cmd.Root().Execute())
}
