# Journal

A terminal-based journaling application with encryption support, file attachments, and version history.

Created by Max Xu

## Overview

Journal is a command-line journaling tool built with Go. It provides a full terminal user interface for creating, editing, and managing daily journal entries. All data is stored locally in a SQLite database file, with optional AES-256-GCM encryption. The application supports attaching files (images, PDFs, documents) directly within the database and maintains a complete version history of all changes.

## Features

### Core Functionality

- One journal entry per day (enforced by date validation)
- Rich text editing with multi-line support
- Entries sorted by date, newest first
- Full-text content preview in entry list

### Multiple Journals

- Support for multiple separate journal databases
- Journal selector on startup with most recently used highlighted
- Each journal can have independent encryption settings
- Journals stored at user-specified paths

### Encryption

- Optional AES-256-GCM encryption per journal
- Password-based key derivation using SHA-256
- Entire database file encrypted (entries, history, and attachments)
- Password required on each application launch for encrypted journals

### File Attachments

- Attach any file type (images, PDFs, documents, etc.)
- Files stored as binary blobs within the SQLite database
- Export attachments to any destination folder
- Attachment metadata (filename, size, MIME type) displayed in UI
- Adding attachments creates a new version in history

### Version History

- Automatic versioning on every save
- Complete content snapshots preserved
- Attachment state recorded with each version
- History sorted most recent to oldest
- View and navigate through all previous versions

### Themes

- Six built-in color themes: monochrome (default), default, ocean, forest, sunset, dracula
- Theme selection at application level (not per-journal)
- Live preview when switching themes
- Theme preference persisted across sessions

## Installation

### From GitHub Releases

Download the appropriate binary for your platform from the [Releases](https://github.com/maxxu/journal/releases) page.

#### macOS

After downloading, you must make the binary executable and allow it to run:

1. Open Terminal and navigate to the download location
2. Make the binary executable:
   ```bash
   chmod +x journal-darwin-arm64
   ```
   (Replace `journal-darwin-arm64` with the actual filename if different)

3. On first run, macOS will block the application. To allow it:
   - Open **System Settings** > **Privacy & Security**
   - Scroll down to the Security section
   - Click **Allow Anyway** next to the message about the blocked application
   - Run the application again and click **Open** when prompted

Alternatively, you can remove the quarantine attribute:
```bash
xattr -d com.apple.quarantine journal-darwin-arm64
```

#### Linux

Make the binary executable:
```bash
chmod +x journal-linux-amd64
```

#### Windows

No additional steps required. Run the `.exe` file directly.

## Building

### Prerequisites

- Go 1.21 or later

### Build Commands

The application is written in pure Go with no CGO dependencies, enabling cross-compilation to any supported platform.

#### Current Platform

```bash
go build -o journal .
```

#### Linux (AMD64)

```bash
GOOS=linux GOARCH=amd64 go build -o journal-linux-amd64 .
```

#### Linux (ARM64)

```bash
GOOS=linux GOARCH=arm64 go build -o journal-linux-arm64 .
```

#### macOS (Intel)

```bash
GOOS=darwin GOARCH=amd64 go build -o journal-darwin-amd64 .
```

#### macOS (Apple Silicon)

```bash
GOOS=darwin GOARCH=arm64 go build -o journal-darwin-arm64 .
```

#### Windows (AMD64)

```bash
GOOS=windows GOARCH=amd64 go build -o journal-windows-amd64.exe .
```

#### Windows (ARM64)

```bash
GOOS=windows GOARCH=arm64 go build -o journal-windows-arm64.exe .
```

### Disabling CGO Explicitly

For environments where CGO might be implicitly enabled:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o journal-linux-amd64 .
```

## Usage

### First Launch

Run the application:

```bash
./journal
```

On first launch, the setup wizard guides you through:

1. Choosing a storage location (default: `~/.journal/journal.db`)
2. Naming your journal
3. Optionally enabling encryption with a password

### Navigation

#### Journal Selector (startup screen when multiple journals exist)

| Key | Action |
|-----|--------|
| Up/Down, j/k | Navigate journal list |
| Left/Right, h/l | Change theme |
| Enter | Select journal |
| q | Quit |

#### Entry List

| Key | Action |
|-----|--------|
| Up/Down, j/k | Navigate entries |
| Enter | Edit selected entry |
| n | Create new entry (disabled if today has entry) |
| a | View/manage attachments |
| h | View version history |
| d | Delete entry |
| s | Settings |
| q | Quit |

#### Editor

| Key | Action |
|-----|--------|
| Tab | Switch between date and content fields |
| Ctrl+S | Save entry |
| Esc | Cancel and return to list |

#### Attachments

| Key | Action |
|-----|--------|
| Up/Down, j/k | Navigate attachments |
| a | Add new attachment |
| e | Export selected attachment |
| d | Delete selected attachment |
| Esc, q | Return to entry list |

#### History

| Key | Action |
|-----|--------|
| Up/Down, j/k | Navigate versions |
| Esc, q | Return to entry list |

### Global

| Key | Action |
|-----|--------|
| Ctrl+C | Force quit from any screen |

## File Structure

```
~/.journal/
    config.json      # Application configuration
    journal.db       # Default journal database (or encrypted blob)
```

### Configuration File

The `config.json` stores:

- List of known journals with paths and encryption status
- Last opened timestamps for each journal
- Active journal path
- Selected theme

### Database Schema

The SQLite database contains three tables:

- `entries`: Journal entries with id, date, content, timestamps
- `history`: Version history with content snapshots and attachment lists
- `attachments`: Binary file storage with metadata

## Libraries

### Direct Dependencies

| Library | Purpose |
|---------|---------|
| github.com/charmbracelet/bubbletea | Terminal UI framework (Elm architecture) |
| github.com/charmbracelet/bubbles | Pre-built UI components (text input, text area) |
| github.com/charmbracelet/lipgloss | Terminal styling and layout |
| github.com/google/uuid | UUID generation for entry and attachment IDs |
| modernc.org/sqlite | Pure Go SQLite implementation |

### Transitive Dependencies

| Library | Purpose |
|---------|---------|
| github.com/atotto/clipboard | Clipboard access |
| github.com/aymanbagabas/go-osc52/v2 | OSC52 terminal sequences |
| github.com/charmbracelet/colorprofile | Terminal color profile detection |
| github.com/charmbracelet/x/ansi | ANSI escape sequence handling |
| github.com/charmbracelet/x/term | Terminal utilities |
| github.com/lucasb-eyer/go-colorful | Color manipulation |
| github.com/mattn/go-isatty | TTY detection |
| github.com/mattn/go-runewidth | Unicode character width |
| github.com/muesli/termenv | Terminal environment detection |
| github.com/rivo/uniseg | Unicode segmentation |
| modernc.org/libc | C library implementation in Go |
| modernc.org/memory | Memory allocator for SQLite |

## Technical Details

### Encryption Implementation

- Key derivation: SHA-256 hash of password produces 32-byte key
- Cipher: AES-256-GCM (Galois/Counter Mode)
- Nonce: 12 bytes, randomly generated per encryption operation
- The entire SQLite database file is encrypted as a single blob
- Decryption creates a temporary file, operations performed, then re-encrypted

### Attachment Handling

- Files read entirely into memory during add operation
- Stored as BLOBs in the attachments table
- MIME type detection based on file extension
- No size limit enforced (limited by available memory)
- Attachment data not loaded into memory when viewing entry list (only metadata)

### Version History

- Created automatically when content changes on save
- Created when attachments are added
- Stores complete content snapshot (not diffs)
- Attachment filenames recorded with each version (not file contents)
- History records include timestamp of the save operation

## Quirks and Limitations

### One Entry Per Day

The application enforces exactly one entry per calendar day. The "new entry" option is disabled when an entry for the current date exists. Entries can be backdated by editing the date field.

### Date Format

Dates must be entered in YYYY-MM-DD format. The editor does not validate date format on input, but duplicate dates are rejected on save.

### Encryption Caveats

- Encrypted journals require the password on every launch
- No password recovery mechanism exists
- Incorrect password shows "Invalid password" error
- Temporary decrypted database files are created during operations

### Attachment Storage

- Attachments are stored inside the database file, increasing its size
- Deleted attachments free space only after SQLite vacuum (not automatic)
- Large attachments may cause slower save operations for encrypted journals
- Attachment history records only filenames, not file contents

### Theme Persistence

- Theme selection is global across all journals
- Theme changes take effect immediately (live preview)
- Theme is saved when selecting a journal or creating a new one

### Terminal Compatibility

- Requires a terminal with ANSI color support
- Some themes may not display correctly on terminals with limited color palettes
- Window resizing is handled but may cause momentary display artifacts

### Database Migrations

- Schema changes add columns with ALTER TABLE when possible
- Old databases are automatically migrated on first open
- No downgrade path exists for database schema changes

## License

MIT License
