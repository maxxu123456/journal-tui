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

func (m *HistoryModel) adjustScroll() {
	visibleItems := (m.height - 10) / 4 // ~4 lines per item
	if visibleItems < 2 {
		visibleItems = 2
	}

	if m.selectedIndex < m.offset {
		m.offset = m.selectedIndex
	} else if m.selectedIndex >= m.offset+visibleItems {
		m.offset = m.selectedIndex - visibleItems + 1
	}
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
				m.adjustScroll()
			}
		case "down", "j":
			if m.selectedIndex < totalItems-1 {
				m.selectedIndex++
				m.expanded = false
				m.adjustScroll()
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

	// Build all items: current + history
	type historyItem struct {
		index       int
		label       string
		content     string
		attachments string
	}

	var items []historyItem

	// Current version (index 0)
	currentLabel := timestampStyle.Render(m.entry.UpdatedAt.Format("2006-01-02 15:04:05"))
	currentLabel += " " + currentBadge.Render("[Current]")
	var currentFiles string
	if len(m.entry.Attachments) > 0 {
		var fileNames []string
		for _, att := range m.entry.Attachments {
			fileNames = append(fileNames, att.Filename)
		}
		currentFiles = strings.Join(fileNames, ", ")
	} else {
		currentFiles = "(none)"
	}
	items = append(items, historyItem{0, currentLabel, m.entry.Content, currentFiles})

	// Historical versions
	for i, record := range sortedHistory {
		label := timestampStyle.Render(record.SavedAt.Format("2006-01-02 15:04:05"))
		label += fmt.Sprintf(" (v%d)", len(sortedHistory)-i)
		files := "(none)"
		if len(record.Attachments) > 0 {
			files = strings.Join(record.Attachments, ", ")
		}
		items = append(items, historyItem{i + 1, label, record.Content, files})
	}

	// Render visible items based on offset
	visibleItems := (m.height - 10) / 4
	if visibleItems < 2 {
		visibleItems = 2
	}
	end := m.offset + visibleItems
	if end > len(items) {
		end = len(items)
	}

	for _, item := range items[m.offset:end] {
		if m.selectedIndex == item.index {
			b.WriteString(selectedStyle.Render("> " + item.label))
		} else {
			b.WriteString(itemStyle.Render("  " + item.label))
		}
		b.WriteString("\n")

		if m.selectedIndex == item.index && m.expanded {
			b.WriteString(expandedContentStyle.Render(item.content))
		} else {
			b.WriteString(contentStyle.Render(truncate(item.content, 100)))
		}
		b.WriteString("\n")

		b.WriteString(fileLabelStyle.Render("Files: "))
		b.WriteString(fileStyle.Render(item.attachments))
		b.WriteString("\n\n")
	}

	if len(items) > visibleItems {
		scrollInfo := fmt.Sprintf("(%d-%d of %d)", m.offset+1, end, len(items))
		scrollStyle := lipgloss.NewStyle().Foreground(t.Muted).Italic(true)
		b.WriteString(scrollStyle.Render("  " + scrollInfo))
		b.WriteString("\n")
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
