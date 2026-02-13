package model

import (
	"time"
)

// Attachment represents a file attached to an entry
type Attachment struct {
	ID        string    `json:"id"`
	EntryID   string    `json:"entry_id"`
	Filename  string    `json:"filename"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	Data      []byte    `json:"-"` // Not serialized to JSON, stored separately
	CreatedAt time.Time `json:"created_at"`
}

// SaveRecord represents a previous version of an entry
type SaveRecord struct {
	Content     string   `json:"content"`
	SavedAt     time.Time `json:"saved_at"`
	Attachments []string `json:"attachments,omitempty"` // Filenames at time of save
}

// Entry represents a single journal entry
type Entry struct {
	ID          string       `json:"id"`
	Date        string       `json:"date"`
	Content     string       `json:"content"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	History     []SaveRecord `json:"history,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Journal represents the collection of entries
type Journal struct {
	Entries []Entry `json:"entries"`
}

// JournalDB represents a journal database
type JournalDB struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Encrypted  bool      `json:"encrypted"`
	LastOpened time.Time `json:"last_opened"`
}

// Config represents the application configuration
type Config struct {
	// Legacy fields for backwards compatibility
	DatabasePath string `json:"database_path,omitempty"`
	Encrypted    bool   `json:"encrypted,omitempty"`

	// New fields
	Journals      []JournalDB `json:"journals,omitempty"`
	ActiveJournal string      `json:"active_journal,omitempty"` // Path of active journal
	Theme         string      `json:"theme,omitempty"`          // Color theme name
}

// Preview returns a truncated preview of the entry content
func (e Entry) Preview(maxLen int) string {
	content := e.Content
	if len(content) > maxLen {
		content = content[:maxLen] + "..."
	}
	return content
}

// AttachmentCount returns the number of attachments
func (e Entry) AttachmentCount() int {
	return len(e.Attachments)
}

// AttachmentFilenames returns a list of attachment filenames
func (e Entry) AttachmentFilenames() []string {
	names := make([]string, len(e.Attachments))
	for i, att := range e.Attachments {
		names[i] = att.Filename
	}
	return names
}
