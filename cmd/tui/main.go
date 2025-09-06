package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/ogefest/findex/app"
	"github.com/ogefest/findex/models"
)

func main() {
	// Load configuration
	cfg, err := app.LoadConfig("index_config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	var idxPtrs []*models.IndexConfig
	var allIndexes []string
	activeIndexes := make(map[string]bool)
	for i := range cfg.Indexes {
		idxPtrs = append(idxPtrs, &cfg.Indexes[i])
		allIndexes = append(allIndexes, cfg.Indexes[i].Name)
		activeIndexes[cfg.Indexes[i].Name] = true // All active by default
	}
	sort.Strings(allIndexes)

	searcher, err := app.NewSearcher(idxPtrs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create searcher: %v\n", err)
		os.Exit(1)
	}

	fd := os.Stdout.Fd()
	width, _, err := term.GetSize(fd)
	if err != nil {
		width = 200 // fallback
	}

	// Calculate column widths for results table
	sizeCol := 10
	indexCol := 20
	pathCol := width - sizeCol - indexCol - 4 // leave some margin
	if pathCol < 10 {
		pathCol = 10
	}

	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "Enter search query..."
	ti.Focus()
	ti.Width = 50

	// Initialize minSize and maxSize inputs
	minSizeInput := textinput.New()
	minSizeInput.Placeholder = "Enter min size in bytes..."
	minSizeInput.Width = 30

	maxSizeInput := textinput.New()
	maxSizeInput.Placeholder = "Enter max size in bytes..."
	maxSizeInput.Width = 30

	// Initialize results table
	columns := []table.Column{
		{Title: "Path", Width: pathCol},
		{Title: "Size", Width: sizeCol},
		{Title: "IndexName", Width: indexCol},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Set table styles
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(styles)

	// Initialize config table
	configColumns := []table.Column{
		{Title: "Index", Width: width / 2},
		{Title: "Active", Width: width/2 - 4},
	}
	ct := table.New(
		table.WithColumns(configColumns),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	ct.SetStyles(styles)

	m := model{
		textInput:     ti,
		minSizeInput:  minSizeInput,
		maxSizeInput:  maxSizeInput,
		table:         t,
		configTable:   ct,
		searcher:      searcher,
		allIndexes:    allIndexes,
		activeIndexes: activeIndexes,
		indexConfigs:  idxPtrs,
		mode:          searchMode,
		filter:        app.FileFilter{},
	}

	m.updateConfigTable()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting program: %v\n", err)
		os.Exit(1)
	}

	if m.searcher != nil {
		m.searcher.Close()
	}
}
