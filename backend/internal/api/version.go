package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
	chi "github.com/go-chi/chi/v5"
)

// handleCreateVersion handles POST /files/{file_id}/versions
func (s *Server) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid multipart payload: %v", err))
		return
	}

	// Get file from form
	var fileHeader *multipart.FileHeader
	for _, headers := range r.MultipartForm.File {
		if len(headers) > 0 {
			fileHeader = headers[0]
			break
		}
	}

	if fileHeader == nil {
		httpError(w, http.StatusBadRequest, "file is required")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("open file: %v", err))
		return
	}
	defer file.Close()

	// Get optional parameters
	comment := r.FormValue("comment")
	uploadedBy := r.FormValue("uploaded_by")
	if uploadedBy == "" {
		uploadedBy = "anonymous"
	}

	// Detect MIME type (same approach as server.go)
	sniff := make([]byte, 512)
	n, _ := io.ReadFull(file, sniff)
	buf := bytes.NewBuffer(sniff[:n])
	reader := io.MultiReader(buf, file)

	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(sniff[:n])
	}

	// Create version
	versionReq := storage.VersionRequest{
		FileID:     fileID,
		Reader:     reader,
		Filename:   fileHeader.Filename,
		MimeType:   mimeType,
		Size:       fileHeader.Size,
		Comment:    comment,
		UploadedBy: uploadedBy,
	}

	result, err := s.storage.CreateVersion(versionReq)
	if err != nil {
		if errors.Is(err, storage.ErrFileNotFound) {
			httpError(w, http.StatusNotFound, err.Error())
		} else if errors.Is(err, storage.ErrVersionLimitReached) {
			httpError(w, http.StatusBadRequest, err.Error())
		} else {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("create version failed: %v", err))
		}
		return
	}

	response := map[string]any{
		"file_id":    result.FileID,
		"version":    result.Version,
		"is_new_file": result.IsNewFile,
	}

	s.logger.Info("version created",
		slog.String("file_id", result.FileID),
		slog.Int("version", result.Version.Version),
		slog.String("hash", result.Version.Hash),
	)

	writeJSON(w, http.StatusOK, response)
}

// handleListVersions handles GET /files/{file_id}/versions
func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	versions, err := s.storage.ListVersions(fileID)
	if err != nil {
		if errors.Is(err, storage.ErrFileNotFound) {
			httpError(w, http.StatusNotFound, err.Error())
		} else {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("list versions failed: %v", err))
		}
		return
	}

	// Format versions for response
	versionList := make([]map[string]any, len(versions))
	for i, v := range versions {
		versionList[i] = map[string]any{
			"version":     v.Version,
			"hash":        v.Hash,
			"size":        v.Size,
			"uploaded_at": v.UploadedAt.Format(time.RFC3339),
			"uploaded_by": v.UploadedBy,
			"comment":     v.Comment,
			"is_current":  v.IsCurrent,
		}
	}

	response := map[string]any{
		"file_id":  fileID,
		"versions": versionList,
		"count":    len(versionList),
	}

	writeJSON(w, http.StatusOK, response)
}

// handleGetVersion handles GET /files/{file_id}/versions/{version_number}
func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	versionStr := chi.URLParam(r, "version_number")
	versionNumber, err := strconv.Atoi(versionStr)
	if err != nil || versionNumber < 1 {
		httpError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	// Check if this is a download request (query param) or metadata request
	download := r.URL.Query().Get("download") == "true"

	if download {
		// Download the file
		result, err := s.storage.GetVersionFile(fileID, versionNumber)
		if err != nil {
			if errors.Is(err, storage.ErrFileNotFound) || errors.Is(err, storage.ErrVersionNotFound) {
				httpError(w, http.StatusNotFound, err.Error())
			} else {
				httpError(w, http.StatusInternalServerError, fmt.Sprintf("get version file failed: %v", err))
			}
			return
		}
		defer result.Reader.Close()

		// Log download
		_ = s.logDownload(r, result, nil, nil)

		// Set headers
		s.setFileHeaders(w, r, result, "attachment")

		// Copy file to response
		if _, err := io.Copy(w, result.Reader); err != nil {
			s.logger.Warn("failed to copy file to response", slog.Any("err", err))
		}
	} else {
		// Return version metadata
		version, err := s.storage.GetVersion(fileID, versionNumber)
		if err != nil {
			if errors.Is(err, storage.ErrFileNotFound) || errors.Is(err, storage.ErrVersionNotFound) {
				httpError(w, http.StatusNotFound, err.Error())
			} else {
				httpError(w, http.StatusInternalServerError, fmt.Sprintf("get version failed: %v", err))
			}
			return
		}

		response := map[string]any{
			"file_id": fileID,
			"version": map[string]any{
				"version":     version.Version,
				"hash":        version.Hash,
				"size":        version.Size,
				"uploaded_at": version.UploadedAt.Format(time.RFC3339),
				"uploaded_by": version.UploadedBy,
				"comment":     version.Comment,
				"is_current":  version.IsCurrent,
			},
		}

		writeJSON(w, http.StatusOK, response)
	}
}

// handleRevertVersion handles POST /files/{file_id}/revert
func (s *Server) handleRevertVersion(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	var req struct {
		Version int    `json:"version"`
		Comment string `json:"comment"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Version < 1 {
		httpError(w, http.StatusBadRequest, "version must be >= 1")
		return
	}

	version, err := s.storage.RevertVersion(fileID, req.Version, req.Comment)
	if err != nil {
		if errors.Is(err, storage.ErrFileNotFound) || errors.Is(err, storage.ErrVersionNotFound) {
			httpError(w, http.StatusNotFound, err.Error())
		} else {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("revert version failed: %v", err))
		}
		return
	}

	response := map[string]any{
		"file_id": fileID,
		"version": map[string]any{
			"version":     version.Version,
			"hash":        version.Hash,
			"size":        version.Size,
			"uploaded_at": version.UploadedAt.Format(time.RFC3339),
			"uploaded_by": version.UploadedBy,
			"comment":     version.Comment,
			"is_current":  version.IsCurrent,
		},
	}

	s.logger.Info("version reverted",
		slog.String("file_id", fileID),
		slog.Int("version", version.Version),
	)

	writeJSON(w, http.StatusOK, response)
}

// handleVersionDiff handles GET /files/{file_id}/versions/diff
func (s *Server) handleVersionDiff(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		httpError(w, http.StatusBadRequest, "from and to query parameters are required")
		return
	}

	fromVersion, err := strconv.Atoi(fromStr)
	if err != nil || fromVersion < 1 {
		httpError(w, http.StatusBadRequest, "invalid from version number")
		return
	}

	toVersion, err := strconv.Atoi(toStr)
	if err != nil || toVersion < 1 {
		httpError(w, http.StatusBadRequest, "invalid to version number")
		return
	}

	diff, err := s.storage.GetVersionDiff(fileID, fromVersion, toVersion)
	if err != nil {
		if errors.Is(err, storage.ErrFileNotFound) || errors.Is(err, storage.ErrVersionNotFound) {
			httpError(w, http.StatusNotFound, err.Error())
		} else {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("get version diff failed: %v", err))
		}
		return
	}

	writeJSON(w, http.StatusOK, diff)
}

