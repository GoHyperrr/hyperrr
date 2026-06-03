package main

import (
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:     "build",
	Short:   "Aggregate schemas, codegen, and compile binary",
	Long:    `Aggregate GraphQL schemas across all workspace modules, run the code generator, and build the final server binary.`,
	GroupID: "engine",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuild()
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
