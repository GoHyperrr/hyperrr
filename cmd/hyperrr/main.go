package main

import (
	"context"
	"fmt"
	"log"
	"os"

	_ "github.com/GoHyperrr/commerce/customer"
	_ "github.com/GoHyperrr/commerce/order"
	_ "github.com/GoHyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/internal/app"
	_ "github.com/GoHyperrr/hyperrr/pkg/ctxengine"
	"github.com/GoHyperrr/hyperrr/internal/tui"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	tea "charm.land/bubbletea/v2"
)

var teaRun = func(m tea.Model) error {
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}

var osExit = func(err error) {
	log.Fatal(err)
}

var appRun = app.Run

func main() {
	if err := run(); err != nil {
		osExit(err)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return nil
	}

	cmd := args[0]
	switch cmd {
	case "admin", "tui", "dashboard":
		return runTUI(args[1:])
	case "server", "start":
		return appRun()
	case "list":
		return runList()
	case "create":
		return runCreate(args[1:])
	case "install":
		return runInstall(args[1:])
	case "uninstall":
		return runUninstall(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		printUsage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func runTUI(args []string) error {
	serverURL := os.Getenv("HYPERRR_SERVER")
	for i, arg := range args {
		if (arg == "--server" || arg == "-s") && i+1 < len(args) {
			serverURL = args[i+1]
		}
	}
	if serverURL == "" {
		serverURL = "http://localhost:8080" // Default fallback URL
	}

	deps := &registry.Dependencies{
		Config:    &config.Config{},
		ServerURL: serverURL,
	}

	m := tui.NewModel(context.Background(), deps)
	return teaRun(m)
}

func printUsage() {
	fmt.Print(`Hyperrr Core Engine & AI Gateway CLI

Usage:
  hyperrr <command> [arguments]

Commands:
  admin                  Launch the Mission Control TUI dashboard
  server                 Start the backend GraphQL commerce server
  list                   List all loaded plug-in modules in the binary
  create module <name>   Scaffold a new standalone plugin project
  install <package>      Download a plugin and compile it into the binary
  uninstall <package>    Remove a plugin and rebuild the binary
  help                   Display help information

Options:
  -s, --server  Specify the target GraphQL server URL (used with 'admin')
                (e.g., hyperrr admin --server http://localhost:8080)

Environment Variables:
  HYPERRR_SERVER  Fallback target URL for the TUI client dashboard
`)
}
