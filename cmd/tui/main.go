package main

import (
	"context"
	"fmt"
	"os"

	"github.com/GoHyperrr/hyperrr/internal/tui"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	tea "github.com/charmbracelet/bubbletea"
)

var teaRun = func(m tea.Model) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

var osExit = os.Exit

func main() {
	if err := run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		osExit(1)
	}
}

func run() error {
	bus := eventbus.NewInMemBus()
	m := tui.NewModel(context.Background(), bus)
	return teaRun(m)
}
