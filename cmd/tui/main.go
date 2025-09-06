package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/ogefest/findex/internal/app"
	"github.com/ogefest/findex/pkg/models"
)

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
	inputStyle = lipgloss.NewStyle().
			Margin(1, 0, 1, 0)
	tableStyle = lipgloss.NewStyle().
			Margin(0, 0, 1, 0)
)

type model struct {
	textInput textinput.Model
	table     table.Model
	searcher  *app.Searcher
	results   []models.FileRecord
	fullPaths []string // Store full paths for each result
	err       error
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	var enter = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "submit/open"),
	)
	var toggleFocus = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "toggle focus"),
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, enter):
			if m.textInput.Focused() {
				query := m.textInput.Value()
				if query != "" {
					results, err := m.searcher.Search(query, nil, 50)
					if err != nil {
						m.err = err
						return m, nil
					}
					m.results = results
					m.updateTable()
					// Switch focus to table after search
					m.textInput.Blur()
					m.table.Focus()
				}
				return m, nil
			} else if m.table.Focused() && len(m.results) > 0 {
				// Open the selected file using full path from fullPaths
				selectedIndex := m.table.Cursor()
				if selectedIndex < len(m.fullPaths) {
					fullPath := m.fullPaths[selectedIndex]
					err := openFile(fullPath)
					if err != nil {
						m.err = err
						return m, nil
					}
				} else {
					m.err = fmt.Errorf("no full path available for selected row")
					return m, nil
				}
				return m, nil
			}
		case key.Matches(msg, toggleFocus):
			if m.textInput.Focused() {
				m.textInput.Blur()
				m.table.Focus()
			} else {
				m.table.Blur()
				m.textInput.Focus()
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			return m, tea.Quit
		}

		if m.textInput.Focused() {
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}
		if m.table.Focused() {
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}
		// If neither is focused, pass to both to catch navigation or typing
		var tiCmd, tCmd tea.Cmd
		m.textInput, tiCmd = m.textInput.Update(msg)
		m.table, tCmd = m.table.Update(msg)
		return m, tea.Batch(tiCmd, tCmd)

	case tea.WindowSizeMsg:
		m.table.SetWidth(msg.Width) // Use full terminal width
		m.table.SetHeight(msg.Height - 8)
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString(inputStyle.Render(m.textInput.View()))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(fmt.Sprintf("Error: %v\n", m.err))
	} else {
		b.WriteString(tableStyle.Render(m.table.View()))
	}

	b.WriteString("\nPress Enter to search (in input) or open file (in table), Tab to toggle focus, Esc to quit.\n")

	return baseStyle.Render(b.String())
}

func (m *model) updateTable() {
	rows := []table.Row{}
	m.fullPaths = make([]string, 0, len(m.results))
	for _, result := range m.results {
		// Format size to human-readable (e.g., KB, MB)
		sizeStr := formatSize(result.Size)
		// Construct full path using Dir and Path
		fullPath := filepath.Join(result.Dir, result.Path)
		m.fullPaths = append(m.fullPaths, fullPath)
		rows = append(rows, table.Row{result.Path, sizeStr, result.IndexName})
	}
	m.table.SetRows(rows)
}

// formatSize converts bytes to a human-readable string
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	if bytes >= GB {
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	} else if bytes >= MB {
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	} else if bytes >= KB {
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	}
	return fmt.Sprintf("%d B", bytes)
}

// openFile opens the file with the default system application
func openFile(filePath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filePath)
	case "darwin":
		cmd = exec.Command("open", filePath)
	default: // linux, bsd, etc.
		cmd = exec.Command("xdg-open", filePath)
	}
	return cmd.Start()
}

func main() {
	// Load configuration
	cfg, err := app.LoadConfig("index_config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	var idxPtrs []*models.IndexConfig
	for i := range cfg.Indexes {
		idxPtrs = append(idxPtrs, &cfg.Indexes[i])
	}

	searcher, err := app.NewSearcher(idxPtrs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create searcher: %v\n", err)
		os.Exit(1)
	}
	defer searcher.Close()

	fd := os.Stdout.Fd()
	width, _, err := term.GetSize(fd)
	if err != nil {
		width = 100 // fallback
	}

	// Oblicz szerokość kolumny Path
	sizeCol := 10
	indexCol := 20
	pathCol := width - sizeCol - indexCol - 4 // zostaw trochę marginesu
	if pathCol < 10 {
		pathCol = 10
	}

	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "Enter search query..."
	ti.Focus()
	ti.Width = 50

	// Initialize table with additional columns
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

	m := model{
		textInput: ti,
		table:     t,
		searcher:  searcher,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting program: %v\n", err)
		os.Exit(1)
	}
}
