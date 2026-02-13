package ui

import (
	"fmt"
	"sort"
	"strings"

	"journal/internal/model"
	"journal/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HistoryModel struct {
	entry         *model.Entry
	selectedIndex int
	expanded      bool
	Back          bool
	width         int
	height        int
	offset        int
}

func NewHistoryModel(entry *model.Entry) HistoryModel {
	return HistoryModel{
		entry:         entry,
		selectedIndex: 0,
		expanded:      false,
	}
}

func (m *HistoryModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m HistoryModel) Init() tea.Cmd {
	return nil
}

func (m HistoryModel) Update(msg tea.Msg) (HistoryModel, tea.Cmd) {
	totalItems := len(m.entry.History) + 1

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
				m.expanded = false
			}
		case "down", "j":
			if m.selectedIndex < totalItems-1 {
				m.selectedIndex++
				m.expanded = false
			}
		case "enter":
			m.expanded = !m.expanded
		case "esc", "q":
			m.Back = true
		}
	}

	return m, nil
}

func (m HistoryModel) View() string {
	t := theme.Current()
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Title)
	dateStyle := lipgloss.NewStyle().Foreground(t.Info).Bold(true)
	itemStyle := lipgloss.NewStyle().PaddingLeft(2)
	selectedStyle := lipgloss.NewStyle().Foreground(t.Selected).Bold(true).PaddingLeft(2)
	timestampStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	contentStyle := lipgloss.NewStyle().Foreground(t.Text).PaddingLeft(4)
	expandedContentStyle := lipgloss.NewStyle().Foreground(t.Text).PaddingLeft(4).Width(70)
	currentBadge := lipgloss.NewStyle().Foreground(t.Success).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted)
	keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	dividerStyle := lipgloss.NewStyle().Foreground(t.Muted)
	fileStyle := lipgloss.NewStyle().Foreground(t.Accent).Italic(true)
	fileLabelStyle := lipgloss.NewStyle().Foreground(t.Muted).PaddingLeft(4)

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Save History"))
	b.WriteString("\n\n")

	b.WriteString(dateStyle.Render("Entry: " + m.entry.Date))
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("-", 60)))
	b.WriteString("\n\n")

	// Sort history by most recent first (create a sorted copy)
	sortedHistory := make([]model.SaveRecord, len(m.entry.History))
	copy(sortedHistory, m.entry.History)
	sort.Slice(sortedHistory, func(i, j int) bool {
		return sortedHistory[i].SavedAt.After(sortedHistory[j].SavedAt)
	})

	// Current version (index 0) - most recent
	currentLabel := timestampStyle.Render(m.entry.UpdatedAt.Format("2006-01-02 15:04:05"))
	currentLabel += " " + currentBadge.Render("[Current]")

	if m.selectedIndex == 0 {
		b.WriteString(selectedStyle.Render("> " + currentLabel))
	} else {
		b.WriteString(itemStyle.Render("  " + currentLabel))
	}
	b.WriteString("\n")

	// Show content (expanded or truncated)
	if m.selectedIndex == 0 && m.expanded {
		b.WriteString(expandedContentStyle.Render(m.entry.Content))
	} else {
		b.WriteString(contentStyle.Render(truncate(m.entry.Content, 100)))
	}
	b.WriteString("\n")

	// Show current attachments
	if len(m.entry.Attachments) > 0 {
		b.WriteString(fileLabelStyle.Render("Files: "))
		var fileNames []string
		for _, att := range m.entry.Attachments {
			fileNames = append(fileNames, att.Filename)
		}
		b.WriteString(fileStyle.Render(strings.Join(fileNames, ", ")))
	} else {
		b.WriteString(fileLabelStyle.Render("Files: "))
		b.WriteString(fileStyle.Render("(none)"))
	}
	b.WriteString("\n\n")

	// Historical versions (sorted most recent first)
	for i, record := range sortedHistory {
		displayIndex := i + 1

		label := timestampStyle.Render(record.SavedAt.Format("2006-01-02 15:04:05"))
		label += fmt.Sprintf(" (v%d)", len(sortedHistory)-i)

		if m.selectedIndex == displayIndex {
			b.WriteString(selectedStyle.Render("> " + label))
		} else {
			b.WriteString(itemStyle.Render("  " + label))
		}
		b.WriteString("\n")

		// Show content (expanded or truncated)
		if m.selectedIndex == displayIndex && m.expanded {
			b.WriteString(expandedContentStyle.Render(record.Content))
		} else {
			b.WriteString(contentStyle.Render(truncate(record.Content, 100)))
		}
		b.WriteString("\n")

		// Show attachments for this save
		if len(record.Attachments) > 0 {
			b.WriteString(fileLabelStyle.Render("Files: "))
			b.WriteString(fileStyle.Render(strings.Join(record.Attachments, ", ")))
		} else {
			b.WriteString(fileLabelStyle.Render("Files: "))
			b.WriteString(fileStyle.Render("(none)"))
		}
		b.WriteString("\n\n")
	}

	b.WriteString(dividerStyle.Render(strings.Repeat("-", 60)))
	b.WriteString("\n")

	var parts []string
	parts = append(parts, keyStyle.Render("Up/Down")+" navigate")
	parts = append(parts, keyStyle.Render("Enter")+" expand/collapse")
	parts = append(parts, keyStyle.Render("Esc/q")+" back")
	b.WriteString(helpStyle.Render(strings.Join(parts, " | ")))

	return b.String()
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
