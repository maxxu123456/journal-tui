package ui

import (
	"fmt"
	"strings"

	"journal/internal/model"
	"journal/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SelectorModel struct {
	journals      []model.JournalDB
	selectedIndex int
	Selected      *model.JournalDB
	CreateNew     bool
	Done          bool
	themeIndex    int
	themes        []string
	ThemeChanged  bool
	NewTheme      string
}

func NewSelectorModel(journals []model.JournalDB, currentTheme string) SelectorModel {
	themes := theme.List()
	themeIndex := 0
	for i, t := range themes {
		if t == currentTheme {
			themeIndex = i
			break
		}
	}

	return SelectorModel{
		journals:      journals,
		selectedIndex: 0, // Most recent is first
		themes:        themes,
		themeIndex:    themeIndex,
		NewTheme:      currentTheme,
	}
}

func (m SelectorModel) Init() tea.Cmd {
	return nil
}

func (m SelectorModel) Update(msg tea.Msg) (SelectorModel, tea.Cmd) {
	// Total options = journals + "Create new journal"
	totalOptions := len(m.journals) + 1

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case "down", "j":
			if m.selectedIndex < totalOptions-1 {
				m.selectedIndex++
			}
		case "left", "h":
			// Change theme
			if m.themeIndex > 0 {
				m.themeIndex--
			} else {
				m.themeIndex = len(m.themes) - 1
			}
			m.NewTheme = m.themes[m.themeIndex]
			theme.Set(m.NewTheme)
			m.ThemeChanged = true
		case "right", "l":
			// Change theme
			if m.themeIndex < len(m.themes)-1 {
				m.themeIndex++
			} else {
				m.themeIndex = 0
			}
			m.NewTheme = m.themes[m.themeIndex]
			theme.Set(m.NewTheme)
			m.ThemeChanged = true
		case "enter":
			if m.selectedIndex < len(m.journals) {
				m.Selected = &m.journals[m.selectedIndex]
			} else {
				m.CreateNew = true
			}
			m.Done = true
		case "q":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m SelectorModel) View() string {
	t := theme.Current()
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Title)
	selectedStyle := lipgloss.NewStyle().Foreground(t.Selected).Bold(true).PaddingLeft(2)
	itemStyle := lipgloss.NewStyle().Foreground(t.Text).PaddingLeft(2)
	pathStyle := lipgloss.NewStyle().Foreground(t.Info).Italic(true)
	mutedStyle := lipgloss.NewStyle().Foreground(t.Muted)
	accentStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted)
	keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	themeStyle := lipgloss.NewStyle().Foreground(t.Success).Bold(true)

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Journal"))
	b.WriteString("\n\n")

	// Theme selector at top
	b.WriteString(mutedStyle.Render("Theme: "))
	b.WriteString(themeStyle.Render(m.themes[m.themeIndex]))
	b.WriteString(mutedStyle.Render("  (use Left/Right to change)"))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render("Select Journal"))
	b.WriteString("\n\n")

	for i, j := range m.journals {
		name := j.Name
		if name == "" {
			name = "Unnamed Journal"
		}

		encrypted := ""
		if j.Encrypted {
			encrypted = mutedStyle.Render(" [encrypted]")
		}

		lastOpened := ""
		if !j.LastOpened.IsZero() {
			lastOpened = mutedStyle.Render(fmt.Sprintf(" (last: %s)", j.LastOpened.Format("2006-01-02")))
		}

		line := name + encrypted + lastOpened

		if i == m.selectedIndex {
			b.WriteString(selectedStyle.Render("> " + line))
		} else {
			b.WriteString(itemStyle.Render("  " + line))
		}
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(pathStyle.Render(j.Path))
		b.WriteString("\n\n")
	}

	// Create new option
	newOption := "Create new journal"
	if m.selectedIndex == len(m.journals) {
		b.WriteString(selectedStyle.Render("> " + accentStyle.Render(newOption)))
	} else {
		b.WriteString(itemStyle.Render("  " + newOption))
	}
	b.WriteString("\n\n")

	b.WriteString(helpStyle.Render(keyStyle.Render("Up/Down") + " navigate | " + keyStyle.Render("Left/Right") + " theme | " + keyStyle.Render("Enter") + " select | " + keyStyle.Render("q") + " quit"))

	return b.String()
}
