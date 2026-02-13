package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette for the application
type Theme struct {
	Name string

	// Primary colors
	Title      lipgloss.Color
	Accent     lipgloss.Color
	Selected   lipgloss.Color
	Muted      lipgloss.Color
	Text       lipgloss.Color
	TextDim    lipgloss.Color
	Success    lipgloss.Color
	Error      lipgloss.Color
	Warning    lipgloss.Color
	Info       lipgloss.Color
	Disabled   lipgloss.Color
}

var themes = map[string]Theme{
	"default": {
		Name:     "default",
		Title:    lipgloss.Color("213"),
		Accent:   lipgloss.Color("219"),
		Selected: lipgloss.Color("212"),
		Muted:    lipgloss.Color("243"),
		Text:     lipgloss.Color("252"),
		TextDim:  lipgloss.Color("245"),
		Success:  lipgloss.Color("46"),
		Error:    lipgloss.Color("196"),
		Warning:  lipgloss.Color("214"),
		Info:     lipgloss.Color("87"),
		Disabled: lipgloss.Color("238"),
	},
	"ocean": {
		Name:     "ocean",
		Title:    lipgloss.Color("39"),
		Accent:   lipgloss.Color("45"),
		Selected: lipgloss.Color("51"),
		Muted:    lipgloss.Color("243"),
		Text:     lipgloss.Color("255"),
		TextDim:  lipgloss.Color("250"),
		Success:  lipgloss.Color("48"),
		Error:    lipgloss.Color("197"),
		Warning:  lipgloss.Color("220"),
		Info:     lipgloss.Color("117"),
		Disabled: lipgloss.Color("240"),
	},
	"forest": {
		Name:     "forest",
		Title:    lipgloss.Color("34"),
		Accent:   lipgloss.Color("40"),
		Selected: lipgloss.Color("46"),
		Muted:    lipgloss.Color("243"),
		Text:     lipgloss.Color("252"),
		TextDim:  lipgloss.Color("245"),
		Success:  lipgloss.Color("82"),
		Error:    lipgloss.Color("196"),
		Warning:  lipgloss.Color("178"),
		Info:     lipgloss.Color("114"),
		Disabled: lipgloss.Color("238"),
	},
	"sunset": {
		Name:     "sunset",
		Title:    lipgloss.Color("208"),
		Accent:   lipgloss.Color("214"),
		Selected: lipgloss.Color("220"),
		Muted:    lipgloss.Color("243"),
		Text:     lipgloss.Color("230"),
		TextDim:  lipgloss.Color("223"),
		Success:  lipgloss.Color("156"),
		Error:    lipgloss.Color("196"),
		Warning:  lipgloss.Color("226"),
		Info:     lipgloss.Color("216"),
		Disabled: lipgloss.Color("240"),
	},
	"monochrome": {
		Name:     "monochrome",
		Title:    lipgloss.Color("255"),
		Accent:   lipgloss.Color("250"),
		Selected: lipgloss.Color("255"),
		Muted:    lipgloss.Color("243"),
		Text:     lipgloss.Color("252"),
		TextDim:  lipgloss.Color("245"),
		Success:  lipgloss.Color("255"),
		Error:    lipgloss.Color("255"),
		Warning:  lipgloss.Color("250"),
		Info:     lipgloss.Color("248"),
		Disabled: lipgloss.Color("240"),
	},
	"dracula": {
		Name:     "dracula",
		Title:    lipgloss.Color("141"),
		Accent:   lipgloss.Color("212"),
		Selected: lipgloss.Color("84"),
		Muted:    lipgloss.Color("61"),
		Text:     lipgloss.Color("253"),
		TextDim:  lipgloss.Color("246"),
		Success:  lipgloss.Color("84"),
		Error:    lipgloss.Color("210"),
		Warning:  lipgloss.Color("228"),
		Info:     lipgloss.Color("117"),
		Disabled: lipgloss.Color("59"),
	},
}

var current = themes["monochrome"]

// Get returns a theme by name, defaulting to "monochrome" if not found
func Get(name string) Theme {
	if t, ok := themes[name]; ok {
		return t
	}
	return themes["monochrome"]
}

// Current returns the currently active theme
func Current() Theme {
	return current
}

// Set sets the current theme by name
func Set(name string) {
	current = Get(name)
}

// List returns all available theme names
func List() []string {
	return []string{"monochrome", "default", "ocean", "forest", "sunset", "dracula"}
}
