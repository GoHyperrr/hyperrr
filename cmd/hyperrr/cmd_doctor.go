package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Check system health (Go, DB, modules, server)",
	Long:    `Perform a diagnostic check on the environment, active configuration, database connection, module registry, and server port.`,
	GroupID: "engine",
	RunE: func(cmd *cobra.Command, args []string) error {
		report := DoctorReport{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		}

		// 1. Go version
		goVerCmd := exec.Command("go", "version")
		var out bytes.Buffer
		goVerCmd.Stdout = &out
		if err := goVerCmd.Run(); err == nil {
			report.GoVersion = strings.TrimSpace(out.String())
		} else {
			report.GoVersion = runtime.Version() // Fallback to compiler version
		}

		// 2. Config file
		path := findConfigFile()
		report.ConfigPath = path
		if _, err := os.Stat(path); err == nil {
			report.ConfigFound = true
		}

		// 3. Deps (Config + DB)
		deps, err := buildDeps(true)
		var dbErr error
		if err == nil {
			report.DBConnected = true
			report.DBDriver = deps.Config.DBDriver
			report.DBDSN = deps.Config.DBDSN
			report.ServerPort = deps.Config.ServerPort

			// Count tables
			var tableCount int64
			if deps.Config.DBDriver == "sqlite" {
				deps.DB.Raw("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount)
			} else if deps.Config.DBDriver == "postgres" {
				deps.DB.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tableCount)
			}
			report.DBTables = tableCount
		} else {
			dbErr = err
			// Fallback config load if DB connection failed
			if fallbackCfg, err := buildDeps(false); err == nil {
				report.DBDriver = fallbackCfg.Config.DBDriver
				report.DBDSN = fallbackCfg.Config.DBDSN
				report.ServerPort = fallbackCfg.Config.ServerPort
			}
		}

		// 4. Modules
		modules := registry.List()
		report.Modules = make([]ModuleState, 0, len(modules))
		for _, m := range modules {
			_, hasGraphQL := m.(registry.GraphQLProvider)
			_, hasMCP := m.(registry.ResourceProvider)
			state := ModuleState{
				ID:         m.ID(),
				Models:     len(m.Models()),
				HasGraphQL: hasGraphQL,
				HasMCP:     hasMCP,
			}
			report.Modules = append(report.Modules, state)
		}

		// 5. Server Port Alive
		if report.ServerPort > 0 {
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", report.ServerPort), 300*time.Millisecond)
			if err == nil {
				report.ServerAlive = true
				conn.Close()
			}
		}

		// Print JSON or text
		if jsonOutput {
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		// Print Text Report
		fmt.Println("\n  Hyperrr Engine Health Check")
		fmt.Println("  ═══════════════════════════")
		
		fmt.Println("\n  System")
		fmt.Printf("  ✓ Go version:     %s\n", report.GoVersion)
		fmt.Printf("  ✓ OS/Arch:        %s/%s\n", report.OS, report.Arch)
		if report.ConfigFound {
			fmt.Printf("  ✓ Config file:    %s (found)\n", report.ConfigPath)
		} else {
			fmt.Printf("  ✗ Config file:    %s (NOT found)\n", report.ConfigPath)
		}

		fmt.Println("\n  Database")
		fmt.Printf("  ✓ Driver:         %s\n", report.DBDriver)
		fmt.Printf("  ✓ DSN:            %s\n", report.DBDSN)
		if report.DBConnected {
			fmt.Printf("  ✓ Connection:     OK (%d tables migrated)\n", report.DBTables)
		} else {
			fmt.Printf("  ✗ Connection:     Failed: %v\n", dbErr)
		}

		fmt.Println("\n  Modules")
		fmt.Printf("  ✓ Loaded:         %d modules\n", len(report.Modules))
		for _, m := range report.Modules {
			gqlStr := "✓"
			if !m.HasGraphQL {
				gqlStr = "✗"
			}
			mcpStr := "✓"
			if !m.HasMCP {
				mcpStr = "✗"
			}
			fmt.Printf("    • %-20s (models: %d, graphql: %s, mcp: %s)\n", m.ID, m.Models, gqlStr, mcpStr)
		}

		fmt.Println("\n  Server")
		if report.ServerAlive {
			fmt.Printf("  ✓ Status:         running (127.0.0.1:%d)\n", report.ServerPort)
		} else {
			fmt.Printf("  ✗ Status:         not running (127.0.0.1:%d)\n", report.ServerPort)
		}

		fmt.Println("\n  ──────────────────────────────")
		issues := 0
		if !report.ConfigFound {
			issues++
			fmt.Println("  • Issue: Config file not found. Run 'hyperrr config init' to create one.")
		}
		if !report.DBConnected {
			issues++
			fmt.Println("  • Issue: Database connection failed. Check your config and database server.")
		}
		if !report.ServerAlive {
			issues++
			fmt.Println("  • Issue: Server is not running. Run 'hyperrr server' to start.")
		}

		if issues == 0 {
			fmt.Println("  ✓ All systems nominal! No issues found.")
		} else {
			fmt.Printf("  %d issue(s) found.\n", issues)
		}
		fmt.Println()

		return nil
	},
}

type DoctorReport struct {
	GoVersion   string        `json:"go_version"`
	OS          string        `json:"os"`
	Arch        string        `json:"arch"`
	ConfigPath  string        `json:"config_path"`
	ConfigFound bool          `json:"config_found"`
	DBDriver    string        `json:"db_driver"`
	DBDSN       string        `json:"db_dsn"`
	DBConnected bool          `json:"db_connected"`
	DBTables    int64         `json:"db_tables"`
	Modules     []ModuleState `json:"modules"`
	ServerPort  int           `json:"server_port"`
	ServerAlive bool          `json:"server_alive"`
}

type ModuleState struct {
	ID         string `json:"id"`
	Models     int    `json:"models"`
	HasGraphQL bool   `json:"has_graphql"`
	HasMCP     bool   `json:"has_mcp"`
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
