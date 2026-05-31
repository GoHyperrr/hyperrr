package main

import (
	"context"
	"fmt"
	"os"

	"github.com/GoHyperrr/hyperrr/commerce/customer"
	"github.com/GoHyperrr/hyperrr/commerce/order"
	"github.com/GoHyperrr/hyperrr/commerce/product"
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

var osExit = os.Exit

func main() {
	if err := run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v\n", err)
		osExit(1)
	}
}

func run() error {
	// 1. Detect server URL from flag or environment variable
	serverURL := os.Getenv("HYPERRR_SERVER")
	for i, arg := range os.Args {
		if (arg == "--server" || arg == "-s") && i+1 < len(os.Args) {
			serverURL = os.Args[i+1]
		}
	}
	if serverURL == "" {
		serverURL = "http://localhost:8080" // Default fallback URL
	}

	// 2. Register modules statically in the global registry
	// This enables scanning TUI pages from registry without GORM connections
	registry.Register(product.NewModule())
	registry.Register(customer.NewModule())
	registry.Register(order.NewModule())
	registry.Register(ctxEngine.NewModule())

	// 3. Setup stateless dependencies adapter
	deps := &registry.Dependencies{
		Config:    &config.Config{},
		ServerURL: serverURL,
	}

	// 4. Create and run the master TUI
	m := tui.NewModel(context.Background(), deps)
	return teaRun(m)
}
