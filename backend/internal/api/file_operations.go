package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/Muneer320/RhinoBox/internal/storage"
	"github.com/go-chi/chi/v5"
)

func (s *Server) fileOperationError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, storage.ErrFileNotFound), errors.Is(err, os.ErrNotExist):
		status = http.StatusNotFound
	case errors.Is(err, storage.ErrCopyConflict), errors.Is(err, storage.ErrCategoryConflict):
		status = http.StatusConflict
	case errors.Is(err, storage.ErrInvalidCategory), errors.Is(err, storage.ErrInvalidFilename), errors.Is(err, storage.ErrInvalidPath):
		status = http.StatusBadRequest
	}
	httpError(w, status, err.Error())
}

func (s *Server) handleFileCopy(w http.ResponseWriter, r *http.Request) {
	var payload storage.CopyRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpError(w, http.StatusBadRequest, "invalid copy request")
		return
	}
	payload.Hash = chi.URLParam(r, "file_id")
	result, err := s.storage.CopyFile(payload)
	if err != nil {
		s.fileOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleBatchFileCopy(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Copies []storage.CopyRequest `json:"copies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload.Copies) == 0 || len(payload.Copies) > 100 {
		httpError(w, http.StatusBadRequest, "provide between 1 and 100 copy requests")
		return
	}
	results := make([]map[string]any, 0, len(payload.Copies))
	succeeded := 0
	for _, copy := range payload.Copies {
		result, err := s.storage.CopyFile(copy)
		if err != nil {
			results = append(results, map[string]any{"success": false, "error": err.Error()})
			continue
		}
		succeeded++
		results = append(results, map[string]any{"success": true, "data": result})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "total": len(results), "success_count": succeeded, "failure_count": len(results) - succeeded})
}

func (s *Server) handleFileMove(w http.ResponseWriter, r *http.Request) {
	if chi.URLParam(r, "file_id") == "batch" {
		s.handleBatchFileMove(w, r)
		return
	}
	var payload storage.MoveRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpError(w, http.StatusBadRequest, "invalid move request")
		return
	}
	payload.Hash = chi.URLParam(r, "file_id")
	result, err := s.storage.MoveFile(payload)
	if err != nil {
		s.fileOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleBatchFileMove(w http.ResponseWriter, r *http.Request) {
	var payload storage.BatchMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || len(payload.Files) == 0 || len(payload.Files) > 100 {
		httpError(w, http.StatusBadRequest, "provide between 1 and 100 move requests")
		return
	}
	result, err := s.storage.BatchMoveFile(payload)
	if err != nil {
		s.fileOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
