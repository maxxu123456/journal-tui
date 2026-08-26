package storage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"journal/internal/model"
	"journal/internal/storage"
)

func newEntry(id, date, content string) model.Entry {
	now := time.Now()
	return model.Entry{ID: id, Date: date, Content: content, CreatedAt: now, UpdatedAt: now}
}

// Saving an encrypted journal must not discard attachments already stored in it.
func TestEncryptedSavePreservesAttachments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enc.db")
	const password = "correct horse"

	journal := &model.Journal{Entries: []model.Entry{newEntry("e1", "2026-01-01", "hello")}}
	if err := storage.SaveJournalEncrypted(journal, path, password); err != nil {
		t.Fatal(err)
	}

	att := &model.Attachment{ID: "a1", EntryID: "e1", Filename: "note.txt",
		MimeType: "text/plain", Size: 3, Data: []byte("abc"), CreatedAt: time.Now()}
	if err := storage.AddAttachmentEncrypted(path, password, att); err != nil {
		t.Fatal(err)
	}

	loaded, err := storage.LoadJournalEncrypted(path, password)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Entries[0].Content = "edited"
	if err := storage.SaveJournalEncrypted(loaded, path, password); err != nil {
		t.Fatal(err)
	}

	after, err := storage.LoadJournalEncrypted(path, password)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(after.Entries[0].Attachments); got != 1 {
		t.Fatalf("attachments after re-save = %d, want 1", got)
	}

	got, err := storage.GetAttachmentEncrypted(path, password, "a1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Data) != "abc" {
		t.Errorf("attachment data = %q, want %q", got.Data, "abc")
	}
}

// Timestamps do not round-trip through SQLite byte for byte, so history dedup
// has to compare parsed values or every save duplicates the whole history.
func TestSaveDoesNotDuplicateHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.db")

	when := time.Now()
	entry := newEntry("e1", "2026-01-01", "v2")
	entry.History = []model.SaveRecord{{Content: "v1", SavedAt: when}}
	journal := &model.Journal{Entries: []model.Entry{entry}}

	for i := 0; i < 3; i++ {
		if err := storage.SaveJournal(journal, path); err != nil {
			t.Fatal(err)
		}
		var err error
		if journal, err = storage.LoadJournal(path); err != nil {
			t.Fatal(err)
		}
		if got := len(journal.Entries[0].History); got != 1 {
			t.Fatalf("history after %d save/load cycles = %d, want 1", i+1, got)
		}
	}
}

// A deleted entry must not come back when the journal is written out again.
func TestSaveRemovesDeletedEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.db")

	journal := &model.Journal{Entries: []model.Entry{
		newEntry("e1", "2026-01-01", "a"),
		newEntry("e2", "2026-01-02", "b"),
	}}
	if err := storage.SaveJournal(journal, path); err != nil {
		t.Fatal(err)
	}

	journal.Entries = journal.Entries[:1]
	if err := storage.SaveJournal(journal, path); err != nil {
		t.Fatal(err)
	}

	after, err := storage.LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(after.Entries); got != 1 {
		t.Errorf("entries after delete = %d, want 1", got)
	}
}

// Moving a journal must take its attachment blobs along.
func TestMigrateCopiesAttachments(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.db")
	newPath := filepath.Join(dir, "new.db")

	journal := &model.Journal{Entries: []model.Entry{newEntry("e1", "2026-01-01", "hello")}}
	if err := storage.SaveJournal(journal, oldPath); err != nil {
		t.Fatal(err)
	}
	att := &model.Attachment{ID: "a1", EntryID: "e1", Filename: "note.txt",
		MimeType: "text/plain", Size: 3, Data: []byte("abc"), CreatedAt: time.Now()}
	if err := storage.AddAttachment(oldPath, att); err != nil {
		t.Fatal(err)
	}

	if err := storage.MigrateJournal(oldPath, newPath); err != nil {
		t.Fatal(err)
	}

	moved, err := storage.GetAttachment(newPath, "a1")
	if err != nil {
		t.Fatalf("attachment missing after migration: %v", err)
	}
	if string(moved.Data) != "abc" {
		t.Errorf("migrated data = %q, want %q", moved.Data, "abc")
	}
}

// The journal and its config hold private writing; they must not be world readable.
func TestFilesAreNotWorldReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enc.db")
	if err := storage.CreateEmptyJournalEncrypted(path, "pw"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("encrypted journal mode = %v, want -rw-------", perm)
	}
}

func TestWrongPasswordIsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enc.db")
	if err := storage.CreateEmptyJournalEncrypted(path, "right"); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.LoadJournalEncrypted(path, "wrong"); err != storage.ErrInvalidPassword {
		t.Errorf("err = %v, want ErrInvalidPassword", err)
	}
}
