package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

// TestNotesEndToEnd tests the complete notes workflow
func TestNotesEndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		MaxUploadBytes: 10 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Step 1: Upload a file
	t.Log("Step 1: Uploading test file")
	testContent := []byte("This is a test file for notes testing.")
	body, contentType := createMultipartFormForNotes(t, "test.txt", testContent, "")

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("upload failed: status %d, body: %s", w.Code, w.Body.String())
	}

	var ingestResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &ingestResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	stored := ingestResp["stored"].([]any)
	if len(stored) == 0 {
		t.Fatal("no files stored")
	}

	fileData := stored[0].(map[string]any)
	fileID := fileData["hash"].(string)
	t.Logf("File uploaded with ID: %s", fileID)

	// Step 2: Get notes (should be empty)
	t.Log("Step 2: Getting notes (should be empty)")
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/files/%s/notes", fileID), nil)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("get notes failed: status %d, body: %s", w2.Code, w2.Body.String())
	}

	var notesResp map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &notesResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if notesResp["file_id"] != fileID {
		t.Errorf("expected file_id %s, got %s", fileID, notesResp["file_id"])
	}

	notes := notesResp["notes"].([]any)
	if len(notes) != 0 {
		t.Errorf("expected 0 notes initially, got %d", len(notes))
	}

	// Step 3: Add a note
	t.Log("Step 3: Adding a note")
	noteBody := bytes.NewBufferString(`{"text": "This is a test note", "author": "testuser"}`)
	req3 := httptest.NewRequest("POST", fmt.Sprintf("/files/%s/notes", fileID), noteBody)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, req3)

	if w3.Code != http.StatusCreated {
		t.Fatalf("add note failed: status %d, body: %s", w3.Code, w3.Body.String())
	}

	var addNoteResp map[string]any
	if err := json.Unmarshal(w3.Body.Bytes(), &addNoteResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	noteID := addNoteResp["id"].(string)
	if noteID == "" {
		t.Error("note ID should not be empty")
	}
	if addNoteResp["text"] != "This is a test note" {
		t.Errorf("expected text 'This is a test note', got '%s'", addNoteResp["text"])
	}
	if addNoteResp["author"] != "testuser" {
		t.Errorf("expected author 'testuser', got '%s'", addNoteResp["author"])
	}

	// Verify timestamps
	createdAtStr := addNoteResp["created_at"].(string)
	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		t.Fatalf("failed to parse created_at: %v", err)
	}
	if createdAt.IsZero() {
		t.Error("created_at should not be zero")
	}

	// Step 4: Get notes (should have 1 note)
	t.Log("Step 4: Getting notes (should have 1 note)")
	req4 := httptest.NewRequest("GET", fmt.Sprintf("/files/%s/notes", fileID), nil)
	w4 := httptest.NewRecorder()
	server.Router().ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("get notes failed: status %d", w4.Code)
	}

	var notesResp2 map[string]any
	json.Unmarshal(w4.Body.Bytes(), &notesResp2)
	notes2 := notesResp2["notes"].([]any)
	if len(notes2) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes2))
	}

	note := notes2[0].(map[string]any)
	if note["id"] != noteID {
		t.Errorf("expected note ID %s, got %s", noteID, note["id"])
	}

	// Step 5: Update the note
	t.Log("Step 5: Updating the note")
	updateBody := bytes.NewBufferString(`{"text": "This is an updated note"}`)
	req5 := httptest.NewRequest("PATCH", fmt.Sprintf("/files/%s/notes/%s", fileID, noteID), updateBody)
	req5.Header.Set("Content-Type", "application/json")
	w5 := httptest.NewRecorder()
	server.Router().ServeHTTP(w5, req5)

	if w5.Code != http.StatusOK {
		t.Fatalf("update note failed: status %d, body: %s", w5.Code, w5.Body.String())
	}

	var updateResp map[string]any
	json.Unmarshal(w5.Body.Bytes(), &updateResp)
	if updateResp["text"] != "This is an updated note" {
		t.Errorf("expected text 'This is an updated note', got '%s'", updateResp["text"])
	}

	// Verify updated_at changed
	updatedAtStr := updateResp["updated_at"].(string)
	updatedAt, err := time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		t.Fatalf("failed to parse updated_at: %v", err)
	}
	if !updatedAt.After(createdAt) {
		t.Error("updated_at should be after created_at")
	}

	// Step 6: Add multiple notes
	t.Log("Step 6: Adding multiple notes")
	for i := 0; i < 3; i++ {
		noteBody := bytes.NewBufferString(fmt.Sprintf(`{"text": "Note %d", "author": "user%d"}`, i+1, i+1))
		req := httptest.NewRequest("POST", fmt.Sprintf("/files/%s/notes", fileID), noteBody)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.Router().ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("add note %d failed: status %d", i+1, w.Code)
		}
	}

	// Step 7: Get all notes (should have 4 total: 1 updated + 3 new)
	req7 := httptest.NewRequest("GET", fmt.Sprintf("/files/%s/notes", fileID), nil)
	w7 := httptest.NewRecorder()
	server.Router().ServeHTTP(w7, req7)

	var notesResp7 map[string]any
	json.Unmarshal(w7.Body.Bytes(), &notesResp7)
	notes7 := notesResp7["notes"].([]any)
	if len(notes7) != 4 {
		t.Errorf("expected 4 notes, got %d", len(notes7))
	}

	count := notesResp7["count"].(float64)
	if count != 4 {
		t.Errorf("expected count 4, got %f", count)
	}

	// Step 8: Delete a note
	t.Log("Step 8: Deleting a note")
	req8 := httptest.NewRequest("DELETE", fmt.Sprintf("/files/%s/notes/%s", fileID, noteID), nil)
	w8 := httptest.NewRecorder()
	server.Router().ServeHTTP(w8, req8)

	if w8.Code != http.StatusOK {
		t.Fatalf("delete note failed: status %d, body: %s", w8.Code, w8.Body.String())
	}

	// Verify note is deleted
	req9 := httptest.NewRequest("GET", fmt.Sprintf("/files/%s/notes", fileID), nil)
	w9 := httptest.NewRecorder()
	server.Router().ServeHTTP(w9, req9)

	var notesResp9 map[string]any
	json.Unmarshal(w9.Body.Bytes(), &notesResp9)
	notes9 := notesResp9["notes"].([]any)
	if len(notes9) != 3 {
		t.Errorf("expected 3 notes after deletion, got %d", len(notes9))
	}

	// Step 9: Test error cases
	t.Log("Step 9: Testing error cases")

	// Try to get notes for non-existent file
	req10 := httptest.NewRequest("GET", "/files/nonexistent/notes", nil)
	w10 := httptest.NewRecorder()
	server.Router().ServeHTTP(w10, req10)
	if w10.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent file, got %d", w10.Code)
	}

	// Try to add note with empty text
	emptyBody := bytes.NewBufferString(`{"text": ""}`)
	req11 := httptest.NewRequest("POST", fmt.Sprintf("/files/%s/notes", fileID), emptyBody)
	req11.Header.Set("Content-Type", "application/json")
	w11 := httptest.NewRecorder()
	server.Router().ServeHTTP(w11, req11)
	if w11.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty text, got %d", w11.Code)
	}

	// Try to update non-existent note
	updateBody2 := bytes.NewBufferString(`{"text": "Updated"}`)
	req12 := httptest.NewRequest("PATCH", fmt.Sprintf("/files/%s/notes/nonexistent", fileID), updateBody2)
	req12.Header.Set("Content-Type", "application/json")
	w12 := httptest.NewRecorder()
	server.Router().ServeHTTP(w12, req12)
	if w12.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent note, got %d", w12.Code)
	}

	t.Log("All notes E2E tests passed!")
}

// Helper function to create multipart form for notes tests
func createMultipartFormForNotes(t *testing.T, filename string, content []byte, comment string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}

	if comment != "" {
		writer.WriteField("comment", comment)
	}

	writer.Close()
	return body, writer.FormDataContentType()
}

