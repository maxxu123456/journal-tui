package ui

import (
	"fmt"
	"strings"
	"time"

	"journal/internal/model"
	"journal/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ListAction int

const (
	ActionNone ListAction = iota
	ActionNewEntry
	ActionEditEntry
	ActionDeleteEntry
	ActionSettings
	ActionViewHistory
	ActionViewAttachments
	ActionQuit
)

type ListModel struct {
	journal       *model.Journal
	SelectedIndex int
	Action        ListAction
	width         int
	height        int
	offset        int
}

func NewListModel(journal *model.Journal) ListModel {
	return ListModel{
		journal:       journal,
		SelectedIndex: 0,
		Action:        ActionNone,
	}
}

func (m *ListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m ListModel) Init() tea.Cmd {
	return nil
}

func (m ListModel) hasTodayEntry() bool {
	today := time.Now().Format("2006-01-02")
	for _, e := range m.journal.Entries {
		if e.Date == today {
			return true
		}
	}
	return false
}

func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.SelectedIndex > 0 {
				m.SelectedIndex--
				m.adjustScroll()
			}
		case "down", "j":
			if m.SelectedIndex < len(m.journal.Entries)-1 {
				m.SelectedIndex++
				m.adjustScroll()
			}
		case "enter":
			if len(m.journal.Entries) > 0 {
				m.Action = ActionEditEntry
			}
		case "n":
			if !m.hasTodayEntry() {
				m.Action = ActionNewEntry
			}
		case "d":
			if len(m.journal.Entries) > 0 {
				m.Action = ActionDeleteEntry
			}
		case "h":
			if len(m.journal.Entries) > 0 {
				m.Action = ActionViewHistory
			}
		case "a":
			if len(m.journal.Entries) > 0 {
				m.Action = ActionViewAttachments
			}
		case "s":
			m.Action = ActionSettings
		case "q":
			m.Action = ActionQuit
		}
	}

	return m, nil
}

func (m *ListModel) adjustScroll() {
	visibleLines := m.height - 8
	if visibleLines < 1 {
		visibleLines = 10
	}

	if m.SelectedIndex < m.offset {
		m.offset = m.SelectedIndex
	} else if m.SelectedIndex >= m.offset+visibleLines {
		m.offset = m.SelectedIndex - visibleLines + 1
	}
}

func (m ListModel) View() string {
	t := theme.Current()
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Title)
	itemStyle := lipgloss.NewStyle().PaddingLeft(2)
	selectedStyle := lipgloss.NewStyle().Foreground(t.Selected).Bold(true).PaddingLeft(2)
	dateStyle := lipgloss.NewStyle().Foreground(t.Info).Bold(true)
	previewStyle := lipgloss.NewStyle().Foreground(t.Text)
	emptyStyle := lipgloss.NewStyle().Foreground(t.TextDim).Italic(true).PaddingLeft(2)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted)
	disabledStyle := lipgloss.NewStyle().Foreground(t.Disabled).Strikethrough(true)
	keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	scrollStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
	badgeStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	attachBadgeStyle := lipgloss.NewStyle().Foreground(t.Success).Bold(true)

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Journal Entries"))
	b.WriteString("\n\n")

	if len(m.journal.Entries) == 0 {
		b.WriteString(emptyStyle.Render("No entries yet. Press 'n' to create one."))
		b.WriteString("\n")
	} else {
		visibleLines := m.height - 8
		if visibleLines < 1 {
			visibleLines = 10
		}

		end := m.offset + visibleLines
		if end > len(m.journal.Entries) {
			end = len(m.journal.Entries)
		}

		for i := m.offset; i < end; i++ {
			entry := m.journal.Entries[i]
			date := dateStyle.Render("[" + entry.Date + "]")
			preview := previewStyle.Render(entry.Preview(40))

			badges := ""
			if len(entry.History) > 0 {
				badges += badgeStyle.Render(fmt.Sprintf(" [%d saves]", len(entry.History)+1))
			}
			if len(entry.Attachments) > 0 {
				badges += attachBadgeStyle.Render(fmt.Sprintf(" [%d files]", len(entry.Attachments)))
			}

			line := fmt.Sprintf("%s %s%s", date, preview, badges)

			if i == m.SelectedIndex {
				b.WriteString(selectedStyle.Render("> " + line))
			} else {
				b.WriteString(itemStyle.Render("  " + line))
			}
			b.WriteString("\n")
		}

		if len(m.journal.Entries) > visibleLines {
			scrollInfo := fmt.Sprintf("(%d-%d of %d)", m.offset+1, end, len(m.journal.Entries))
			b.WriteString(scrollStyle.Render("  " + scrollInfo))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	var parts []string
	parts = append(parts, keyStyle.Render("Up/Down")+" navigate")
	parts = append(parts, keyStyle.Render("Enter")+" edit")

	if m.hasTodayEntry() {
		parts = append(parts, disabledStyle.Render("n new"))
	} else {
		parts = append(parts, keyStyle.Render("n")+" new")
	}

	parts = append(parts, keyStyle.Render("a")+" attachments")
	parts = append(parts, keyStyle.Render("h")+" history")
	parts = append(parts, keyStyle.Render("d")+" delete")
	parts = append(parts, keyStyle.Render("s")+" settings")
	parts = append(parts, keyStyle.Render("q")+" quit")

	b.WriteString(helpStyle.Render(strings.Join(parts, " | ")))

	return b.String()
}
