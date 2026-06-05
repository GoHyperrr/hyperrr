package main

import (
	"github.com/GoHyperrr/hyperrr/internal/builder"
	"github.com/spf13/cobra"
)

var codegenCmd = &cobra.Command{
	Use:     "codegen",
	Short:   "Run the GraphQL resolver code generator",
	Long:    `Scan modules implementing GraphQLProvider, aggregate their queries, mutations, and fields, and dynamically regenerate the resolver implementations.`,
	GroupID: "engine",
	RunE: func(cmd *cobra.Command, args []string) error {
		return builder.RunCodegen()
	},
}

func init() {
	rootCmd.AddCommand(codegenCmd)
}
