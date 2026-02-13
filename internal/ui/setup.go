package ui

import (
	"strings"

	"journal/internal/storage"
	"journal/internal/theme"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type setupStep int

const (
	stepChoosePath setupStep = iota
	stepEnterName
	stepChooseEncryption
	stepEnterPassword
	stepConfirmPassword
)

type SetupModel struct {
	step            setupStep
	textInput       textinput.Model
	nameInput       textinput.Model
	passwordInput   textinput.Model
	confirmInput    textinput.Model
	selectedOpt     int
	encryptSelected int
	showPathInput   bool
	DBPath          string
	Name            string
	Encrypt         bool
	Password        string
	Done            bool
	Error           string
	defaultPath     string
}

func NewSetupModel() SetupModel {
	ti := textinput.New()
	ti.Placeholder = "Enter path..."
	ti.CharLimit = 256
	ti.Width = 50

	ni := textinput.New()
	ni.Placeholder = "My Journal"
	ni.CharLimit = 50
	ni.Width = 30

	pi := textinput.New()
	pi.Placeholder = "Enter password"
	pi.EchoMode = textinput.EchoPassword
	pi.EchoCharacter = '*'
	pi.CharLimit = 256
	pi.Width = 30

	ci := textinput.New()
	ci.Placeholder = "Confirm password"
	ci.EchoMode = textinput.EchoPassword
	ci.EchoCharacter = '*'
	ci.CharLimit = 256
	ci.Width = 30

	defaultPath, _ := storage.GetDefaultDBPath()

	return SetupModel{
		step:          stepChoosePath,
		textInput:     ti,
		nameInput:     ni,
		passwordInput: pi,
		confirmInput:  ci,
		selectedOpt:   0,
		defaultPath:   defaultPath,
	}
}

func (m SetupModel) Init() tea.Cmd {
	return nil
}

func (m SetupModel) Update(msg tea.Msg) (SetupModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case stepChoosePath:
			if m.showPathInput {
				switch msg.String() {
				case "enter":
					if m.textInput.Value() != "" {
						m.DBPath = m.textInput.Value()
						m.step = stepEnterName
						m.nameInput.Focus()
						m.showPathInput = false
						return m, textinput.Blink
					}
					return m, nil
				case "esc":
					m.showPathInput = false
					m.textInput.Blur()
					return m, nil
				}
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}

			switch msg.String() {
			case "up", "k":
				if m.selectedOpt > 0 {
					m.selectedOpt--
				}
			case "down", "j":
				if m.selectedOpt < 1 {
					m.selectedOpt++
				}
			case "enter":
				if m.selectedOpt == 0 {
					m.DBPath = m.defaultPath
					m.step = stepEnterName
					m.nameInput.Focus()
					return m, textinput.Blink
				} else {
					m.showPathInput = true
					m.textInput.Focus()
					return m, textinput.Blink
				}
			}

		case stepEnterName:
			switch msg.String() {
			case "enter":
				m.Name = m.nameInput.Value()
				if m.Name == "" {
					m.Name = "My Journal"
				}
				m.step = stepChooseEncryption
				m.nameInput.Blur()
				return m, nil
			case "esc":
				m.step = stepChoosePath
				m.nameInput.Blur()
				return m, nil
			}
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd

		case stepChooseEncryption:
			switch msg.String() {
			case "up", "k":
				if m.encryptSelected > 0 {
					m.encryptSelected--
				}
			case "down", "j":
				if m.encryptSelected < 1 {
					m.encryptSelected++
				}
			case "enter":
				if m.encryptSelected == 0 {
					m.Encrypt = false
					m.Done = true
				} else {
					m.Encrypt = true
					m.step = stepEnterPassword
					m.passwordInput.Focus()
					return m, textinput.Blink
				}
			case "esc":
				m.step = stepEnterName
				m.nameInput.Focus()
				return m, textinput.Blink
			}

		case stepEnterPassword:
			switch msg.String() {
			case "enter":
				if m.passwordInput.Value() != "" {
					m.Password = m.passwordInput.Value()
					m.step = stepConfirmPassword
					m.confirmInput.Focus()
					return m, textinput.Blink
				}
				return m, nil
			case "esc":
				m.step = stepChooseEncryption
				m.passwordInput.SetValue("")
				return m, nil
			}
			m.Error = ""
			m.passwordInput, cmd = m.passwordInput.Update(msg)
			return m, cmd

		case stepConfirmPassword:
			switch msg.String() {
			case "enter":
				if m.confirmInput.Value() == m.Password {
					m.Done = true
				} else {
					m.Error = "Passwords do not match"
					m.confirmInput.SetValue("")
				}
				return m, nil
			case "esc":
				m.step = stepEnterPassword
				m.confirmInput.SetValue("")
				return m, nil
			}
			m.Error = ""
			m.confirmInput, cmd = m.confirmInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m SetupModel) View() string {
	t := theme.Current()
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Title)
	welcomeStyle := lipgloss.NewStyle().Foreground(t.Title).Bold(true)
	promptStyle := lipgloss.NewStyle().Foreground(t.Text)
	optionStyle := lipgloss.NewStyle().Foreground(t.Text).PaddingLeft(2)
	selectedStyle := lipgloss.NewStyle().Foreground(t.Selected).Bold(true).PaddingLeft(2)
	pathStyle := lipgloss.NewStyle().Foreground(t.Info).Italic(true)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted)
	keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Journal Setup"))
	b.WriteString("\n\n")

	b.WriteString(welcomeStyle.Render("Welcome to Journal!"))
	b.WriteString("\n\n")

	switch m.step {
	case stepChoosePath:
		b.WriteString(promptStyle.Render("Where would you like to store your journal?"))
		b.WriteString("\n\n")

		opt1 := "Use default location"
		if m.selectedOpt == 0 {
			b.WriteString(selectedStyle.Render("> " + opt1))
		} else {
			b.WriteString(optionStyle.Render("  " + opt1))
		}
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(pathStyle.Render(m.defaultPath))
		b.WriteString("\n\n")

		opt2 := "Enter custom path"
		if m.selectedOpt == 1 {
			b.WriteString(selectedStyle.Render("> " + opt2))
		} else {
			b.WriteString(optionStyle.Render("  " + opt2))
		}
		b.WriteString("\n")

		if m.showPathInput {
			b.WriteString("\n")
			b.WriteString("    ")
			b.WriteString(m.textInput.View())
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("    " + keyStyle.Render("Enter") + " confirm  " + keyStyle.Render("Esc") + " cancel"))
		} else {
			b.WriteString("\n")
			b.WriteString(helpStyle.Render(keyStyle.Render("Up/Down") + " navigate  " + keyStyle.Render("Enter") + " select"))
		}

	case stepEnterName:
		b.WriteString(promptStyle.Render("Give your journal a name:"))
		b.WriteString("\n\n")
		b.WriteString("  ")
		b.WriteString(m.nameInput.View())
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render(keyStyle.Render("Enter") + " continue  " + keyStyle.Render("Esc") + " back"))

	case stepChooseEncryption:
		b.WriteString(promptStyle.Render("Would you like to encrypt your journal?"))
		b.WriteString("\n\n")

		opt1 := "No encryption"
		if m.encryptSelected == 0 {
			b.WriteString(selectedStyle.Render("> " + opt1))
		} else {
			b.WriteString(optionStyle.Render("  " + opt1))
		}
		b.WriteString("\n")

		opt2 := "Yes, encrypt with password"
		if m.encryptSelected == 1 {
			b.WriteString(selectedStyle.Render("> " + opt2))
		} else {
			b.WriteString(optionStyle.Render("  " + opt2))
		}
		b.WriteString("\n\n")

		b.WriteString(helpStyle.Render(keyStyle.Render("Enter") + " select  " + keyStyle.Render("Esc") + " back"))

	case stepEnterPassword:
		b.WriteString(promptStyle.Render("Enter a password for encryption:"))
		b.WriteString("\n\n")
		b.WriteString("  ")
		b.WriteString(m.passwordInput.View())
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render(keyStyle.Render("Enter") + " continue  " + keyStyle.Render("Esc") + " back"))

	case stepConfirmPassword:
		b.WriteString(promptStyle.Render("Confirm your password:"))
		b.WriteString("\n\n")
		b.WriteString("  ")
		b.WriteString(m.confirmInput.View())
		b.WriteString("\n")

		if m.Error != "" {
			b.WriteString("\n")
			b.WriteString("  ")
			b.WriteString(errorStyle.Render(m.Error))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(helpStyle.Render(keyStyle.Render("Enter") + " confirm  " + keyStyle.Render("Esc") + " back"))
	}

	return b.String()
}
