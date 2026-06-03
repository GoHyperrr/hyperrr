package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:     "new <project-name>",
	Short:   "Scaffold a new Hyperrr project from template",
	Long:    `Scaffold a brand new Hyperrr commerce application project including workspace configurations, default directories, configuration skeleton, and environment file templates.`,
	Args:    cobra.ExactArgs(1),
	GroupID: "engine",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Scaffolding new project '%s'...\n", args[0])
		fmt.Println("🚀 Scaffold feature is coming soon!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}
