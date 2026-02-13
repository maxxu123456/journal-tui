package ui

import (
	"strings"

	"journal/internal/theme"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PasswordModel struct {
	passwordInput textinput.Model
	Password      string
	Done          bool
	Cancelled     bool
	Error         string
}

func NewPasswordModel() PasswordModel {
	ti := textinput.New()
	ti.Placeholder = "Enter password"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.CharLimit = 256
	ti.Width = 30
	ti.Focus()

	return PasswordModel{
		passwordInput: ti,
	}
}

func (m PasswordModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m PasswordModel) Update(msg tea.Msg) (PasswordModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.passwordInput.Value() != "" {
				m.Password = m.passwordInput.Value()
				m.Done = true
			}
			return m, nil
		case "esc":
			m.Cancelled = true
			return m, nil
		}
	}

	m.Error = ""
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m PasswordModel) View() string {
	t := theme.Current()
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Title)
	promptStyle := lipgloss.NewStyle().Foreground(t.Text)
	errorStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted)
	keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Journal - Encrypted"))
	b.WriteString("\n\n")

	b.WriteString(promptStyle.Render("Enter your password to unlock:"))
	b.WriteString("\n\n")

	b.WriteString("  ")
	b.WriteString(m.passwordInput.View())
	b.WriteString("\n")

	if m.Error != "" {
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(errorStyle.Render(m.Error))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render(keyStyle.Render("Enter") + " unlock | " + keyStyle.Render("Esc") + " back"))

	return b.String()
}
