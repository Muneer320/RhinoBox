package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNotesIndex(t *testing.T) {
	tmpDir := t.TempDir()
	notesPath := filepath.Join(tmpDir, "notes.json")

	// Create new index
	idx, err := NewNotesIndex(notesPath)
	if err != nil {
		t.Fatalf("failed to create notes index: %v", err)
	}

	// Test adding a note
	note, err := idx.AddNote("file1", "Test note", "user1")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}
	if note.ID == "" {
		t.Error("note ID should not be empty")
	}
	if note.FileID != "file1" {
		t.Errorf("expected file_id 'file1', got '%s'", note.FileID)
	}
	if note.Text != "Test note" {
		t.Errorf("expected text 'Test note', got '%s'", note.Text)
	}
	if note.Author != "user1" {
		t.Errorf("expected author 'user1', got '%s'", note.Author)
	}

	// Test getting notes
	notes := idx.GetNotes("file1")
	if len(notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Text != "Test note" {
		t.Errorf("expected text 'Test note', got '%s'", notes[0].Text)
	}

	// Test getting notes for non-existent file
	notes = idx.GetNotes("file2")
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}

	// Test updating a note
	updatedNote, err := idx.UpdateNote("file1", note.ID, "Updated note")
	if err != nil {
		t.Fatalf("failed to update note: %v", err)
	}
	if updatedNote.Text != "Updated note" {
		t.Errorf("expected text 'Updated note', got '%s'", updatedNote.Text)
	}
	if updatedNote.UpdatedAt.Before(updatedNote.CreatedAt) {
		t.Error("updated_at should be after created_at")
	}

	// Verify update persisted
	notes = idx.GetNotes("file1")
	if len(notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Text != "Updated note" {
		t.Errorf("expected text 'Updated note', got '%s'", notes[0].Text)
	}

	// Test deleting a note
	err = idx.DeleteNote("file1", note.ID)
	if err != nil {
		t.Fatalf("failed to delete note: %v", err)
	}

	// Verify deletion
	notes = idx.GetNotes("file1")
	if len(notes) != 0 {
		t.Errorf("expected 0 notes after deletion, got %d", len(notes))
	}
}

func TestNotesIndexPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	notesPath := filepath.Join(tmpDir, "notes.json")

	// Create index and add notes
	idx1, err := NewNotesIndex(notesPath)
	if err != nil {
		t.Fatalf("failed to create notes index: %v", err)
	}

	note1, err := idx1.AddNote("file1", "Note 1", "user1")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	note2, err := idx1.AddNote("file1", "Note 2", "user2")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	_, err = idx1.AddNote("file2", "Note 3", "user1")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	// Create new index instance (simulating server restart)
	idx2, err := NewNotesIndex(notesPath)
	if err != nil {
		t.Fatalf("failed to create new notes index: %v", err)
	}

	// Verify notes persisted
	notes := idx2.GetNotes("file1")
	if len(notes) != 2 {
		t.Errorf("expected 2 notes for file1, got %d", len(notes))
	}

	notes = idx2.GetNotes("file2")
	if len(notes) != 1 {
		t.Errorf("expected 1 note for file2, got %d", len(notes))
	}

	// Verify note IDs match
	found1 := false
	found2 := false
	for _, note := range idx2.GetNotes("file1") {
		if note.ID == note1.ID {
			found1 = true
			if note.Text != "Note 1" {
				t.Errorf("expected text 'Note 1', got '%s'", note.Text)
			}
		}
		if note.ID == note2.ID {
			found2 = true
			if note.Text != "Note 2" {
				t.Errorf("expected text 'Note 2', got '%s'", note.Text)
			}
		}
	}
	if !found1 {
		t.Error("note1 not found after reload")
	}
	if !found2 {
		t.Error("note2 not found after reload")
	}
}

func TestNotesIndexValidation(t *testing.T) {
	tmpDir := t.TempDir()
	notesPath := filepath.Join(tmpDir, "notes.json")

	idx, err := NewNotesIndex(notesPath)
	if err != nil {
		t.Fatalf("failed to create notes index: %v", err)
	}

	// Test empty file_id
	_, err = idx.AddNote("", "text", "author")
	if err == nil {
		t.Error("expected error for empty file_id")
	}

	// Test empty text
	_, err = idx.AddNote("file1", "", "author")
	if err == nil {
		t.Error("expected error for empty text")
	}

	// Test update with empty note_id
	_, err = idx.UpdateNote("file1", "", "text")
	if err == nil {
		t.Error("expected error for empty note_id")
	}

	// Test delete with empty note_id
	err = idx.DeleteNote("file1", "")
	if err == nil {
		t.Error("expected error for empty note_id")
	}
}

func TestNotesIndexDeleteAllNotes(t *testing.T) {
	tmpDir := t.TempDir()
	notesPath := filepath.Join(tmpDir, "notes.json")

	idx, err := NewNotesIndex(notesPath)
	if err != nil {
		t.Fatalf("failed to create notes index: %v", err)
	}

	// Add multiple notes
	_, err = idx.AddNote("file1", "Note 1", "user1")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	_, err = idx.AddNote("file1", "Note 2", "user2")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	_, err = idx.AddNote("file2", "Note 3", "user1")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	// Delete all notes for file1
	err = idx.DeleteAllNotes("file1")
	if err != nil {
		t.Fatalf("failed to delete all notes: %v", err)
	}

	// Verify file1 has no notes
	notes := idx.GetNotes("file1")
	if len(notes) != 0 {
		t.Errorf("expected 0 notes for file1, got %d", len(notes))
	}

	// Verify file2 still has notes
	notes = idx.GetNotes("file2")
	if len(notes) != 1 {
		t.Errorf("expected 1 note for file2, got %d", len(notes))
	}
}

func TestNotesIndexConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	notesPath := filepath.Join(tmpDir, "notes.json")

	idx, err := NewNotesIndex(notesPath)
	if err != nil {
		t.Fatalf("failed to create notes index: %v", err)
	}

	// Add initial note
	note, err := idx.AddNote("file1", "Initial note", "user1")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	// Simulate concurrent reads (should be safe)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			notes := idx.GetNotes("file1")
			if len(notes) != 1 {
				t.Errorf("expected 1 note, got %d", len(notes))
			}
			done <- true
		}()
	}

	// Wait for all reads
	for i := 0; i < 10; i++ {
		<-done
	}

	// Update note (write operation)
	_, err = idx.UpdateNote("file1", note.ID, "Updated")
	if err != nil {
		t.Fatalf("failed to update note: %v", err)
	}

	// Verify update
	notes := idx.GetNotes("file1")
	if len(notes) != 1 || notes[0].Text != "Updated" {
		t.Errorf("note not updated correctly")
	}
}

func TestNoteTimestamps(t *testing.T) {
	tmpDir := t.TempDir()
	notesPath := filepath.Join(tmpDir, "notes.json")

	idx, err := NewNotesIndex(notesPath)
	if err != nil {
		t.Fatalf("failed to create notes index: %v", err)
	}

	beforeCreate := time.Now().UTC()
	note, err := idx.AddNote("file1", "Test", "user1")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}
	afterCreate := time.Now().UTC()

	// Check created_at is within reasonable time
	if note.CreatedAt.Before(beforeCreate) || note.CreatedAt.After(afterCreate) {
		t.Errorf("created_at should be between %v and %v, got %v", beforeCreate, afterCreate, note.CreatedAt)
	}

	// Check updated_at equals created_at initially
	if !note.UpdatedAt.Equal(note.CreatedAt) {
		t.Errorf("updated_at should equal created_at initially, got created_at=%v, updated_at=%v", note.CreatedAt, note.UpdatedAt)
	}

	// Wait a bit and update
	time.Sleep(10 * time.Millisecond)
	beforeUpdate := time.Now().UTC()
	updatedNote, err := idx.UpdateNote("file1", note.ID, "Updated")
	if err != nil {
		t.Fatalf("failed to update note: %v", err)
	}
	afterUpdate := time.Now().UTC()

	// Check updated_at changed
	if !updatedNote.UpdatedAt.After(updatedNote.CreatedAt) {
		t.Errorf("updated_at should be after created_at after update")
	}
	if updatedNote.UpdatedAt.Before(beforeUpdate) || updatedNote.UpdatedAt.After(afterUpdate) {
		t.Errorf("updated_at should be between %v and %v, got %v", beforeUpdate, afterUpdate, updatedNote.UpdatedAt)
	}
}

func TestNotesIndexFileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	notesPath := filepath.Join(tmpDir, "notes.json")

	idx, err := NewNotesIndex(notesPath)
	if err != nil {
		t.Fatalf("failed to create notes index: %v", err)
	}

	// Should create file if it doesn't exist
	if _, err := os.Stat(notesPath); err == nil {
		t.Error("notes file should not exist yet")
	}

	// Add a note (should create the file)
	_, err = idx.AddNote("file1", "Test", "user1")
	if err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	// File should now exist
	if _, err := os.Stat(notesPath); err != nil {
		t.Errorf("notes file should exist after adding note: %v", err)
	}
}

