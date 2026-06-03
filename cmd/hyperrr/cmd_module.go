package main

import (
	"github.com/spf13/cobra"
)

var moduleCmd = &cobra.Command{
	Use:     "module",
	Short:   "Module Management",
	Long:    `Manage Hyperrr plugin modules including listing, creating new skeletons, installing remote modules, or uninstalling existing modules.`,
	GroupID: "module",
}

var moduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all loaded plug-in modules",
	Long:  `Scan the workspace and active binary registry, listing all registered and loaded plugin modules.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList()
	},
}

var moduleCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Scaffold a new plugin module project",
	Long:  `Scaffold the directory structure and code skeleton for a new standalone Hyperrr plugin module.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreate([]string{"module", args[0]})
	},
}

var moduleInstallCmd = &cobra.Command{
	Use:   "install <package>",
	Short: "Download a plugin and compile it into the binary",
	Long:  `Download a remote plugin module repository, import it into the local project structure, and trigger a binary rebuild.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall(args)
	},
}

var moduleUninstallCmd = &cobra.Command{
	Use:   "uninstall <package>",
	Short: "Remove a plugin and rebuild",
	Long:  `Remove a plugin module dependency from imports, remove references, and rebuild the server binary.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUninstall(args)
	},
}

func init() {
	moduleCmd.AddCommand(moduleListCmd, moduleCreateCmd, moduleInstallCmd, moduleUninstallCmd)
	rootCmd.AddCommand(moduleCmd)
}
