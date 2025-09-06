package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ogefest/findex/app"
	"github.com/ogefest/findex/models"
)

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
	inputStyle = lipgloss.NewStyle().
			Margin(1, 0, 0, 0)
	tableStyle = lipgloss.NewStyle().
			Margin(0, 0, 1, 0)
	indexListStyle = lipgloss.NewStyle().
			Margin(0, 0, 1, 0)
	buttonStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1).
			Margin(1, 1, 0, 1)
)

const (
	searchMode = "search"
	configMode = "config"
)

type model struct {
	textInput     textinput.Model
	minSizeInput  textinput.Model
	maxSizeInput  textinput.Model
	table         table.Model
	configTable   table.Model
	searcher      *app.Searcher
	results       []models.FileRecord
	fullPaths     []string
	allIndexes    []string
	activeIndexes map[string]bool
	indexConfigs  []*models.IndexConfig
	mode          string
	err           error
	filter        app.FileFilter
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) updateSearcher() error {
	if m.searcher != nil {
		m.searcher.Close()
	}

	var activeIdxPtrs []*models.IndexConfig
	for _, cfg := range m.indexConfigs {
		if m.activeIndexes[cfg.Name] {
			activeIdxPtrs = append(activeIdxPtrs, cfg)
		}
	}

	if len(activeIdxPtrs) == 0 {
		m.searcher = nil
		return nil
	}

	searcher, err := app.NewSearcher(activeIdxPtrs)
	if err != nil {
		return err
	}
	m.searcher = searcher
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	var enter = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("⏎", "submit/open/toggle"),
	)
	var toggleFocus = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "toggle focus"),
	)
	var configKey = key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "configure indexes"),
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, configKey):
			if m.mode == searchMode {
				m.mode = configMode
				m.textInput.Blur()
				m.table.Blur()
				m.minSizeInput.Focus()
			} else if m.mode == configMode {
				m.mode = searchMode
				m.configTable.Blur()
				m.minSizeInput.Blur()
				m.maxSizeInput.Blur()
				m.textInput.Focus()
			}
			m.updateConfigTable()
			return m, nil
		case key.Matches(msg, enter):
			if m.mode == searchMode {
				if m.textInput.Focused() {
					query := m.textInput.Value()
					if query != "" && m.searcher != nil {
						results, err := m.searcher.Search(query, &m.filter, 200)
						if err != nil {
							m.err = err
							return m, nil
						}
						m.results = results
						m.updateTable()
						m.textInput.Blur()
						m.table.Focus()
					} else if m.searcher == nil {
						m.err = fmt.Errorf("no active indexes selected")
						return m, nil
					}
					return m, nil
				} else if m.table.Focused() && len(m.results) > 0 {
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
			} else if m.mode == configMode {
				if m.minSizeInput.Focused() {
					// Move focus to maxSizeInput
					m.minSizeInput.Blur()
					m.maxSizeInput.Focus()
					return m, nil
				} else if m.maxSizeInput.Focused() {
					// Parse inputs and update filter
					minSizeStr := m.minSizeInput.Value()
					maxSizeStr := m.maxSizeInput.Value()
					if minSizeStr != "" {
						if minSize, err := strconv.ParseInt(minSizeStr, 10, 64); err == nil {
							m.filter.MinSize = minSize
						}
					} else {
						m.filter.MinSize = 0
					}
					if maxSizeStr != "" {
						if maxSize, err := strconv.ParseInt(maxSizeStr, 10, 64); err == nil {
							m.filter.MaxSize = maxSize
						}
					} else {
						m.filter.MaxSize = 0
					}
					m.maxSizeInput.Blur()
					m.configTable.Focus()
					return m, nil
				} else if m.configTable.Focused() {
					// Toggle active status for selected index
					selectedIndex := m.configTable.Cursor()
					if selectedIndex < len(m.allIndexes) {
						indexName := m.allIndexes[selectedIndex]
						m.activeIndexes[indexName] = !m.activeIndexes[indexName]
						if err := m.updateSearcher(); err != nil {
							m.err = err
							return m, nil
						}
						m.updateConfigTable()
					}
					return m, nil
				}
			}
		case key.Matches(msg, toggleFocus):
			if m.mode == searchMode {
				if m.textInput.Focused() {
					m.textInput.Blur()
					m.table.Focus()
				} else {
					m.table.Blur()
					m.textInput.Focus()
				}
			} else if m.mode == configMode {
				if m.minSizeInput.Focused() {
					m.minSizeInput.Blur()
					m.maxSizeInput.Focus()
				} else if m.maxSizeInput.Focused() {
					m.maxSizeInput.Blur()
					m.configTable.Focus()
				} else {
					m.configTable.Blur()
					m.minSizeInput.Focus()
				}
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.mode == configMode {
				m.mode = searchMode
				m.configTable.Blur()
				m.minSizeInput.Blur()
				m.maxSizeInput.Blur()
				m.textInput.Focus()
				return m, nil
			}
			return m, tea.Quit
		}

		if m.mode == searchMode {
			if m.textInput.Focused() {
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
			if m.table.Focused() {
				m.table, cmd = m.table.Update(msg)
				return m, cmd
			}
			var tiCmd, tCmd tea.Cmd
			m.textInput, tiCmd = m.textInput.Update(msg)
			m.table, tCmd = m.table.Update(msg)
			return m, tea.Batch(tiCmd, tCmd)
		} else if m.mode == configMode {
			if m.minSizeInput.Focused() {
				m.minSizeInput, cmd = m.minSizeInput.Update(msg)
				return m, cmd
			}
			if m.maxSizeInput.Focused() {
				m.maxSizeInput, cmd = m.maxSizeInput.Update(msg)
				return m, cmd
			}
			if m.configTable.Focused() {
				m.configTable, cmd = m.configTable.Update(msg)
				return m, cmd
			}
		}

	case tea.WindowSizeMsg:
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(msg.Height - 11)
		m.configTable.SetWidth(msg.Width)
		m.configTable.SetHeight(msg.Height - 8) // Adjusted for additional inputs
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	// Instructions for key bindings
	instructions := "Press Enter to search (in input) or open file (in table), Tab to toggle focus, Ctrl+P to configure indexes, Esc to quit."
	if m.mode == configMode {
		instructions = "Press Enter to set size or toggle index, Tab to toggle focus, Ctrl+P or Esc to return to search."
	}

	if m.mode == searchMode {
		// Render text input only
		inputView := inputStyle.Width(m.table.Width() - 4).Render(m.textInput.View())
		b.WriteString(inputView)
		b.WriteString("\n")

		// Render active indexes horizontally as filters
		var filtersView strings.Builder
		filtersView.WriteString("Selected filters: ")
		for i, index := range m.getActiveIndexes() {
			color := generateColorForIndex(index)
			indexStyle := lipgloss.NewStyle().
				Background(lipgloss.Color(color)).
				Padding(0, 1).
				Foreground(lipgloss.Color("229"))
			filtersView.WriteString(indexStyle.Render(index))
			if i < len(m.getActiveIndexes())-1 {
				filtersView.WriteString(" ")
			}
		}
		b.WriteString(indexListStyle.Width(m.table.Width() - 2).Render(filtersView.String()))
		b.WriteString("\n")

		if m.err != nil {
			b.WriteString(fmt.Sprintf("Error: %v\n", m.err))
		} else {
			b.WriteString(tableStyle.Render(m.table.View()))
		}
	} else if m.mode == configMode {
		b.WriteString("Configure Search Filters\n")
		minSizeView := inputStyle.Width(m.configTable.Width()/2 - 4).Render("Min Size (bytes): " + m.minSizeInput.View())
		maxSizeView := inputStyle.Width(m.configTable.Width()/2 - 4).Render("Max Size (bytes): " + m.maxSizeInput.View())
		sizeInputs := lipgloss.JoinHorizontal(lipgloss.Top, minSizeView, maxSizeView)
		b.WriteString(sizeInputs)
		b.WriteString("\n")
		b.WriteString("Indexes\n")
		b.WriteString(indexListStyle.Render(m.configTable.View()))
	}

	// Wrap content with instructions below the border
	return baseStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			b.String(),
			lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(instructions),
		),
	)
}

func (m *model) updateTable() {
	rows := []table.Row{}
	m.fullPaths = make([]string, 0, len(m.results))
	for _, result := range m.results {
		sizeStr := formatSize(result.Size)
		fullPath := filepath.Join(result.Dir, result.Path)
		m.fullPaths = append(m.fullPaths, fullPath)
		rows = append(rows, table.Row{result.Path, sizeStr, result.IndexName})
	}
	m.table.SetRows(rows)
}

func (m *model) updateConfigTable() {
	rows := []table.Row{}
	for _, index := range m.allIndexes {
		active := "No"
		if m.activeIndexes[index] {
			active = "Yes"
		}
		rows = append(rows, table.Row{index, active})
	}
	m.configTable.SetRows(rows)
}

func (m *model) getActiveIndexes() []string {
	active := []string{}
	for _, index := range m.allIndexes {
		if m.activeIndexes[index] {
			active = append(active, index)
		}
	}
	sort.Strings(active)
	return active
}
