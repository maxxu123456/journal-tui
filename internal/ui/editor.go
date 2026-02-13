package ui

import (
	"strings"
	"time"

	"journal/internal/model"
	"journal/internal/theme"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

type editorField int

const (
	fieldDate editorField = iota
	fieldContent
)

type EditorModel struct {
	dateInput    textinput.Model
	contentArea  textarea.Model
	focusedField editorField
	EditingEntry *model.Entry
	Saved        bool
	Cancelled    bool
	Error        string
	width        int
	height       int
}

func NewEditorModel(entry *model.Entry) EditorModel {
	ti := textinput.New()
	ti.Placeholder = "YYYY-MM-DD"
	ti.CharLimit = 10
	ti.Width = 12

	ta := textarea.New()
	ta.Placeholder = "Write your journal entry..."
	ta.CharLimit = 0
	ta.SetWidth(60)
	ta.SetHeight(10)

	m := EditorModel{
		dateInput:    ti,
		contentArea:  ta,
		focusedField: fieldDate,
		EditingEntry: entry,
	}

	if entry != nil {
		ti.SetValue(entry.Date)
		ta.SetValue(entry.Content)
		m.dateInput = ti
		m.contentArea = ta
	} else {
		ti.SetValue(time.Now().Format("2006-01-02"))
		m.dateInput = ti
	}

	return m
}

func (m *EditorModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	contentWidth := width - 6
	if contentWidth < 20 {
		contentWidth = 20
	}
	if contentWidth > 100 {
		contentWidth = 100
	}

	contentHeight := height - 14
	if contentHeight < 5 {
		contentHeight = 5
	}

	m.contentArea.SetWidth(contentWidth)
	m.contentArea.SetHeight(contentHeight)
}

func (m EditorModel) Init() tea.Cmd {
	m.dateInput.Focus()
	return textinput.Blink
}

func (m EditorModel) Update(msg tea.Msg) (EditorModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			if m.focusedField == fieldDate {
				m.focusedField = fieldContent
				m.dateInput.Blur()
				m.contentArea.Focus()
				return m, textarea.Blink
			} else {
				m.focusedField = fieldDate
				m.contentArea.Blur()
				m.dateInput.Focus()
				return m, textinput.Blink
			}

		case "esc":
			m.Cancelled = true
			return m, nil

		case "ctrl+s":
			if m.dateInput.Value() != "" && m.contentArea.Value() != "" {
				m.Saved = true
			}
			return m, nil
		}
	}

	m.Error = ""

	if m.focusedField == fieldDate {
		m.dateInput, cmd = m.dateInput.Update(msg)
	} else {
		m.contentArea, cmd = m.contentArea.Update(msg)
	}

	return m, cmd
}

func (m EditorModel) GetDate() string {
	return m.dateInput.Value()
}

func (m EditorModel) GetEntry() model.Entry {
	now := time.Now()

	if m.EditingEntry != nil {
		return model.Entry{
			ID:        m.EditingEntry.ID,
			Date:      m.dateInput.Value(),
			Content:   m.contentArea.Value(),
			CreatedAt: m.EditingEntry.CreatedAt,
			UpdatedAt: now,
		}
	}

	return model.Entry{
		ID:        uuid.New().String(),
		Date:      m.dateInput.Value(),
		Content:   m.contentArea.Value(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (m EditorModel) View() string {
	t := theme.Current()
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Title)
	labelStyle := lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	labelActiveStyle := lipgloss.NewStyle().Foreground(t.Selected).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted)
	keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(t.TextDim).Italic(true)

	b.WriteString("\n")

	title := "New Entry"
	if m.EditingEntry != nil {
		title = "Edit Entry"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	dateLabel := "Date:"
	if m.focusedField == fieldDate {
		b.WriteString(labelActiveStyle.Render("> " + dateLabel))
	} else {
		b.WriteString(labelStyle.Render("  " + dateLabel))
	}
	b.WriteString(" ")
	b.WriteString(m.dateInput.View())
	b.WriteString("  ")
	b.WriteString(hintStyle.Render("(YYYY-MM-DD)"))
	b.WriteString("\n\n")

	contentLabel := "Content:"
	if m.focusedField == fieldContent {
		b.WriteString(labelActiveStyle.Render("> " + contentLabel))
	} else {
		b.WriteString(labelStyle.Render("  " + contentLabel))
	}
	b.WriteString("\n")
	b.WriteString(m.contentArea.View())
	b.WriteString("\n")

	if m.Error != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.Error))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	var parts []string
	parts = append(parts, keyStyle.Render("Tab")+" switch fields")
	parts = append(parts, keyStyle.Render("Ctrl+S")+" save")
	parts = append(parts, keyStyle.Render("Esc")+" cancel")
	b.WriteString(helpStyle.Render(strings.Join(parts, " | ")))

	return b.String()
}
