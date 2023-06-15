package main

import (
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use: "mirage-debug",
	}
	root.AddCommand(configCmd())
	root.AddCommand(debugCmd())
	root.AddCommand(serverCmd())
	if err := root.Execute(); err != nil {
		panic(err)
	}
}
