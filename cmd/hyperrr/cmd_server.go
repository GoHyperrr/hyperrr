package main

import (
	"github.com/spf13/cobra"
	"github.com/GoHyperrr/hyperrr/internal/app"
)

var serverCmd = &cobra.Command{
	Use:     "server",
	Aliases: []string{"start"},
	Short:   "Start the GraphQL + MCP server",
	Long:    `Start the core Hyperrr commerce backend server including the GraphQL API and Model Context Protocol (MCP) server gateways.`,
	GroupID: "engine",
	RunE: func(cmd *cobra.Command, args []string) error {
		deps, err := buildDeps(false)
		if err != nil {
			return err
		}
		return app.RunWithConfig(deps.Config)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
