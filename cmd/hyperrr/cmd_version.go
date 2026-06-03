package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/GoHyperrr/hyperrr"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Print version and build info",
	Long:    `Print the current version, environment info, and build configuration of the Hyperrr command line engine.`,
	GroupID: "utils",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Hyperrr Engine SDK v%s\n", hyperrr.Version)
		fmt.Println("AI-Native Commerce Gateway")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
