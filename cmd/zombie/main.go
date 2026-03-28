package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jpastorm/zombie/internal/executor"
	"github.com/jpastorm/zombie/internal/scanner"
	"github.com/jpastorm/zombie/internal/tui"
)

var (
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B0000")).Bold(true)
	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7FFF00"))
	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#696969"))
)

func main() {
	// Check xh is installed
	version, err := executor.CheckXh()
	if err != nil {
		fmt.Println(errStyle.Render("\n  ☠  ZOMBIE CANNOT RISE  ☠\n"))
		fmt.Println(errStyle.Render("  " + err.Error()))
		fmt.Println()
		os.Exit(1)
	}

	// Determine base directory (current directory or argument)
	baseDir, err := os.Getwd()
	if err != nil {
		fmt.Println(errStyle.Render("cannot determine working directory: " + err.Error()))
		os.Exit(1)
	}
	if len(os.Args) > 1 {
		baseDir = os.Args[1]
		if !filepath.IsAbs(baseDir) {
			cwd, _ := os.Getwd()
			baseDir = filepath.Join(cwd, baseDir)
		}
	}

	// Scan for request files
	requestsDir := filepath.Join(baseDir, "requests")
	requests, err := scanner.Scan(requestsDir)
	if err != nil {
		fmt.Println(errStyle.Render("cannot scan requests: " + err.Error()))
		os.Exit(1)
	}

	// Show startup info
	fmt.Println(infoStyle.Render("  🧟 zombie"))
	fmt.Println(dimStyle.Render(fmt.Sprintf("  xh: %s", version)))
	fmt.Println(dimStyle.Render(fmt.Sprintf("  dir: %s", baseDir)))
	fmt.Println(dimStyle.Render(fmt.Sprintf("  requests found: %d", len(requests))))
	fmt.Println()

	// Launch TUI
	model := tui.New(baseDir, requests)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println(errStyle.Render("zombie crashed: " + err.Error()))
		os.Exit(1)
	}
}
