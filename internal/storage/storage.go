package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"journal/internal/model"

	_ "modernc.org/sqlite"
)

const (
	DefaultConfigDir  = ".journal"
	DefaultConfigFile = "config.json"
	DefaultDBFile     = "journal.db"
)

var ErrInvalidPassword = errors.New("invalid password")

// ExpandPath expands ~ to the user's home directory
func ExpandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// GetConfigPath returns the full path to the config file
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DefaultConfigDir, DefaultConfigFile), nil
}

// GetDefaultDBPath returns the default database path
func GetDefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DefaultConfigDir, DefaultDBFile), nil
}

// ConfigExists checks if the config file exists
func ConfigExists() (bool, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(configPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// LoadConfig loads the configuration from disk
func LoadConfig() (*model.Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config model.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig saves the configuration to disk
func SaveConfig(config *model.Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// deriveKey derives a 32-byte key from a password using SHA-256
func deriveKey(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}

// encrypt encrypts data using AES-GCM
func encrypt(data []byte, password string) ([]byte, error) {
	key := deriveKey(password)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// decrypt decrypts data using AES-GCM
func decrypt(data []byte, password string) ([]byte, error) {
	key := deriveKey(password)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrInvalidPassword
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrInvalidPassword
	}

	return plaintext, nil
}

// Database operations

func openDB(path string) (*sql.DB, error) {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(expandedPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", expandedPath)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS entries (
		id TEXT PRIMARY KEY,
		date TEXT NOT NULL UNIQUE,
		content TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		entry_id TEXT NOT NULL,
		content TEXT NOT NULL,
		saved_at DATETIME NOT NULL,
		attachment_names TEXT DEFAULT '',
		FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS attachments (
		id TEXT PRIMARY KEY,
		entry_id TEXT NOT NULL,
		filename TEXT NOT NULL,
		mime_type TEXT NOT NULL,
		size INTEGER NOT NULL,
		data BLOB NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_entries_date ON entries(date);
	CREATE INDEX IF NOT EXISTS idx_history_entry ON history(entry_id);
	CREATE INDEX IF NOT EXISTS idx_attachments_entry ON attachments(entry_id);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: add attachment_names column if it doesn't exist
	_, _ = db.Exec(`ALTER TABLE history ADD COLUMN attachment_names TEXT DEFAULT ''`)

	return nil
}

// LoadJournal loads the journal from a SQLite database
func LoadJournal(path string) (*model.Journal, error) {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return &model.Journal{Entries: []model.Entry{}}, nil
	}

	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return loadJournalFromDB(db)
}

func loadJournalFromDB(db *sql.DB) (*model.Journal, error) {
	journal := &model.Journal{Entries: []model.Entry{}}

	rows, err := db.Query(`SELECT id, date, content, created_at, updated_at FROM entries ORDER BY date DESC`)
	if err != nil {
		return journal, nil // Table might not exist yet
	}
	defer rows.Close()

	for rows.Next() {
		var entry model.Entry
		if err := rows.Scan(&entry.ID, &entry.Date, &entry.Content, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, err
		}

		// Load history for this entry
		historyRows, err := db.Query(`SELECT content, saved_at, COALESCE(attachment_names, '') FROM history WHERE entry_id = ? ORDER BY saved_at DESC`, entry.ID)
		if err == nil {
			for historyRows.Next() {
				var record model.SaveRecord
				var attachmentNames string
				if err := historyRows.Scan(&record.Content, &record.SavedAt, &attachmentNames); err == nil {
					if attachmentNames != "" {
						record.Attachments = strings.Split(attachmentNames, "|")
					}
					entry.History = append(entry.History, record)
				}
			}
			historyRows.Close()
		}

		// Load attachments metadata (not data) for this entry
		attachRows, err := db.Query(`SELECT id, filename, mime_type, size, created_at FROM attachments WHERE entry_id = ?`, entry.ID)
		if err == nil {
			for attachRows.Next() {
				var att model.Attachment
				att.EntryID = entry.ID
				if err := attachRows.Scan(&att.ID, &att.Filename, &att.MimeType, &att.Size, &att.CreatedAt); err == nil {
					entry.Attachments = append(entry.Attachments, att)
				}
			}
			attachRows.Close()
		}

		journal.Entries = append(journal.Entries, entry)
	}

	return journal, nil
}

// SaveJournal saves the journal to a SQLite database
func SaveJournal(journal *model.Journal, path string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := initSchema(db); err != nil {
		return err
	}

	return saveJournalToDB(db, journal)
}

func saveJournalToDB(db *sql.DB, journal *model.Journal) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, entry := range journal.Entries {
		_, err := tx.Exec(`
			INSERT OR REPLACE INTO entries (id, date, content, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, entry.ID, entry.Date, entry.Content, entry.CreatedAt, entry.UpdatedAt)
		if err != nil {
			return err
		}

		// Save history
		for _, record := range entry.History {
			// Check if this history record already exists
			var count int
			tx.QueryRow(`SELECT COUNT(*) FROM history WHERE entry_id = ? AND saved_at = ?`,
				entry.ID, record.SavedAt).Scan(&count)
			if count == 0 {
				attachmentNames := strings.Join(record.Attachments, "|")
				_, err := tx.Exec(`INSERT INTO history (entry_id, content, saved_at, attachment_names) VALUES (?, ?, ?, ?)`,
					entry.ID, record.Content, record.SavedAt, attachmentNames)
				if err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

// DeleteEntry deletes an entry and its attachments from the database
func DeleteEntry(path string, entryID string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete history
	_, err = tx.Exec(`DELETE FROM history WHERE entry_id = ?`, entryID)
	if err != nil {
		return err
	}

	// Delete attachments
	_, err = tx.Exec(`DELETE FROM attachments WHERE entry_id = ?`, entryID)
	if err != nil {
		return err
	}

	// Delete entry
	_, err = tx.Exec(`DELETE FROM entries WHERE id = ?`, entryID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// History operations

// AddHistoryRecord adds a history record for an entry
func AddHistoryRecord(path string, entryID string, record model.SaveRecord, password string) error {
	if password != "" {
		return addHistoryRecordEncrypted(path, entryID, record, password)
	}

	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := initSchema(db); err != nil {
		return err
	}

	attachmentNames := strings.Join(record.Attachments, "|")
	_, err = db.Exec(`INSERT INTO history (entry_id, content, saved_at, attachment_names) VALUES (?, ?, ?, ?)`,
		entryID, record.Content, record.SavedAt, attachmentNames)

	return err
}

func addHistoryRecordEncrypted(path string, entryID string, record model.SaveRecord, password string) error {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return err
	}

	encryptedData, err := os.ReadFile(expandedPath)
	if err != nil {
		return err
	}

	decryptedData, err := decrypt(encryptedData, password)
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp("", "journal-*.db")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(decryptedData); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return err
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return err
	}

	attachmentNames := strings.Join(record.Attachments, "|")
	_, err = db.Exec(`INSERT INTO history (entry_id, content, saved_at, attachment_names) VALUES (?, ?, ?, ?)`,
		entryID, record.Content, record.SavedAt, attachmentNames)
	db.Close()

	if err != nil {
		return err
	}

	// Re-encrypt and save
	sqliteData, err := os.ReadFile(tmpPath)
	if err != nil {
		return err
	}

	encryptedData, err = encrypt(sqliteData, password)
	if err != nil {
		return err
	}

	return os.WriteFile(expandedPath, encryptedData, 0644)
}

// Attachment operations

// AddAttachment adds an attachment to an entry
func AddAttachment(path string, attachment *model.Attachment) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := initSchema(db); err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT INTO attachments (id, entry_id, filename, mime_type, size, data, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, attachment.ID, attachment.EntryID, attachment.Filename, attachment.MimeType,
		attachment.Size, attachment.Data, attachment.CreatedAt)

	return err
}

// GetAttachment retrieves an attachment with its data
func GetAttachment(path string, attachmentID string) (*model.Attachment, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var att model.Attachment
	err = db.QueryRow(`
		SELECT id, entry_id, filename, mime_type, size, data, created_at
		FROM attachments WHERE id = ?
	`, attachmentID).Scan(&att.ID, &att.EntryID, &att.Filename, &att.MimeType,
		&att.Size, &att.Data, &att.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &att, nil
}

// DeleteAttachment deletes an attachment
func DeleteAttachment(path string, attachmentID string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`DELETE FROM attachments WHERE id = ?`, attachmentID)
	return err
}

// GetEntryAttachments gets all attachments for an entry (with data)
func GetEntryAttachments(path string, entryID string) ([]model.Attachment, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT id, entry_id, filename, mime_type, size, data, created_at
		FROM attachments WHERE entry_id = ?
	`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []model.Attachment
	for rows.Next() {
		var att model.Attachment
		if err := rows.Scan(&att.ID, &att.EntryID, &att.Filename, &att.MimeType,
			&att.Size, &att.Data, &att.CreatedAt); err != nil {
			return nil, err
		}
		attachments = append(attachments, att)
	}

	return attachments, nil
}

// ExportAttachment exports an attachment to a file
func ExportAttachment(dbPath string, attachmentID string, destPath string) error {
	att, err := GetAttachment(dbPath, attachmentID)
	if err != nil {
		return err
	}

	expandedDest, err := ExpandPath(destPath)
	if err != nil {
		return err
	}

	// If destPath is a directory, use the original filename
	info, err := os.Stat(expandedDest)
	if err == nil && info.IsDir() {
		expandedDest = filepath.Join(expandedDest, att.Filename)
	}

	return os.WriteFile(expandedDest, att.Data, 0644)
}

// Encrypted database operations
// For encrypted databases, we encrypt the entire SQLite file

// LoadJournalEncrypted loads an encrypted journal
func LoadJournalEncrypted(path string, password string) (*model.Journal, error) {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return &model.Journal{Entries: []model.Entry{}}, nil
	}

	// Read encrypted file
	encryptedData, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, err
	}

	if len(encryptedData) == 0 {
		return &model.Journal{Entries: []model.Entry{}}, nil
	}

	// Decrypt to temporary file
	decryptedData, err := decrypt(encryptedData, password)
	if err != nil {
		return nil, err
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "journal-*.db")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(decryptedData); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()

	// Load from temp SQLite file
	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return loadJournalFromDB(db)
}

// SaveJournalEncrypted saves the journal encrypted
func SaveJournalEncrypted(journal *model.Journal, path string, password string) error {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(expandedPath), 0755); err != nil {
		return err
	}

	// Create temp SQLite file
	tmpFile, err := os.CreateTemp("", "journal-*.db")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Save to temp SQLite file
	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return err
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return err
	}

	if err := saveJournalToDB(db, journal); err != nil {
		db.Close()
		return err
	}
	db.Close()

	// Read the SQLite file
	sqliteData, err := os.ReadFile(tmpPath)
	if err != nil {
		return err
	}

	// Encrypt
	encryptedData, err := encrypt(sqliteData, password)
	if err != nil {
		return err
	}

	return os.WriteFile(expandedPath, encryptedData, 0644)
}

// AddAttachmentEncrypted adds an attachment to an encrypted journal
func AddAttachmentEncrypted(path string, password string, attachment *model.Attachment) error {
	journal, err := LoadJournalEncrypted(path, password)
	if err != nil {
		return err
	}

	// Find the entry and add attachment
	for i := range journal.Entries {
		if journal.Entries[i].ID == attachment.EntryID {
			journal.Entries[i].Attachments = append(journal.Entries[i].Attachments, *attachment)
			break
		}
	}

	// For encrypted, we need to handle attachments differently
	// We'll save the attachment data directly in a temp db then encrypt
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return err
	}

	// Decrypt existing data to temp file
	var tmpPath string
	if _, err := os.Stat(expandedPath); err == nil {
		encryptedData, err := os.ReadFile(expandedPath)
		if err != nil {
			return err
		}

		if len(encryptedData) > 0 {
			decryptedData, err := decrypt(encryptedData, password)
			if err != nil {
				return err
			}

			tmpFile, err := os.CreateTemp("", "journal-*.db")
			if err != nil {
				return err
			}
			tmpPath = tmpFile.Name()
			defer os.Remove(tmpPath)

			if _, err := tmpFile.Write(decryptedData); err != nil {
				tmpFile.Close()
				return err
			}
			tmpFile.Close()
		}
	}

	if tmpPath == "" {
		tmpFile, err := os.CreateTemp("", "journal-*.db")
		if err != nil {
			return err
		}
		tmpPath = tmpFile.Name()
		tmpFile.Close()
		defer os.Remove(tmpPath)
	}

	// Open temp db and add attachment
	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return err
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return err
	}

	_, err = db.Exec(`
		INSERT INTO attachments (id, entry_id, filename, mime_type, size, data, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, attachment.ID, attachment.EntryID, attachment.Filename, attachment.MimeType,
		attachment.Size, attachment.Data, attachment.CreatedAt)
	db.Close()

	if err != nil {
		return err
	}

	// Re-encrypt and save
	sqliteData, err := os.ReadFile(tmpPath)
	if err != nil {
		return err
	}

	encryptedData, err := encrypt(sqliteData, password)
	if err != nil {
		return err
	}

	return os.WriteFile(expandedPath, encryptedData, 0644)
}

// GetAttachmentEncrypted retrieves an attachment from an encrypted journal
func GetAttachmentEncrypted(path string, password string, attachmentID string) (*model.Attachment, error) {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return nil, err
	}

	encryptedData, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, err
	}

	decryptedData, err := decrypt(encryptedData, password)
	if err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp("", "journal-*.db")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(decryptedData); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()

	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var att model.Attachment
	err = db.QueryRow(`
		SELECT id, entry_id, filename, mime_type, size, data, created_at
		FROM attachments WHERE id = ?
	`, attachmentID).Scan(&att.ID, &att.EntryID, &att.Filename, &att.MimeType,
		&att.Size, &att.Data, &att.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &att, nil
}

// ExportAttachmentEncrypted exports an attachment from an encrypted journal
func ExportAttachmentEncrypted(dbPath string, password string, attachmentID string, destPath string) error {
	att, err := GetAttachmentEncrypted(dbPath, password, attachmentID)
	if err != nil {
		return err
	}

	expandedDest, err := ExpandPath(destPath)
	if err != nil {
		return err
	}

	info, err := os.Stat(expandedDest)
	if err == nil && info.IsDir() {
		expandedDest = filepath.Join(expandedDest, att.Filename)
	}

	return os.WriteFile(expandedDest, att.Data, 0644)
}

// DeleteAttachmentEncrypted deletes an attachment from an encrypted journal
func DeleteAttachmentEncrypted(path string, password string, attachmentID string) error {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return err
	}

	encryptedData, err := os.ReadFile(expandedPath)
	if err != nil {
		return err
	}

	decryptedData, err := decrypt(encryptedData, password)
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp("", "journal-*.db")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(decryptedData); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return err
	}

	_, err = db.Exec(`DELETE FROM attachments WHERE id = ?`, attachmentID)
	db.Close()

	if err != nil {
		return err
	}

	// Re-encrypt and save
	sqliteData, err := os.ReadFile(tmpPath)
	if err != nil {
		return err
	}

	encryptedData, err = encrypt(sqliteData, password)
	if err != nil {
		return err
	}

	return os.WriteFile(expandedPath, encryptedData, 0644)
}

// CreateEmptyJournal creates an empty journal database
func CreateEmptyJournal(path string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()

	return initSchema(db)
}

// CreateEmptyJournalEncrypted creates an empty encrypted journal
func CreateEmptyJournalEncrypted(path string, password string) error {
	journal := &model.Journal{Entries: []model.Entry{}}
	return SaveJournalEncrypted(journal, path, password)
}

// MigrateJournal copies journal data from old path to new path
func MigrateJournal(oldPath, newPath string) error {
	journal, err := LoadJournal(oldPath)
	if err != nil {
		return err
	}
	return SaveJournal(journal, newPath)
}

// MigrateJournalEncrypted copies encrypted journal data
func MigrateJournalEncrypted(oldPath, newPath string, password string) error {
	journal, err := LoadJournalEncrypted(oldPath, password)
	if err != nil {
		return err
	}
	return SaveJournalEncrypted(journal, newPath, password)
}

// MigrateConfigToNewFormat migrates old config format to new format
func MigrateConfigToNewFormat(config *model.Config) bool {
	if config.DatabasePath != "" && len(config.Journals) == 0 {
		config.Journals = []model.JournalDB{
			{
				Name:      "Default Journal",
				Path:      config.DatabasePath,
				Encrypted: config.Encrypted,
			},
		}
		config.ActiveJournal = config.DatabasePath
		config.DatabasePath = ""
		config.Encrypted = false
		return true
	}
	return false
}

// GetSortedJournals returns journals sorted by last opened (most recent first)
func GetSortedJournals(config *model.Config) []model.JournalDB {
	journals := make([]model.JournalDB, len(config.Journals))
	copy(journals, config.Journals)

	for i := 0; i < len(journals)-1; i++ {
		for j := i + 1; j < len(journals); j++ {
			if journals[j].LastOpened.After(journals[i].LastOpened) {
				journals[i], journals[j] = journals[j], journals[i]
			}
		}
	}

	return journals
}

// AddJournal adds a new journal to the config
func AddJournal(config *model.Config, name, path string, encrypted bool) {
	config.Journals = append(config.Journals, model.JournalDB{
		Name:      name,
		Path:      path,
		Encrypted: encrypted,
	})
}

// FindJournal finds a journal by path
func FindJournal(config *model.Config, path string) *model.JournalDB {
	for i := range config.Journals {
		if config.Journals[i].Path == path {
			return &config.Journals[i]
		}
	}
	return nil
}

// UpdateJournalLastOpened updates the last opened time for a journal
func UpdateJournalLastOpened(config *model.Config, path string, t time.Time) {
	for i := range config.Journals {
		if config.Journals[i].Path == path {
			config.Journals[i].LastOpened = t
			break
		}
	}
}

// DetectMimeType returns a mime type based on file extension
func DetectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeTypes := map[string]string{
		".pdf":  "application/pdf",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".txt":  "text/plain",
		".md":   "text/markdown",
		".json": "application/json",
		".xml":  "application/xml",
		".zip":  "application/zip",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// FormatFileSize formats bytes as human readable string
func FormatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
