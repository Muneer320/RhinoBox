package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// DeleteFileRequest represents the body for batch deletion requests.
type DeleteFileRequest struct {
	Hashes []string `json:"hashes"`
	Soft   bool     `json:"soft"`
}

// DeleteFileResponse represents the response for deletion operations.
type DeleteFileResponse struct {
	Hash           string `json:"hash"`
	Success        bool   `json:"success"`
	SpaceReclaimed int64  `json:"space_reclaimed,omitempty"`
	Error          string `json:"error,omitempty"`
}

// BatchDeleteResponse represents the response for batch deletion.
type BatchDeleteResponse struct {
	Results        []DeleteFileResponse `json:"results"`
	TotalDeleted   int                  `json:"total_deleted"`
	TotalFailed    int                  `json:"total_failed"`
	SpaceReclaimed int64                `json:"space_reclaimed"`
}

// handleDeleteFile handles DELETE /files/{hash} with soft/hard delete option.
func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if hash == "" {
		httpError(w, http.StatusBadRequest, "missing file hash")
		return
	}

	// Check if soft delete is requested (default: true)
	softDelete := true
	if softParam := r.URL.Query().Get("soft"); softParam != "" {
		if parsed, err := strconv.ParseBool(softParam); err == nil {
			softDelete = parsed
		}
	}

	var result *DeleteFileResponse
	var err error

	if softDelete {
		storageResult, deleteErr := s.storage.SoftDeleteFile(hash)
		err = deleteErr
		result = &DeleteFileResponse{
			Hash:    storageResult.Hash,
			Success: storageResult.Success,
			Error:   storageResult.Error,
		}
	} else {
		storageResult, deleteErr := s.storage.HardDeleteFile(hash)
		err = deleteErr
		result = &DeleteFileResponse{
			Hash:           storageResult.Hash,
			Success:        storageResult.Success,
			SpaceReclaimed: storageResult.SpaceReclaimed,
			Error:          storageResult.Error,
		}
	}

	// Log the deletion operation
	logRecord := map[string]any{
		"operation":       "delete_file",
		"hash":            hash,
		"soft_delete":     softDelete,
		"success":         result.Success,
		"space_reclaimed": result.SpaceReclaimed,
		"deleted_at":      time.Now().UTC().Format(time.RFC3339),
	}
	if err != nil {
		logRecord["error"] = err.Error()
	}

	if _, logErr := s.storage.AppendNDJSON(filepath.Join("audit", "deletion_log.ndjson"), []map[string]any{logRecord}); logErr != nil {
		s.logger.Warn("failed to append deletion log", slog.Any("err", logErr))
	}

	if err != nil {
		httpError(w, http.StatusBadRequest, result.Error)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleRestoreFile handles POST /files/{hash}/restore.
func (s *Server) handleRestoreFile(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if hash == "" {
		httpError(w, http.StatusBadRequest, "missing file hash")
		return
	}

	storageResult, err := s.storage.RestoreFile(hash)
	result := &DeleteFileResponse{
		Hash:    storageResult.Hash,
		Success: storageResult.Success,
		Error:   storageResult.Error,
	}

	// Log the restore operation
	logRecord := map[string]any{
		"operation":   "restore_file",
		"hash":        hash,
		"success":     result.Success,
		"restored_at": time.Now().UTC().Format(time.RFC3339),
	}
	if err != nil {
		logRecord["error"] = err.Error()
	}

	if _, logErr := s.storage.AppendNDJSON(filepath.Join("audit", "deletion_log.ndjson"), []map[string]any{logRecord}); logErr != nil {
		s.logger.Warn("failed to append deletion log", slog.Any("err", logErr))
	}

	if err != nil {
		httpError(w, http.StatusBadRequest, result.Error)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleBatchDelete handles DELETE /files/batch for batch deletions.
func (s *Server) handleBatchDelete(w http.ResponseWriter, r *http.Request) {
	var req DeleteFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if len(req.Hashes) == 0 {
		httpError(w, http.StatusBadRequest, "no hashes provided")
		return
	}

	storageResults := s.storage.BatchDelete(req.Hashes, req.Soft)

	// Convert storage results to API results
	results := make([]DeleteFileResponse, len(storageResults))
	totalDeleted := 0
	totalFailed := 0
	var spaceReclaimed int64

	for i, sr := range storageResults {
		results[i] = DeleteFileResponse{
			Hash:           sr.Hash,
			Success:        sr.Success,
			SpaceReclaimed: sr.SpaceReclaimed,
			Error:          sr.Error,
		}
		if sr.Success {
			totalDeleted++
			spaceReclaimed += sr.SpaceReclaimed
		} else {
			totalFailed++
		}
	}

	response := BatchDeleteResponse{
		Results:        results,
		TotalDeleted:   totalDeleted,
		TotalFailed:    totalFailed,
		SpaceReclaimed: spaceReclaimed,
	}

	// Log the batch deletion operation
	logRecord := map[string]any{
		"operation":       "batch_delete",
		"total_requested": len(req.Hashes),
		"total_deleted":   totalDeleted,
		"total_failed":    totalFailed,
		"soft_delete":     req.Soft,
		"space_reclaimed": spaceReclaimed,
		"deleted_at":      time.Now().UTC().Format(time.RFC3339),
	}

	if _, logErr := s.storage.AppendNDJSON(filepath.Join("audit", "deletion_log.ndjson"), []map[string]any{logRecord}); logErr != nil {
		s.logger.Warn("failed to append deletion log", slog.Any("err", logErr))
	}

	writeJSON(w, http.StatusOK, response)
}
