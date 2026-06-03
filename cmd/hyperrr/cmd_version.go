package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Print version and build info",
	Long:    `Print the current version, environment info, and build configuration of the Hyperrr command line engine.`,
	GroupID: "utils",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Hyperrr Engine SDK v0.1.0")
		fmt.Println("AI-Native Commerce Gateway")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
