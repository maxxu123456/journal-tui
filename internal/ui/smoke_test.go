package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestResizeZeroValueSubModels(t *testing.T) {
	a := App{currentView: ViewSelector}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic resizing zero-value sub-models: %v", r)
		}
	}()
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
}

func TestEditorDateFieldAcceptsInputImmediately(t *testing.T) {
	m := NewEditorModel(nil)
	m.dateInput.SetValue("")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if got := m.dateInput.Value(); got != "2" {
		t.Errorf("date field ignored keystroke, value = %q", got)
	}
}

func TestEditorRejectsInvalidDate(t *testing.T) {
	for _, date := range []string{"", "not-a-date", "2026-13-45", "01/02/2026"} {
		m := NewEditorModel(nil)
		m.dateInput.SetValue(date)
		m.contentArea.SetValue("hello")
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
		if m.Saved {
			t.Errorf("accepted invalid date %q", date)
		}
		if m.Error == "" {
			t.Errorf("no error reported for invalid date %q", date)
		}
	}

	m := NewEditorModel(nil)
	m.dateInput.SetValue("2026-01-02")
	m.contentArea.SetValue("hello")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if !m.Saved {
		t.Errorf("rejected valid entry: %q", m.Error)
	}
}

func TestErrorSurvivesBlinkTick(t *testing.T) {
	m := NewPasswordModel()
	m.Error = "Invalid password"
	m, _ = m.Update(struct{ tea.Msg }{})
	if m.Error == "" {
		t.Error("password error cleared by a non-key message")
	}
}
