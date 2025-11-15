package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Note represents a single note/comment associated with a file.
type Note struct {
	ID        string    `json:"id"`
	FileID    string    `json:"file_id"`
	Text      string    `json:"text"`
	Author    string    `json:"author,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NotesIndex persists file notes to disk and enables querying by file ID.
type NotesIndex struct {
	path string
	mu   sync.RWMutex
	data map[string][]Note // fileID -> []Note
}

// NewNotesIndex creates a new notes index and loads existing data from disk.
func NewNotesIndex(path string) (*NotesIndex, error) {
	idx := &NotesIndex{
		path: path,
		data: make(map[string][]Note),
	}
	if err := idx.load(); err != nil {
		return nil, err
	}
	return idx, nil
}

// load reads notes from disk into memory.
func (idx *NotesIndex) load() error {
	dir := filepath.Dir(idx.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	raw, err := os.ReadFile(idx.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil // File doesn't exist yet, start with empty index
	}
	if err != nil {
		return err
	}

	if len(raw) == 0 {
		return nil
	}

	var items []Note
	if err := json.Unmarshal(raw, &items); err != nil {
		return err
	}

	// Group notes by file ID
	for _, note := range items {
		idx.data[note.FileID] = append(idx.data[note.FileID], note)
	}

	return nil
}

// persistLocked writes all notes to disk. Must be called with lock held.
func (idx *NotesIndex) persistLocked() error {
	// Flatten all notes into a single array
	var allNotes []Note
	for _, notes := range idx.data {
		allNotes = append(allNotes, notes...)
	}

	tmp := idx.path + ".tmp"
	buf, err := json.MarshalIndent(allNotes, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, idx.path)
}

// GetNotes returns all notes for a given file ID.
func (idx *NotesIndex) GetNotes(fileID string) []Note {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	notes, exists := idx.data[fileID]
	if !exists {
		return []Note{}
	}

	// Return a copy to prevent external modification
	result := make([]Note, len(notes))
	copy(result, notes)
	return result
}

// AddNote adds a new note for a file.
func (idx *NotesIndex) AddNote(fileID, text, author string) (*Note, error) {
	if fileID == "" {
		return nil, errors.New("file_id is required")
	}
	if text == "" {
		return nil, errors.New("note text is required")
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	now := time.Now().UTC()
	note := Note{
		ID:        uuid.New().String(),
		FileID:    fileID,
		Text:      text,
		Author:    author,
		CreatedAt: now,
		UpdatedAt: now,
	}

	idx.data[fileID] = append(idx.data[fileID], note)
	if err := idx.persistLocked(); err != nil {
		// Rollback: remove the note we just added
		notes := idx.data[fileID]
		if len(notes) > 0 {
			idx.data[fileID] = notes[:len(notes)-1]
		}
		return nil, err
	}

	return &note, nil
}

// UpdateNote updates an existing note by ID.
func (idx *NotesIndex) UpdateNote(fileID, noteID, text string) (*Note, error) {
	if fileID == "" {
		return nil, errors.New("file_id is required")
	}
	if noteID == "" {
		return nil, errors.New("note_id is required")
	}
	if text == "" {
		return nil, errors.New("note text is required")
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	notes, exists := idx.data[fileID]
	if !exists {
		return nil, errors.New("file not found")
	}

	// Find the note
	for i := range notes {
		if notes[i].ID == noteID {
			notes[i].Text = text
			notes[i].UpdatedAt = time.Now().UTC()
			if err := idx.persistLocked(); err != nil {
				// Rollback: reload from disk
				_ = idx.load()
				return nil, err
			}
			result := notes[i]
			return &result, nil
		}
	}

	return nil, errors.New("note not found")
}

// DeleteNote removes a note by ID.
func (idx *NotesIndex) DeleteNote(fileID, noteID string) error {
	if fileID == "" {
		return errors.New("file_id is required")
	}
	if noteID == "" {
		return errors.New("note_id is required")
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	notes, exists := idx.data[fileID]
	if !exists {
		return errors.New("file not found")
	}

	// Find and remove the note
	for i, note := range notes {
		if note.ID == noteID {
			// Remove note by creating new slice without it
			idx.data[fileID] = append(notes[:i], notes[i+1:]...)
			if err := idx.persistLocked(); err != nil {
				// Rollback: reload from disk
				_ = idx.load()
				return err
			}
			return nil
		}
	}

	return errors.New("note not found")
}

// DeleteAllNotes removes all notes for a file (useful when file is deleted).
func (idx *NotesIndex) DeleteAllNotes(fileID string) error {
	if fileID == "" {
		return errors.New("file_id is required")
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.data[fileID]; !exists {
		return nil // No notes to delete, not an error
	}

	delete(idx.data, fileID)
	return idx.persistLocked()
}

