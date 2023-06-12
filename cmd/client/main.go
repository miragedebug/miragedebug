package main

import (
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use: "mirage-debug",
	}
	root.AddCommand(initCmd())
	root.AddCommand(debugCmd())
	if err := root.Execute(); err != nil {
		panic(err)
	}
}
