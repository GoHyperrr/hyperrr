package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/GoHyperrr/hyperrr/commerce/customer"
	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/commerce/product"
	"github.com/GoHyperrr/hyperrr/internal/app"
	ctxEngine "github.com/GoHyperrr/hyperrr/internal/context"
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

	// Register modules statically in the global registry
	// This enables scanning TUI pages from registry without GORM connections
	registry.Register(product.NewModule())
	registry.Register(customer.NewModule())
	registry.Register(order.NewModule())
	registry.Register(ctxEngine.NewModule())

	deps := &registry.Dependencies{
		Config:    &config.Config{},
		ServerURL: serverURL,
	}

	m := tui.NewModel(context.Background(), deps)
	return teaRun(m)
}

func printUsage() {
	fmt.Print(`Hyperrr Commerce AI CLI

Usage:
  hyperrr <command> [arguments]

Commands:
  admin         Launch the Mission Control TUI dashboard
  server        Start the backend GraphQL commerce server
  help          Display help information

Options:
  -s, --server  Specify the target GraphQL server URL (used with 'admin')
                (e.g., hyperrr admin --server http://localhost:8080)

Environment Variables:
  HYPERRR_SERVER  Fallback target URL for the TUI client dashboard
`)
}
