package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	chi "github.com/go-chi/chi/v5"
	"log/slog"
)

// handleGetNotes handles GET /files/{file_id}/notes
func (s *Server) handleGetNotes(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	notes, err := s.fileService.GetNotes(fileID)
	if err != nil {
		if err.Error() == "file not found" || err.Error() == "file_id is required" {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get notes: %v", err))
		return
	}

	// Convert notes to response format
	response := make([]map[string]any, len(notes))
	for i, note := range notes {
		response[i] = map[string]any{
			"id":         note.ID,
			"file_id":    note.FileID,
			"text":       note.Text,
			"author":     note.Author,
			"created_at": note.CreatedAt.Format(time.RFC3339),
			"updated_at": note.UpdatedAt.Format(time.RFC3339),
		}
	}

	s.logger.Info("notes retrieved",
		slog.String("file_id", fileID),
		slog.Int("count", len(notes)),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"file_id": fileID,
		"notes":   response,
		"count":   len(notes),
	})
}

// handleAddNote handles POST /files/{file_id}/notes
func (s *Server) handleAddNote(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	var req struct {
		Text   string `json:"text"`
		Author string `json:"author,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Text == "" {
		httpError(w, http.StatusBadRequest, "text is required")
		return
	}

	note, err := s.fileService.AddNote(fileID, req.Text, req.Author)
	if err != nil {
		if err.Error() == "file not found" || err.Error() == "file_id is required" {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
		if err.Error() == "note text is required" {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to add note: %v", err))
		return
	}

	s.logger.Info("note added",
		slog.String("file_id", fileID),
		slog.String("note_id", note.ID),
	)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         note.ID,
		"file_id":   note.FileID,
		"text":      note.Text,
		"author":     note.Author,
		"created_at": note.CreatedAt.Format(time.RFC3339),
		"updated_at": note.UpdatedAt.Format(time.RFC3339),
	})
}

// handleUpdateNote handles PATCH /files/{file_id}/notes/{note_id}
func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	noteID := chi.URLParam(r, "note_id")
	if noteID == "" {
		httpError(w, http.StatusBadRequest, "note_id is required")
		return
	}

	var req struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Text == "" {
		httpError(w, http.StatusBadRequest, "text is required")
		return
	}

	note, err := s.fileService.UpdateNote(fileID, noteID, req.Text)
	if err != nil {
		if err.Error() == "file not found" || err.Error() == "file_id is required" {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
		if err.Error() == "note not found" || err.Error() == "note_id is required" {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
		if err.Error() == "note text is required" {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update note: %v", err))
		return
	}

	s.logger.Info("note updated",
		slog.String("file_id", fileID),
		slog.String("note_id", noteID),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         note.ID,
		"file_id":   note.FileID,
		"text":      note.Text,
		"author":     note.Author,
		"created_at": note.CreatedAt.Format(time.RFC3339),
		"updated_at": note.UpdatedAt.Format(time.RFC3339),
	})
}

// handleDeleteNote handles DELETE /files/{file_id}/notes/{note_id}
func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	noteID := chi.URLParam(r, "note_id")
	if noteID == "" {
		httpError(w, http.StatusBadRequest, "note_id is required")
		return
	}

	err := s.fileService.DeleteNote(fileID, noteID)
	if err != nil {
		if err.Error() == "file not found" || err.Error() == "file_id is required" {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
		if err.Error() == "note not found" || err.Error() == "note_id is required" {
			httpError(w, http.StatusNotFound, err.Error())
			return
		}
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete note: %v", err))
		return
	}

	s.logger.Info("note deleted",
		slog.String("file_id", fileID),
		slog.String("note_id", noteID),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "note deleted successfully",
		"file_id": fileID,
		"note_id": noteID,
	})
}

