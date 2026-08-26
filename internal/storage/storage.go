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
	"strconv"
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
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
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

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return writeFileAtomic(configPath, data, 0600)
}

// writeFileAtomic writes data through a temporary file in the same directory
// and renames it into place, so an interrupted write cannot truncate or
// corrupt the existing file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	fail := func(err error) error {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}

	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
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

	if err := os.MkdirAll(filepath.Dir(expandedPath), 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", expandedPath)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// isMissingTable reports whether err is SQLite complaining about a table that
// has not been created yet, which is expected for a brand new database file.
func isMissingTable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table")
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
		if isMissingTable(err) {
			return journal, nil // Nothing has been stored yet
		}
		return nil, err
	}

	for rows.Next() {
		var entry model.Entry
		if err := rows.Scan(&entry.ID, &entry.Date, &entry.Content, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		journal.Entries = append(journal.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// Load history and attachments once the entry cursor is closed, so we never
	// hold two cursors open on the same connection.
	for i := range journal.Entries {
		if err := loadEntryHistory(db, &journal.Entries[i]); err != nil {
			return nil, err
		}
		if err := loadEntryAttachments(db, &journal.Entries[i]); err != nil {
			return nil, err
		}
	}

	return journal, nil
}

func loadEntryHistory(db *sql.DB, entry *model.Entry) error {
	rows, err := db.Query(`SELECT content, saved_at, COALESCE(attachment_names, '') FROM history WHERE entry_id = ? ORDER BY saved_at DESC`, entry.ID)
	if err != nil {
		if isMissingTable(err) {
			return nil
		}
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var record model.SaveRecord
		var attachmentNames string
		if err := rows.Scan(&record.Content, &record.SavedAt, &attachmentNames); err != nil {
			return err
		}
		if attachmentNames != "" {
			record.Attachments = strings.Split(attachmentNames, "|")
		}
		entry.History = append(entry.History, record)
	}

	return rows.Err()
}

func loadEntryAttachments(db *sql.DB, entry *model.Entry) error {
	rows, err := db.Query(`SELECT id, filename, mime_type, size, created_at FROM attachments WHERE entry_id = ?`, entry.ID)
	if err != nil {
		if isMissingTable(err) {
			return nil
		}
		return err
	}
	defer rows.Close()

	for rows.Next() {
		att := model.Attachment{EntryID: entry.ID}
		if err := rows.Scan(&att.ID, &att.Filename, &att.MimeType, &att.Size, &att.CreatedAt); err != nil {
			return err
		}
		entry.Attachments = append(entry.Attachments, att)
	}

	return rows.Err()
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

	if err := pruneDeletedEntries(tx, journal); err != nil {
		return err
	}

	for _, entry := range journal.Entries {
		_, err := tx.Exec(`
			INSERT OR REPLACE INTO entries (id, date, content, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, entry.ID, entry.Date, entry.Content, entry.CreatedAt, entry.UpdatedAt)
		if err != nil {
			return err
		}

		if len(entry.History) == 0 {
			continue
		}

		// Timestamps do not round-trip through SQLite byte for byte, so
		// comparing saved_at in SQL would re-insert every record on each save.
		// Compare the parsed values in Go instead.
		seen, err := existingHistoryKeys(tx, entry.ID)
		if err != nil {
			return err
		}

		for _, record := range entry.History {
			key := historyKey(record.SavedAt, record.Content)
			if seen[key] {
				continue
			}
			seen[key] = true

			attachmentNames := strings.Join(record.Attachments, "|")
			_, err := tx.Exec(`INSERT INTO history (entry_id, content, saved_at, attachment_names) VALUES (?, ?, ?, ?)`,
				entry.ID, record.Content, record.SavedAt, attachmentNames)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// pruneDeletedEntries removes rows for entries that are no longer part of the
// in-memory journal, along with their history and attachments.
func pruneDeletedEntries(tx *sql.Tx, journal *model.Journal) error {
	keep := make(map[string]bool, len(journal.Entries))
	for _, entry := range journal.Entries {
		keep[entry.ID] = true
	}

	rows, err := tx.Query(`SELECT id FROM entries`)
	if err != nil {
		if isMissingTable(err) {
			return nil
		}
		return err
	}

	var stale []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if !keep[id] {
			stale = append(stale, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, id := range stale {
		for _, stmt := range []string{
			`DELETE FROM history WHERE entry_id = ?`,
			`DELETE FROM attachments WHERE entry_id = ?`,
			`DELETE FROM entries WHERE id = ?`,
		} {
			if _, err := tx.Exec(stmt, id); err != nil {
				return err
			}
		}
	}

	return nil
}

// historyKey identifies a save record independently of how its timestamp is
// formatted on disk.
func historyKey(savedAt time.Time, content string) string {
	return strconv.FormatInt(savedAt.UTC().UnixMilli(), 10) + "\x00" + content
}

func existingHistoryKeys(tx *sql.Tx, entryID string) (map[string]bool, error) {
	keys := make(map[string]bool)

	rows, err := tx.Query(`SELECT content, saved_at FROM history WHERE entry_id = ?`, entryID)
	if err != nil {
		if isMissingTable(err) {
			return keys, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var content string
		var savedAt time.Time
		if err := rows.Scan(&content, &savedAt); err != nil {
			return nil, err
		}
		keys[historyKey(savedAt, content)] = true
	}

	return keys, rows.Err()
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
	return withEncryptedDB(path, password, true, func(db *sql.DB) error {
		attachmentNames := strings.Join(record.Attachments, "|")
		_, err := db.Exec(`INSERT INTO history (entry_id, content, saved_at, attachment_names) VALUES (?, ?, ?, ?)`,
			entryID, record.Content, record.SavedAt, attachmentNames)
		return err
	})
}

// decryptToTemp decrypts the journal at expandedPath into a temporary SQLite
// file. A missing or empty source yields an empty temp file. The caller owns
// the returned path and must remove it.
func decryptToTemp(expandedPath, password string) (string, error) {
	tmpFile, err := os.CreateTemp("", "journal-*.db")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()

	fail := func(err error) (string, error) {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", err
	}

	encryptedData, err := os.ReadFile(expandedPath)
	if err != nil && !os.IsNotExist(err) {
		return fail(err)
	}

	if len(encryptedData) > 0 {
		decryptedData, err := decrypt(encryptedData, password)
		if err != nil {
			return fail(err)
		}
		if _, err := tmpFile.Write(decryptedData); err != nil {
			return fail(err)
		}
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	return tmpPath, nil
}

// encryptFromTemp encrypts the SQLite file at tmpPath back over expandedPath.
func encryptFromTemp(tmpPath, expandedPath, password string) error {
	sqliteData, err := os.ReadFile(tmpPath)
	if err != nil {
		return err
	}

	encryptedData, err := encrypt(sqliteData, password)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(expandedPath), 0700); err != nil {
		return err
	}

	return writeFileAtomic(expandedPath, encryptedData, 0600)
}

// withEncryptedDB decrypts the journal at path, hands the decrypted database to
// fn, and re-encrypts the result when mutate is true. Working on the existing
// database (rather than a fresh one) is what keeps attachments alive across
// saves.
func withEncryptedDB(path, password string, mutate bool, fn func(db *sql.DB) error) error {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return err
	}

	tmpPath, err := decryptToTemp(expandedPath, password)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return err
	}

	if err := initSchema(db); err != nil {
		db.Close()
		return err
	}

	fnErr := fn(db)
	closeErr := db.Close()

	if fnErr != nil {
		return fnErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !mutate {
		return nil
	}

	return encryptFromTemp(tmpPath, expandedPath, password)
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

	_, err = db.Exec(insertAttachmentSQL, attachment.ID, attachment.EntryID,
		attachment.Filename, attachment.MimeType, attachment.Size,
		attachment.Data, attachment.CreatedAt)

	return err
}

const insertAttachmentSQL = `
	INSERT OR REPLACE INTO attachments (id, entry_id, filename, mime_type, size, data, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
`

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
		if isMissingTable(err) {
			return nil, nil
		}
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

	return attachments, rows.Err()
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

	var journal *model.Journal
	err = withEncryptedDB(path, password, false, func(db *sql.DB) error {
		var loadErr error
		journal, loadErr = loadJournalFromDB(db)
		return loadErr
	})
	if err != nil {
		return nil, err
	}

	return journal, nil
}

// SaveJournalEncrypted saves the journal encrypted
func SaveJournalEncrypted(journal *model.Journal, path string, password string) error {
	return withEncryptedDB(path, password, true, func(db *sql.DB) error {
		return saveJournalToDB(db, journal)
	})
}

// AddAttachmentEncrypted adds an attachment to an encrypted journal
func AddAttachmentEncrypted(path string, password string, attachment *model.Attachment) error {
	return withEncryptedDB(path, password, true, func(db *sql.DB) error {
		_, err := db.Exec(insertAttachmentSQL, attachment.ID, attachment.EntryID,
			attachment.Filename, attachment.MimeType, attachment.Size,
			attachment.Data, attachment.CreatedAt)
		return err
	})
}

// GetAttachmentEncrypted retrieves an attachment from an encrypted journal
func GetAttachmentEncrypted(path string, password string, attachmentID string) (*model.Attachment, error) {
	var att model.Attachment

	err := withEncryptedDB(path, password, false, func(db *sql.DB) error {
		return db.QueryRow(`
			SELECT id, entry_id, filename, mime_type, size, data, created_at
			FROM attachments WHERE id = ?
		`, attachmentID).Scan(&att.ID, &att.EntryID, &att.Filename, &att.MimeType,
			&att.Size, &att.Data, &att.CreatedAt)
	})
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
	return withEncryptedDB(path, password, true, func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM attachments WHERE id = ?`, attachmentID)
		return err
	})
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

	if err := SaveJournal(journal, newPath); err != nil {
		return err
	}

	// Attachment blobs are not part of the journal struct, so copy them across
	// explicitly or they are left behind at the old location.
	for _, entry := range journal.Entries {
		attachments, err := GetEntryAttachments(oldPath, entry.ID)
		if err != nil {
			return err
		}
		for i := range attachments {
			if err := AddAttachment(newPath, &attachments[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

// MigrateJournalEncrypted copies encrypted journal data
func MigrateJournalEncrypted(oldPath, newPath string, password string) error {
	journal, err := LoadJournalEncrypted(oldPath, password)
	if err != nil {
		return err
	}

	if err := SaveJournalEncrypted(journal, newPath, password); err != nil {
		return err
	}

	var attachments []model.Attachment
	err = withEncryptedDB(oldPath, password, false, func(db *sql.DB) error {
		var loadErr error
		attachments, loadErr = loadAllAttachments(db)
		return loadErr
	})
	if err != nil {
		return err
	}

	if len(attachments) == 0 {
		return nil
	}

	return withEncryptedDB(newPath, password, true, func(db *sql.DB) error {
		for _, att := range attachments {
			if _, err := db.Exec(insertAttachmentSQL, att.ID, att.EntryID,
				att.Filename, att.MimeType, att.Size, att.Data, att.CreatedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

func loadAllAttachments(db *sql.DB) ([]model.Attachment, error) {
	rows, err := db.Query(`SELECT id, entry_id, filename, mime_type, size, data, created_at FROM attachments`)
	if err != nil {
		if isMissingTable(err) {
			return nil, nil
		}
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

	return attachments, rows.Err()
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
