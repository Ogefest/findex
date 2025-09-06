package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ogefest/findex/internal/app"
	"github.com/ogefest/findex/pkg/models"
)

type model struct {
	textInput *textinput.Model
	searcher  *app.Searcher
	results   []models.FileRecord
	done      bool
	filter    *app.FileFilter
}

func main() {
	// 1. Wczytujemy konfigurację indeksów
	cfg, err := app.LoadConfig("index_config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	var idxPtrs []*models.IndexConfig
	for i := range cfg.Indexes {
		idxPtrs = append(idxPtrs, &cfg.Indexes[i])
	}

	// 2. Tworzymy searchera i otwieramy połączenia
	searcher, err := app.NewSearcher(idxPtrs)
	if err != nil {
		log.Fatalf("Failed to initialize searcher: %v", err)
	}
	defer searcher.Close()

	// 3. Konfigurujemy input TUI
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.Focus()

	m := model{
		textInput: &ti,
		searcher:  searcher,
		filter:    &app.FileFilter{}, // na start brak dodatkowych filtrów
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if err := p.Start(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
	}
}

// --- Bubble Tea Init ---
func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// --- Bubble Tea Update ---
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit
		}
	}

	*m.textInput, cmd = m.textInput.Update(msg)

	// 4. Dynamiczne wyszukiwanie w istniejących bazach
	query := m.textInput.Value()
	if query != "" {
		results, err := m.searcher.Search(query, m.filter, 50)
		if err == nil {
			m.results = results
		}
	} else {
		m.results = nil
	}

	return m, cmd
}

// --- Bubble Tea View ---
func (m model) View() string {
	if m.done {
		// przy wyjściu wypisujemy wszystkie znalezione ścieżki
		var out []string
		for _, f := range m.results {
			out = append(out, f.Path)
		}
		return strings.Join(out, "\n")
	}

	s := "Search index (type to filter, Esc/Ctrl+C to exit):\n\n"
	s += m.textInput.View() + "\n\n"

	// tabela wyników
	s += fmt.Sprintf("%-50s %-20s %-10s %-20s\n", "Path", "Name", "Size", "ModTime")
	s += strings.Repeat("-", 110) + "\n"

	max := 20
	for i, f := range m.results {
		if i >= max {
			s += fmt.Sprintf("...and %d more\n", len(m.results)-max)
			break
		}
		s += fmt.Sprintf("%-50s %-20s %-10d %-20s\n",
			f.Path,
			f.Name,
			f.Size,
			f.ModTime.Format("2006-01-02 15:04:05"))
	}

	return s
}
