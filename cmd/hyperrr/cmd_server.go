package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/GoHyperrr/hyperrr/internal/app"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:     "server",
	Aliases: []string{"start"},
	Short:   "Start the GraphQL + MCP server",
	Long:    `Start the core Hyperrr commerce backend server including the GraphQL API and Model Context Protocol (MCP) server gateways.`,
	GroupID: "engine",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Detect if we are inside a project
		root, err := findProjectRoot()
		if err == nil {
			// We are inside a project! Check if the project-local binary exists.
			binaryName := "hyperrr"
			if os.PathSeparator == '\\' {
				binaryName += ".exe"
			}
			localBinaryPath := filepath.Join(root, "bin", binaryName)
			
			// If missing, trigger a build first
			if _, err := os.Stat(localBinaryPath); os.IsNotExist(err) {
				fmt.Println("Project-local binary not found. Running initial build...")
				buildCmd := exec.Command("go", "run", "github.com/GoHyperrr/hyperrr/cmd/hyperrr@latest", "build")
				buildCmd.Dir = root
				buildCmd.Stdout = os.Stdout
				buildCmd.Stderr = os.Stderr
				if err := buildCmd.Run(); err != nil {
					return fmt.Errorf("initial build failed: %w", err)
				}
			}
			
			// Exec the local binary with the same arguments
			fmt.Printf("Starting project-local server: %s\n", localBinaryPath)
			subArgs := append([]string{"server"}, args...)
			// If config flag was set, pass it down
			if cfgFile != "" {
				subArgs = append(subArgs, "--config", cfgFile)
			}
			if verbose {
				subArgs = append(subArgs, "--verbose")
			}
			
			runCmd := exec.Command(localBinaryPath, subArgs...)
			runCmd.Dir = root
			runCmd.Stdout = os.Stdout
			runCmd.Stderr = os.Stderr
			runCmd.Stdin = os.Stdin
			
			return runCmd.Run()
		}

		// Fallback: Standalone run (for testing/development within core repository)
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
