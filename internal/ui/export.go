package ui

import (
	"strings"

	"journal/internal/model"
	"journal/internal/storage"
	"journal/internal/theme"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ExportModel struct {
	attachment *model.Attachment
	dbPath     string
	encrypted  bool
	password   string
	pathInput  textinput.Model
	Done       bool
	Cancelled  bool
	Error      string
	Message    string
}

func NewExportModel(attachment *model.Attachment, dbPath string, encrypted bool, password string) ExportModel {
	ti := textinput.New()
	ti.Placeholder = "Enter destination path or directory..."
	ti.CharLimit = 512
	ti.Width = 50
	ti.Focus()

	// Default to home directory
	home, _ := storage.ExpandPath("~/")
	if home != "" {
		ti.SetValue(home)
	}

	return ExportModel{
		attachment: attachment,
		dbPath:     dbPath,
		encrypted:  encrypted,
		password:   password,
		pathInput:  ti,
	}
}

func (m ExportModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ExportModel) Update(msg tea.Msg) (ExportModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			destPath := m.pathInput.Value()
			if destPath != "" {
				var err error
				if m.encrypted {
					err = storage.ExportAttachmentEncrypted(m.dbPath, m.password, m.attachment.ID, destPath)
				} else {
					err = storage.ExportAttachment(m.dbPath, m.attachment.ID, destPath)
				}

				if err != nil {
					m.Error = err.Error()
				} else {
					m.Message = "Exported successfully"
					m.Done = true
				}
			}
			return m, nil
		case "esc":
			m.Cancelled = true
			return m, nil
		}
	}

	m.Error = ""
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

func (m ExportModel) View() string {
	t := theme.Current()
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Title)
	labelStyle := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(t.Info)
	sizeStyle := lipgloss.NewStyle().Foreground(t.Muted)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted)
	keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(t.Success).Bold(true)

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Export Attachment"))
	b.WriteString("\n\n")

	if m.attachment != nil {
		b.WriteString(labelStyle.Render("File: "))
		b.WriteString(valueStyle.Render(m.attachment.Filename))
		b.WriteString(" ")
		b.WriteString(sizeStyle.Render("(" + storage.FormatFileSize(m.attachment.Size) + ")"))
		b.WriteString("\n\n")
	}

	b.WriteString(labelStyle.Render("Destination:"))
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(m.pathInput.View())
	b.WriteString("\n\n")

	if m.Error != "" {
		b.WriteString(errorStyle.Render("Error: " + m.Error))
		b.WriteString("\n\n")
	}

	if m.Message != "" {
		b.WriteString(successStyle.Render(m.Message))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render(keyStyle.Render("Enter") + " export | " + keyStyle.Render("Esc") + " cancel"))

	return b.String()
}
