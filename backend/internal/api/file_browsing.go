package api

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

func (s *Server) handleFileByID(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	query.Set("hash", chi.URLParam(r, "file_id"))
	r.URL.RawQuery = query.Encode()
	s.handleFileMetadata(w, r)
}

func (s *Server) handleBrowseDirectory(w http.ResponseWriter, r *http.Request) {
	result, err := s.storage.BrowseDirectory(r.URL.Query().Get("path"))
	if err != nil {
		s.fileOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetCategories(w http.ResponseWriter, r *http.Request) {
	result, err := s.storage.GetCategories()
	if err != nil {
		s.fileOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": result, "count": len(result)})
}

func (s *Server) handleStorageStats(w http.ResponseWriter, r *http.Request) {
	result, err := s.storage.GetStorageStats()
	if err != nil {
		s.fileOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
