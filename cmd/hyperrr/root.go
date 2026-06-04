package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"gorm.io/gorm"
)

var rootCmd = &cobra.Command{
	Use:   "hyperrr",
	Short: "hyperrr — AI-Native Commerce Engine SDK",
	Long: `Hyperrr is an AI-native commerce engine designed for agentic workflows.
It provides high-performance commerce modules, GraphQL API, and Model Context Protocol (MCP) servers.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	// Set up global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Config file (default \"./hyperrr.yml\")")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose/debug logging")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format (where supported)")

	// Define built-in command groups
	rootCmd.AddGroup(
		&cobra.Group{ID: "engine", Title: "Engine Commands:"},
		&cobra.Group{ID: "module", Title: "Module Management Commands:"},
		&cobra.Group{ID: "config", Title: "Configuration Commands:"},
		&cobra.Group{ID: "utils", Title: "Utility Commands:"},
	)
}

func Execute() {
	registerPluginCommands()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var groupCmds = make(map[string]*cobra.Command)

func findOrCreateGroupCmd(groupName string) *cobra.Command {
	if cmd, ok := groupCmds[groupName]; ok {
		return cmd
	}

	title := groupName
	if len(title) > 0 {
		title = strings.ToUpper(title[:1]) + title[1:]
	}

	// Define a custom group for this parent command to list it separately in help
	groupID := "dyn-" + groupName
	rootCmd.AddGroup(&cobra.Group{ID: groupID, Title: fmt.Sprintf("%s Commands:", title)})

	cmd := &cobra.Command{
		Use:     groupName,
		Short:   fmt.Sprintf("Commands registered by %s modules", groupName),
		GroupID: groupID,
	}

	rootCmd.AddCommand(cmd)
	groupCmds[groupName] = cmd
	return cmd
}

func registerPluginCommands() {
	for _, c := range registry.ListCommands() {
		var parent *cobra.Command
		if c.Group != "" {
			parent = findOrCreateGroupCmd(c.Group)
		} else {
			parent = rootCmd
		}

		cobraCmd := &cobra.Command{
			Use:     c.Name,
			Aliases: c.Aliases,
			Short:   c.Short,
			Long:    c.Long,
			RunE: func(cmd *cobra.Command, args []string) error {
				deps, err := buildDeps(c.NeedsDB)
				if err != nil {
					return err
				}
				var gormDB *gorm.DB
				if deps.DB != nil {
					gormDB = deps.DB.DB
				}
				rt := &runtimeImpl{
					db:  gormDB,
					cfg: deps.Config,
				}
				return c.Run(rt, args)
			},
		}

		parent.AddCommand(cobraCmd)
	}
}
