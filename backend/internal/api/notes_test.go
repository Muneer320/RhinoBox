package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

func setupNotesTestServerHelper(t *testing.T) (*Server, string) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:       tmpDir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return server, tmpDir
}

func uploadTestFileForNotes(t *testing.T, server *Server) string {
	// Upload a test file first
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("test content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to upload test file: status %d, body: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	stored := response["stored"].([]any)
	if len(stored) == 0 {
		t.Fatal("no files stored")
	}

	fileData := stored[0].(map[string]any)
	fileID := fileData["hash"].(string)
	return fileID
}

func TestGetNotes(t *testing.T) {
	server, _ := setupNotesTestServerHelper(t)
	fileID := uploadTestFileForNotes(t, server)

	// Get notes for file (should be empty initially)
	req := httptest.NewRequest("GET", "/files/"+fileID+"/notes", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["file_id"] != fileID {
		t.Errorf("expected file_id %s, got %s", fileID, response["file_id"])
	}

	notes := response["notes"].([]any)
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}

	count := response["count"].(float64)
	if count != 0 {
		t.Errorf("expected count 0, got %f", count)
	}
}

func TestAddNote(t *testing.T) {
	server, _ := setupNotesTestServerHelper(t)
	fileID := uploadTestFileForNotes(t, server)

	// Add a note
	body := bytes.NewBufferString(`{"text": "Test note", "author": "testuser"}`)
	req := httptest.NewRequest("POST", "/files/"+fileID+"/notes", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["id"] == nil {
		t.Error("response missing note id")
	}
	if response["text"] != "Test note" {
		t.Errorf("expected text 'Test note', got '%s'", response["text"])
	}
	if response["author"] != "testuser" {
		t.Errorf("expected author 'testuser', got '%s'", response["author"])
	}
	if response["file_id"] != fileID {
		t.Errorf("expected file_id %s, got %s", fileID, response["file_id"])
	}

	// Verify note appears in GET
	req2 := httptest.NewRequest("GET", "/files/"+fileID+"/notes", nil)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w2.Code)
	}

	var response2 map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &response2); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	notes := response2["notes"].([]any)
	if len(notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes))
	}

	note := notes[0].(map[string]any)
	if note["text"] != "Test note" {
		t.Errorf("expected text 'Test note', got '%s'", note["text"])
	}
}

func TestUpdateNote(t *testing.T) {
	server, _ := setupNotesTestServerHelper(t)
	fileID := uploadTestFileForNotes(t, server)

	// Add a note
	body1 := bytes.NewBufferString(`{"text": "Original note"}`)
	req1 := httptest.NewRequest("POST", "/files/"+fileID+"/notes", body1)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	server.Router().ServeHTTP(w1, req1)

	var addResponse map[string]any
	json.Unmarshal(w1.Body.Bytes(), &addResponse)
	noteID := addResponse["id"].(string)

	// Update the note
	body2 := bytes.NewBufferString(`{"text": "Updated note"}`)
	req2 := httptest.NewRequest("PATCH", "/files/"+fileID+"/notes/"+noteID, body2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w2.Code, w2.Body.String())
	}

	var updateResponse map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &updateResponse); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if updateResponse["text"] != "Updated note" {
		t.Errorf("expected text 'Updated note', got '%s'", updateResponse["text"])
	}

	// Verify update persisted
	req3 := httptest.NewRequest("GET", "/files/"+fileID+"/notes", nil)
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, req3)

	var getResponse map[string]any
	json.Unmarshal(w3.Body.Bytes(), &getResponse)
	notes := getResponse["notes"].([]any)
	note := notes[0].(map[string]any)
	if note["text"] != "Updated note" {
		t.Errorf("expected text 'Updated note', got '%s'", note["text"])
	}
}

func TestDeleteNote(t *testing.T) {
	server, _ := setupNotesTestServerHelper(t)
	fileID := uploadTestFileForNotes(t, server)

	// Add a note
	body1 := bytes.NewBufferString(`{"text": "Note to delete"}`)
	req1 := httptest.NewRequest("POST", "/files/"+fileID+"/notes", body1)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	server.Router().ServeHTTP(w1, req1)

	var addResponse map[string]any
	json.Unmarshal(w1.Body.Bytes(), &addResponse)
	noteID := addResponse["id"].(string)

	// Delete the note
	req2 := httptest.NewRequest("DELETE", "/files/"+fileID+"/notes/"+noteID, nil)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w2.Code, w2.Body.String())
	}

	// Verify note is deleted
	req3 := httptest.NewRequest("GET", "/files/"+fileID+"/notes", nil)
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, req3)

	var getResponse map[string]any
	json.Unmarshal(w3.Body.Bytes(), &getResponse)
	notes := getResponse["notes"].([]any)
	if len(notes) != 0 {
		t.Errorf("expected 0 notes after deletion, got %d", len(notes))
	}
}

func TestNotesValidation(t *testing.T) {
	server, _ := setupNotesTestServerHelper(t)
	fileID := uploadTestFileForNotes(t, server)

	// Test missing file_id
	req := httptest.NewRequest("GET", "/files//notes", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for missing file_id, got %d", w.Code)
	}

	// Test missing text in POST
	body := bytes.NewBufferString(`{}`)
	req2 := httptest.NewRequest("POST", "/files/"+fileID+"/notes", body)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing text, got %d", w2.Code)
	}

	// Test missing text in PATCH
	body3 := bytes.NewBufferString(`{}`)
	req3 := httptest.NewRequest("PATCH", "/files/"+fileID+"/notes/invalid", body3)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	server.Router().ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing text, got %d", w3.Code)
	}
}

func TestNotesFileNotFound(t *testing.T) {
	server, _ := setupNotesTestServerHelper(t)

	// Try to get notes for non-existent file
	req := httptest.NewRequest("GET", "/files/nonexistent/notes", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent file, got %d", w.Code)
	}

	// Try to add note to non-existent file
	body := bytes.NewBufferString(`{"text": "Test"}`)
	req2 := httptest.NewRequest("POST", "/files/nonexistent/notes", body)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent file, got %d", w2.Code)
	}
}

func TestNotesNoteNotFound(t *testing.T) {
	server, _ := setupNotesTestServerHelper(t)
	fileID := uploadTestFileForNotes(t, server)

	// Try to update non-existent note
	body := bytes.NewBufferString(`{"text": "Updated"}`)
	req := httptest.NewRequest("PATCH", "/files/"+fileID+"/notes/invalid-note-id", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent note, got %d", w.Code)
	}

	// Try to delete non-existent note
	req2 := httptest.NewRequest("DELETE", "/files/"+fileID+"/notes/invalid-note-id", nil)
	w2 := httptest.NewRecorder()
	server.Router().ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent note, got %d", w2.Code)
	}
}

func TestNotesMultipleNotes(t *testing.T) {
	server, _ := setupNotesTestServerHelper(t)
	fileID := uploadTestFileForNotes(t, server)

	// Add multiple notes
	for i := 0; i < 5; i++ {
		body := bytes.NewBufferString(`{"text": "Note ` + string(rune('0'+i)) + `"}`)
		req := httptest.NewRequest("POST", "/files/"+fileID+"/notes", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.Router().ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Errorf("failed to add note %d: status %d", i, w.Code)
		}
	}

	// Get all notes
	req := httptest.NewRequest("GET", "/files/"+fileID+"/notes", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	var response map[string]any
	json.Unmarshal(w.Body.Bytes(), &response)
	notes := response["notes"].([]any)
	if len(notes) != 5 {
		t.Errorf("expected 5 notes, got %d", len(notes))
	}

	count := response["count"].(float64)
	if count != 5 {
		t.Errorf("expected count 5, got %f", count)
	}
}

