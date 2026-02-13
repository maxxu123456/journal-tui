package ui

import (
	"strings"

	"journal/internal/model"
	"journal/internal/theme"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsField int

const (
	settingsFieldPath settingsField = iota
	settingsFieldMigrate
)

type SettingsModel struct {
	config        *model.Config
	activeJournal *model.JournalDB
	pathInput     textinput.Model
	focusedField  settingsField
	Migrate       bool
	DBPath        string
	Saved         bool
	Cancelled     bool
}

func NewSettingsModel(config *model.Config, activeJournal *model.JournalDB) SettingsModel {
	ti := textinput.New()
	ti.SetValue(config.ActiveJournal)
	ti.CharLimit = 256
	ti.Width = 50
	ti.Focus()

	return SettingsModel{
		config:        config,
		activeJournal: activeJournal,
		pathInput:     ti,
		focusedField:  settingsFieldPath,
		Migrate:       true,
		DBPath:        config.ActiveJournal,
	}
}

func (m SettingsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			if m.focusedField == settingsFieldPath {
				m.focusedField = settingsFieldMigrate
				m.pathInput.Blur()
			} else {
				m.focusedField = settingsFieldPath
				m.pathInput.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case "enter", " ":
			if m.focusedField == settingsFieldMigrate {
				m.Migrate = !m.Migrate
				return m, nil
			}

		case "esc":
			m.Cancelled = true
			return m, nil

		case "ctrl+s":
			m.DBPath = m.pathInput.Value()
			m.Saved = true
			return m, nil
		}
	}

	if m.focusedField == settingsFieldPath {
		m.pathInput, cmd = m.pathInput.Update(msg)
	}

	return m, cmd
}

func (m SettingsModel) View() string {
	t := theme.Current()
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Title)
	labelStyle := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	labelActiveStyle := lipgloss.NewStyle().Foreground(t.Selected).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(t.Info)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted)
	keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	checkboxStyle := lipgloss.NewStyle().Foreground(t.Text).PaddingLeft(2)
	checkboxSelectedStyle := lipgloss.NewStyle().Foreground(t.Selected).Bold(true).PaddingLeft(2)
	checkmarkStyle := lipgloss.NewStyle().Foreground(t.Success).Bold(true)
	dividerStyle := lipgloss.NewStyle().Foreground(t.Muted)
	mutedStyle := lipgloss.NewStyle().Foreground(t.Muted)

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Journal Settings"))
	b.WriteString("\n\n")

	// Journal info
	if m.activeJournal != nil {
		b.WriteString(labelStyle.Render("Journal: "))
		b.WriteString(valueStyle.Render(m.activeJournal.Name))
		if m.activeJournal.Encrypted {
			b.WriteString(mutedStyle.Render(" [encrypted]"))
		}
		b.WriteString("\n\n")
	}

	b.WriteString(dividerStyle.Render(strings.Repeat("-", 60)))
	b.WriteString("\n\n")

	// Path input
	b.WriteString(labelStyle.Render("Current database path:"))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(valueStyle.Render(m.config.ActiveJournal))
	b.WriteString("\n\n")

	pathLabel := "New path:"
	if m.focusedField == settingsFieldPath {
		b.WriteString(labelActiveStyle.Render("> " + pathLabel))
	} else {
		b.WriteString(labelStyle.Render("  " + pathLabel))
	}
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(m.pathInput.View())
	b.WriteString("\n\n")

	// Migrate checkbox
	checkbox := "[ ]"
	if m.Migrate {
		checkbox = "[" + checkmarkStyle.Render("x") + "]"
	}
	migrateLabel := checkbox + " Migrate existing data to new location"
	if m.focusedField == settingsFieldMigrate {
		b.WriteString(checkboxSelectedStyle.Render("> " + migrateLabel))
	} else {
		b.WriteString(checkboxStyle.Render("  " + migrateLabel))
	}
	b.WriteString("\n\n")

	var parts []string
	parts = append(parts, keyStyle.Render("Tab")+" switch fields")
	parts = append(parts, keyStyle.Render("Space/Enter")+" toggle")
	parts = append(parts, keyStyle.Render("Ctrl+S")+" save")
	parts = append(parts, keyStyle.Render("Esc")+" cancel")

	b.WriteString(helpStyle.Render(strings.Join(parts, " | ")))

	return b.String()
}
