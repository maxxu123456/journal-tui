package ui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"journal/internal/model"
	"journal/internal/storage"
	"journal/internal/theme"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

type AttachmentModel struct {
	entry          *model.Entry
	dbPath         string
	encrypted      bool
	password       string
	selectedIndex  int
	Back           bool
	ExportSelected bool
	addMode        bool
	pathInput      textinput.Model
	Error          string
	Message        string
	width          int
	height         int
	HistoryAdded   bool // Flag to indicate history was modified
}

func NewAttachmentModel(entry *model.Entry, dbPath string, encrypted bool, password string) AttachmentModel {
	ti := textinput.New()
	ti.Placeholder = "Enter file path to attach..."
	ti.CharLimit = 512
	ti.Width = 50

	return AttachmentModel{
		entry:         entry,
		dbPath:        dbPath,
		encrypted:     encrypted,
		password:      password,
		selectedIndex: 0,
		pathInput:     ti,
	}
}

func (m *AttachmentModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m AttachmentModel) Init() tea.Cmd {
	return nil
}

func (m AttachmentModel) SelectedAttachment() *model.Attachment {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.entry.Attachments) {
		return &m.entry.Attachments[m.selectedIndex]
	}
	return nil
}

func (m AttachmentModel) Update(msg tea.Msg) (AttachmentModel, tea.Cmd) {
	var cmd tea.Cmd

	if m.addMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				path := m.pathInput.Value()
				if path != "" {
					err := m.addAttachment(path)
					if err != nil {
						m.Error = err.Error()
					} else {
						m.Message = "Attachment added successfully"
						m.addMode = false
						m.pathInput.SetValue("")
						m.pathInput.Blur()
					}
				}
				return m, nil
			case "esc":
				m.addMode = false
				m.pathInput.SetValue("")
				m.pathInput.Blur()
				return m, nil
			}
		}
		m.Error = ""
		m.pathInput, cmd = m.pathInput.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.Error = ""
		m.Message = ""

		switch msg.String() {
		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case "down", "j":
			if m.selectedIndex < len(m.entry.Attachments)-1 {
				m.selectedIndex++
			}
		case "a":
			m.addMode = true
			m.pathInput.Focus()
			return m, textinput.Blink
		case "e":
			if len(m.entry.Attachments) > 0 {
				m.ExportSelected = true
			}
		case "d":
			if len(m.entry.Attachments) > 0 && m.selectedIndex < len(m.entry.Attachments) {
				err := m.deleteAttachment()
				if err != nil {
					m.Error = err.Error()
				} else {
					m.Message = "Attachment deleted"
					if m.selectedIndex >= len(m.entry.Attachments) && m.selectedIndex > 0 {
						m.selectedIndex--
					}
				}
			}
		case "esc", "q":
			m.Back = true
		}
	}

	return m, nil
}

func (m *AttachmentModel) addAttachment(path string) error {
	expandedPath, err := storage.ExpandPath(path)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return err
	}

	filename := filepath.Base(expandedPath)
	mimeType := storage.DetectMimeType(filename)
	now := time.Now()

	// Create a history record capturing the current state BEFORE adding the attachment
	historyRecord := model.SaveRecord{
		Content:     m.entry.Content,
		SavedAt:     now,
		Attachments: m.entry.AttachmentFilenames(),
	}
	m.entry.History = append(m.entry.History, historyRecord)
	m.entry.UpdatedAt = now
	m.HistoryAdded = true

	attachment := &model.Attachment{
		ID:        uuid.New().String(),
		EntryID:   m.entry.ID,
		Filename:  filename,
		MimeType:  mimeType,
		Size:      int64(len(data)),
		Data:      data,
		CreatedAt: now,
	}

	if m.encrypted {
		err = storage.AddAttachmentEncrypted(m.dbPath, m.password, attachment)
	} else {
		err = storage.AddAttachment(m.dbPath, attachment)
	}

	if err != nil {
		// Rollback history addition on error
		m.entry.History = m.entry.History[:len(m.entry.History)-1]
		m.HistoryAdded = false
		return err
	}

	// Update local entry
	attachment.Data = nil // Don't keep data in memory
	m.entry.Attachments = append(m.entry.Attachments, *attachment)

	// Save the history record to the database
	if m.encrypted {
		err = storage.AddHistoryRecord(m.dbPath, m.entry.ID, historyRecord, m.password)
	} else {
		err = storage.AddHistoryRecord(m.dbPath, m.entry.ID, historyRecord, "")
	}

	return err
}

func (m *AttachmentModel) deleteAttachment() error {
	if m.selectedIndex >= len(m.entry.Attachments) {
		return nil
	}

	att := m.entry.Attachments[m.selectedIndex]

	var err error
	if m.encrypted {
		err = storage.DeleteAttachmentEncrypted(m.dbPath, m.password, att.ID)
	} else {
		err = storage.DeleteAttachment(m.dbPath, att.ID)
	}

	if err != nil {
		return err
	}

	// Remove from local entry
	m.entry.Attachments = append(
		m.entry.Attachments[:m.selectedIndex],
		m.entry.Attachments[m.selectedIndex+1:]...,
	)

	return nil
}

func (m AttachmentModel) View() string {
	t := theme.Current()
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Title)
	dateStyle := lipgloss.NewStyle().Foreground(t.Info).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(t.Selected).Bold(true).PaddingLeft(2)
	itemStyle := lipgloss.NewStyle().Foreground(t.Text).PaddingLeft(2)
	sizeStyle := lipgloss.NewStyle().Foreground(t.Muted)
	typeStyle := lipgloss.NewStyle().Foreground(t.Warning)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted)
	keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(t.Success).Bold(true)
	dividerStyle := lipgloss.NewStyle().Foreground(t.Muted)

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Attachments"))
	b.WriteString("\n\n")

	b.WriteString(dateStyle.Render("Entry: " + m.entry.Date))
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("-", 60)))
	b.WriteString("\n\n")

	if m.addMode {
		b.WriteString("Add attachment:\n\n")
		b.WriteString("  ")
		b.WriteString(m.pathInput.View())
		b.WriteString("\n\n")

		if m.Error != "" {
			b.WriteString("  ")
			b.WriteString(errorStyle.Render(m.Error))
			b.WriteString("\n\n")
		}

		b.WriteString(helpStyle.Render(keyStyle.Render("Enter") + " add | " + keyStyle.Render("Esc") + " cancel"))
		return b.String()
	}

	if len(m.entry.Attachments) == 0 {
		b.WriteString(itemStyle.Render("No attachments"))
		b.WriteString("\n\n")
	} else {
		for i, att := range m.entry.Attachments {
			line := att.Filename
			line += " " + sizeStyle.Render("("+storage.FormatFileSize(att.Size)+")")
			line += " " + typeStyle.Render("["+att.MimeType+"]")

			if i == m.selectedIndex {
				b.WriteString(selectedStyle.Render("> " + line))
			} else {
				b.WriteString(itemStyle.Render("  " + line))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if m.Error != "" {
		b.WriteString(errorStyle.Render(m.Error))
		b.WriteString("\n\n")
	}

	if m.Message != "" {
		b.WriteString(successStyle.Render(m.Message))
		b.WriteString("\n\n")
	}

	b.WriteString(dividerStyle.Render(strings.Repeat("-", 60)))
	b.WriteString("\n")

	var parts []string
	parts = append(parts, keyStyle.Render("a")+" add")
	if len(m.entry.Attachments) > 0 {
		parts = append(parts, keyStyle.Render("e")+" export")
		parts = append(parts, keyStyle.Render("d")+" delete")
	}
	parts = append(parts, keyStyle.Render("Esc/q")+" back")
	b.WriteString(helpStyle.Render(strings.Join(parts, " | ")))

	return b.String()
}
