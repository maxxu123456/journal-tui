package ui

import (
	"sort"
	"time"

	"journal/internal/model"
	"journal/internal/storage"
	"journal/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ViewState represents the current view
type ViewState int

const (
	ViewSelector ViewState = iota
	ViewSetup
	ViewPassword
	ViewList
	ViewEditor
	ViewSettings
	ViewDeleteConfirm
	ViewHistory
	ViewAttachments
	ViewExport
)

// App is the main application model
type App struct {
	config        *model.Config
	journal       *model.Journal
	activeJournal *model.JournalDB
	currentView   ViewState
	password      string

	// Sub-models
	selectorModel    SelectorModel
	setupModel       SetupModel
	passwordModel    PasswordModel
	listModel        ListModel
	editorModel      EditorModel
	settingsModel    SettingsModel
	historyModel     HistoryModel
	attachmentModel  AttachmentModel
	exportModel      ExportModel

	// State
	width  int
	height int
	err    error
}

// InitialModel creates the initial application model
func InitialModel() App {
	app := App{
		currentView: ViewSetup,
	}

	// Check if config exists
	exists, err := storage.ConfigExists()
	if err != nil {
		app.err = err
		return app
	}

	if exists {
		config, err := storage.LoadConfig()
		if err != nil {
			app.err = err
			return app
		}
		app.config = config

		// Migrate old config format if needed
		if storage.MigrateConfigToNewFormat(config) {
			storage.SaveConfig(config)
		}

		// Set theme from config
		if config.Theme != "" {
			theme.Set(config.Theme)
		}

		// If there are journals, show selector
		if len(config.Journals) > 0 {
			journals := storage.GetSortedJournals(config)
			app.selectorModel = NewSelectorModel(journals, config.Theme)
			app.currentView = ViewSelector
		} else {
			app.setupModel = NewSetupModel()
			app.currentView = ViewSetup
		}
	} else {
		app.setupModel = NewSetupModel()
	}

	return app
}

func sortEntriesNewestFirst(journal *model.Journal) {
	sort.Slice(journal.Entries, func(i, j int) bool {
		return journal.Entries[i].Date > journal.Entries[j].Date
	})
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		switch a.currentView {
		case ViewList:
			a.listModel.SetSize(msg.Width, msg.Height)
		case ViewEditor:
			a.editorModel.SetSize(msg.Width, msg.Height)
		case ViewHistory:
			a.historyModel.SetSize(msg.Width, msg.Height)
		case ViewAttachments:
			a.attachmentModel.SetSize(msg.Width, msg.Height)
		}
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		}
	}

	var cmd tea.Cmd

	switch a.currentView {
	case ViewSelector:
		a.selectorModel, cmd = a.selectorModel.Update(msg)
		if a.selectorModel.Done {
			// Save theme if changed
			if a.selectorModel.ThemeChanged {
				a.config.Theme = a.selectorModel.NewTheme
				storage.SaveConfig(a.config)
			}

			if a.selectorModel.CreateNew {
				a.setupModel = NewSetupModel(a.existingJournalPaths()...)
				a.currentView = ViewSetup
			} else if a.selectorModel.Selected != nil {
				// Find the journal in config to get a pointer into config.Journals
				// (selectorModel.Selected is a copy, not a reference into config)
				selected := a.selectorModel.Selected
				a.activeJournal = storage.FindJournal(a.config, selected.Path)
				if a.activeJournal == nil {
					// Fallback: use the selector's copy
					a.activeJournal = selected
				}

				// Update last opened time
				storage.UpdateJournalLastOpened(a.config, a.activeJournal.Path, time.Now())
				a.config.ActiveJournal = a.activeJournal.Path
				storage.SaveConfig(a.config)

				if a.activeJournal.Encrypted {
					a.passwordModel = NewPasswordModel()
					a.currentView = ViewPassword
				} else {
					journal, err := storage.LoadJournal(a.activeJournal.Path)
					if err != nil {
						a.err = err
						return a, nil
					}
					a.journal = journal
					sortEntriesNewestFirst(a.journal)
					a.currentView = ViewList
					a.listModel = NewListModel(a.journal)
					a.listModel.SetSize(a.width, a.height)
				}
			}
		}

	case ViewSetup:
		a.setupModel, cmd = a.setupModel.Update(msg)
		if a.setupModel.Done {
			if a.config == nil {
				a.config = &model.Config{}
			}

			// Add new journal to config
			storage.AddJournal(a.config, a.setupModel.Name, a.setupModel.DBPath, a.setupModel.Encrypt)
			a.config.ActiveJournal = a.setupModel.DBPath

			// Find the journal we just added
			a.activeJournal = storage.FindJournal(a.config, a.setupModel.DBPath)
			storage.UpdateJournalLastOpened(a.config, a.setupModel.DBPath, time.Now())

			if err := storage.SaveConfig(a.config); err != nil {
				a.err = err
				return a, nil
			}

			if a.setupModel.Encrypt {
				a.password = a.setupModel.Password
				if err := storage.CreateEmptyJournalEncrypted(a.setupModel.DBPath, a.password); err != nil {
					a.err = err
					return a, nil
				}
			} else {
				if err := storage.CreateEmptyJournal(a.setupModel.DBPath); err != nil {
					a.err = err
					return a, nil
				}
			}

			a.journal = &model.Journal{Entries: []model.Entry{}}
			a.currentView = ViewList
			a.listModel = NewListModel(a.journal)
			a.listModel.SetSize(a.width, a.height)
		}

	case ViewPassword:
		a.passwordModel, cmd = a.passwordModel.Update(msg)
		if a.passwordModel.Cancelled {
			// Go back to selector
			journals := storage.GetSortedJournals(a.config)
			a.selectorModel = NewSelectorModel(journals, a.config.Theme)
			a.currentView = ViewSelector
			a.activeJournal = nil
			a.password = ""
			return a, nil
		}
		if a.passwordModel.Done {
			journal, err := storage.LoadJournalEncrypted(a.activeJournal.Path, a.passwordModel.Password)
			if err != nil {
				if err == storage.ErrInvalidPassword {
					a.passwordModel.Error = "Invalid password"
					a.passwordModel.Done = false
					a.passwordModel.Password = ""
				} else {
					a.err = err
				}
				return a, nil
			}

			a.password = a.passwordModel.Password
			a.journal = journal
			sortEntriesNewestFirst(a.journal)
			a.currentView = ViewList
			a.listModel = NewListModel(a.journal)
			a.listModel.SetSize(a.width, a.height)
		}

	case ViewList:
		a.listModel, cmd = a.listModel.Update(msg)

		switch a.listModel.Action {
		case ActionNewEntry:
			a.editorModel = NewEditorModel(nil)
			a.editorModel.SetSize(a.width, a.height)
			a.currentView = ViewEditor
			a.listModel.Action = ActionNone
			return a, a.editorModel.Init()

		case ActionEditEntry:
			if a.listModel.SelectedIndex >= 0 && a.listModel.SelectedIndex < len(a.journal.Entries) {
				entry := &a.journal.Entries[a.listModel.SelectedIndex]
				a.editorModel = NewEditorModel(entry)
				a.editorModel.SetSize(a.width, a.height)
				a.currentView = ViewEditor
				a.listModel.Action = ActionNone
				return a, a.editorModel.Init()
			}

		case ActionDeleteEntry:
			a.currentView = ViewDeleteConfirm
			a.listModel.Action = ActionNone

		case ActionViewHistory:
			if a.listModel.SelectedIndex >= 0 && a.listModel.SelectedIndex < len(a.journal.Entries) {
				entry := &a.journal.Entries[a.listModel.SelectedIndex]
				a.historyModel = NewHistoryModel(entry)
				a.historyModel.SetSize(a.width, a.height)
				a.currentView = ViewHistory
				a.listModel.Action = ActionNone
			}

		case ActionViewAttachments:
			if a.listModel.SelectedIndex >= 0 && a.listModel.SelectedIndex < len(a.journal.Entries) {
				entry := &a.journal.Entries[a.listModel.SelectedIndex]
				a.attachmentModel = NewAttachmentModel(entry, a.activeJournal.Path, a.activeJournal.Encrypted, a.password)
				a.attachmentModel.SetSize(a.width, a.height)
				a.currentView = ViewAttachments
				a.listModel.Action = ActionNone
			}

		case ActionSettings:
			a.settingsModel = NewSettingsModel(a.config, a.activeJournal)
			a.currentView = ViewSettings
			a.listModel.Action = ActionNone

		case ActionQuit:
			return a, tea.Quit
		}

	case ViewEditor:
		a.editorModel, cmd = a.editorModel.Update(msg)

		if a.editorModel.Cancelled {
			a.currentView = ViewList
			a.editorModel.Cancelled = false
		} else if a.editorModel.Saved {
			newDate := a.editorModel.GetDate()
			duplicate := false
			for _, e := range a.journal.Entries {
				if e.Date == newDate {
					if a.editorModel.EditingEntry != nil && e.ID == a.editorModel.EditingEntry.ID {
						continue
					}
					duplicate = true
					break
				}
			}

			if duplicate {
				a.editorModel.Error = "An entry for " + newDate + " already exists"
				a.editorModel.Saved = false
				return a, nil
			}

			entry := a.editorModel.GetEntry()
			if a.editorModel.EditingEntry != nil {
				for i, e := range a.journal.Entries {
					if e.ID == entry.ID {
						if e.Content != entry.Content {
							historyRecord := model.SaveRecord{
								Content:     e.Content,
								SavedAt:     e.UpdatedAt,
								Attachments: e.AttachmentFilenames(),
							}
							entry.History = append(e.History, historyRecord)
						} else {
							entry.History = e.History
						}
						entry.Attachments = e.Attachments
						a.journal.Entries[i] = entry
						break
					}
				}
			} else {
				a.journal.Entries = append(a.journal.Entries, entry)
			}

			sortEntriesNewestFirst(a.journal)
			if err := a.saveJournal(); err != nil {
				a.err = err
				return a, nil
			}

			a.listModel = NewListModel(a.journal)
			a.listModel.SetSize(a.width, a.height)
			a.currentView = ViewList
			a.editorModel.Saved = false
		}

	case ViewDeleteConfirm:
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				if a.listModel.SelectedIndex >= 0 && a.listModel.SelectedIndex < len(a.journal.Entries) {
					entryID := a.journal.Entries[a.listModel.SelectedIndex].ID
					a.journal.Entries = append(
						a.journal.Entries[:a.listModel.SelectedIndex],
						a.journal.Entries[a.listModel.SelectedIndex+1:]...,
					)
					// Delete from database (handles attachments too)
					if a.activeJournal.Encrypted {
						a.saveJournal()
					} else {
						storage.DeleteEntry(a.activeJournal.Path, entryID)
					}
					a.listModel = NewListModel(a.journal)
					a.listModel.SetSize(a.width, a.height)
				}
				a.currentView = ViewList
			case "n", "N", "esc":
				a.currentView = ViewList
			}
		}

	case ViewHistory:
		a.historyModel, cmd = a.historyModel.Update(msg)

		if a.historyModel.Back {
			a.currentView = ViewList
			a.historyModel.Back = false
		}

	case ViewAttachments:
		a.attachmentModel, cmd = a.attachmentModel.Update(msg)

		if a.attachmentModel.Back {
			// Reload entry attachments
			if a.listModel.SelectedIndex >= 0 && a.listModel.SelectedIndex < len(a.journal.Entries) {
				entry := &a.journal.Entries[a.listModel.SelectedIndex]
				entry.Attachments = a.attachmentModel.entry.Attachments
			}
			a.currentView = ViewList
			a.attachmentModel.Back = false
		} else if a.attachmentModel.ExportSelected {
			a.exportModel = NewExportModel(
				a.attachmentModel.SelectedAttachment(),
				a.activeJournal.Path,
				a.activeJournal.Encrypted,
				a.password,
			)
			a.currentView = ViewExport
			a.attachmentModel.ExportSelected = false
		}

	case ViewExport:
		a.exportModel, cmd = a.exportModel.Update(msg)

		if a.exportModel.Done || a.exportModel.Cancelled {
			a.currentView = ViewAttachments
			a.exportModel.Done = false
			a.exportModel.Cancelled = false
		}

	case ViewSettings:
		a.settingsModel, cmd = a.settingsModel.Update(msg)

		if a.settingsModel.Cancelled {
			a.currentView = ViewList
			a.settingsModel.Cancelled = false
		} else if a.settingsModel.Saved {
			oldPath := a.config.ActiveJournal
			newPath := a.settingsModel.DBPath

			if oldPath != newPath {
				if a.settingsModel.Migrate {
					if a.activeJournal != nil && a.activeJournal.Encrypted {
						if err := storage.MigrateJournalEncrypted(oldPath, newPath, a.password); err != nil {
							a.err = err
							return a, nil
						}
					} else {
						if err := storage.MigrateJournal(oldPath, newPath); err != nil {
							a.err = err
							return a, nil
						}
					}
				} else {
					if a.activeJournal != nil && a.activeJournal.Encrypted {
						if err := storage.CreateEmptyJournalEncrypted(newPath, a.password); err != nil {
							a.err = err
							return a, nil
						}
					} else {
						if err := storage.CreateEmptyJournal(newPath); err != nil {
							a.err = err
							return a, nil
						}
					}
				}

				a.config.ActiveJournal = newPath
				if a.activeJournal != nil {
					a.activeJournal.Path = newPath
				}

				var journal *model.Journal
				var err error
				if a.activeJournal != nil && a.activeJournal.Encrypted {
					journal, err = storage.LoadJournalEncrypted(newPath, a.password)
				} else {
					journal, err = storage.LoadJournal(newPath)
				}
				if err != nil {
					a.err = err
					return a, nil
				}
				a.journal = journal
				sortEntriesNewestFirst(a.journal)
				a.listModel = NewListModel(a.journal)
				a.listModel.SetSize(a.width, a.height)
			}

			if err := storage.SaveConfig(a.config); err != nil {
				a.err = err
				return a, nil
			}

			a.currentView = ViewList
			a.settingsModel.Saved = false
		}
	}

	return a, cmd
}

func (a App) existingJournalPaths() []string {
	if a.config == nil {
		return nil
	}
	paths := make([]string, len(a.config.Journals))
	for i, j := range a.config.Journals {
		paths[i] = j.Path
	}
	return paths
}

func (a App) saveJournal() error {
	path := a.config.ActiveJournal
	if a.activeJournal != nil {
		path = a.activeJournal.Path
	}
	if a.activeJournal != nil && a.activeJournal.Encrypted {
		return storage.SaveJournalEncrypted(a.journal, path, a.password)
	}
	return storage.SaveJournal(a.journal, path)
}

func (a App) View() string {
	if a.err != nil {
		return "Error: " + a.err.Error() + "\n\nPress Ctrl+C to quit."
	}

	switch a.currentView {
	case ViewSelector:
		return a.selectorModel.View()
	case ViewSetup:
		return a.setupModel.View()
	case ViewPassword:
		return a.passwordModel.View()
	case ViewList:
		return a.listModel.View()
	case ViewEditor:
		return a.editorModel.View()
	case ViewSettings:
		return a.settingsModel.View()
	case ViewDeleteConfirm:
		return a.renderDeleteConfirm()
	case ViewHistory:
		return a.historyModel.View()
	case ViewAttachments:
		return a.attachmentModel.View()
	case ViewExport:
		return a.exportModel.View()
	}

	return ""
}

func (a App) renderDeleteConfirm() string {
	t := theme.Current()

	if a.listModel.SelectedIndex < 0 || a.listModel.SelectedIndex >= len(a.journal.Entries) {
		return "No entry selected"
	}

	entry := a.journal.Entries[a.listModel.SelectedIndex]

	promptStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(t.Muted)
	keyStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)

	var s string
	s += "\n"
	s += promptStyle.Render("Delete Entry?") + "\n\n"
	s += labelStyle.Render("  Date: ") + entry.Date + "\n"
	s += labelStyle.Render("  Preview: ") + entry.Preview(50) + "\n\n"
	s += helpStyle.Render("  Press ") + keyStyle.Render("y") + helpStyle.Render(" to confirm, ")
	s += keyStyle.Render("n") + helpStyle.Render(" or ") + keyStyle.Render("Esc") + helpStyle.Render(" to cancel")

	return s
}
