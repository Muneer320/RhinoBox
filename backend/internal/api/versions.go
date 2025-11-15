package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
	"github.com/go-chi/chi/v5"
)

// setupVersionRoutes configures the file versioning API endpoints.
func (s *Server) setupVersionRoutes(r chi.Router) {
	r.Route("/files/{fileID}/versions", func(r chi.Router) {
		r.Post("/", s.handleUploadVersion)           // Upload new version
		r.Get("/", s.handleListVersions)             // List all versions
		r.Get("/{version}", s.handleGetVersion)      // Get specific version
		r.Get("/diff", s.handleCompareVersions)      // Compare versions
	})
	r.Post("/files/{fileID}/revert", s.handleRevertVersion) // Revert to version
}

// handleUploadVersion uploads a new version of an existing file.
func (s *Server) handleUploadVersion(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	// Verify file exists
	if _, err := s.storage.GetVersionedFile(fileID); err != nil {
		httpError(w, http.StatusNotFound, fmt.Sprintf("file not found: %v", err))
		return
	}

	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid multipart payload: %v", err))
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("file field required: %v", err))
		return
	}
	defer file.Close()

	comment := r.FormValue("comment")
	uploadedBy := r.FormValue("uploaded_by")
	if uploadedBy == "" {
		uploadedBy = "anonymous"
	}

	// Read file content for MIME detection
	sniff := make([]byte, 512)
	n, _ := io.ReadFull(file, sniff)
	buf := bytes.NewBuffer(sniff[:n])
	reader := io.MultiReader(buf, file)

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(sniff[:n])
	}

	metadata := map[string]string{}
	if comment != "" {
		metadata["comment"] = comment
	}

	// Store the new version
	version, err := s.storage.StoreFileVersion(fileID, uploadedBy, storage.StoreRequest{
		Reader:   reader,
		Filename: header.Filename,
		MimeType: mimeType,
		Size:     header.Size,
		Metadata: metadata,
	})
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("store version: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"version":      version.Version,
		"hash":         version.Hash,
		"size":         version.Size,
		"uploaded_at":  version.UploadedAt.Format(time.RFC3339),
		"uploaded_by":  version.UploadedBy,
		"comment":      version.Comment,
		"is_current":   version.IsCurrent,
		"stored_path":  version.StoredPath,
		"mime_type":    version.MimeType,
	})
}

// handleListVersions lists all versions of a file.
func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	versionedFile, err := s.storage.GetVersionedFile(fileID)
	if err != nil {
		httpError(w, http.StatusNotFound, fmt.Sprintf("file not found: %v", err))
		return
	}

	versions := make([]map[string]any, 0, len(versionedFile.Versions))
	for _, v := range versionedFile.Versions {
		versions = append(versions, map[string]any{
			"version":       v.Version,
			"hash":          v.Hash,
			"size":          v.Size,
			"uploaded_at":   v.UploadedAt.Format(time.RFC3339),
			"uploaded_by":   v.UploadedBy,
			"comment":       v.Comment,
			"is_current":    v.IsCurrent,
			"mime_type":     v.MimeType,
			"original_name": v.OriginalName,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"file_id":         versionedFile.FileID,
		"current_version": versionedFile.CurrentVersion,
		"total_versions":  versionedFile.TotalVersions,
		"category":        versionedFile.Category,
		"created_at":      versionedFile.CreatedAt.Format(time.RFC3339),
		"updated_at":      versionedFile.UpdatedAt.Format(time.RFC3339),
		"versions":        versions,
	})
}

// handleGetVersion retrieves a specific version of a file.
func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")
	versionStr := chi.URLParam(r, "version")
	
	if fileID == "" || versionStr == "" {
		httpError(w, http.StatusBadRequest, "file_id and version are required")
		return
	}

	versionNum, err := strconv.Atoi(versionStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid version number")
		return
	}

	version, err := s.storage.GetFileVersion(fileID, versionNum)
	if err != nil {
		httpError(w, http.StatusNotFound, fmt.Sprintf("version not found: %v", err))
		return
	}

	// Check if download is requested
	if r.URL.Query().Get("download") == "true" {
		content, err := s.storage.ReadFileVersion(fileID, versionNum)
		if err != nil {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("read file: %v", err))
			return
		}

		w.Header().Set("Content-Type", version.MimeType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", version.OriginalName))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
		return
	}

	// Return version metadata
	writeJSON(w, http.StatusOK, map[string]any{
		"version":       version.Version,
		"hash":          version.Hash,
		"size":          version.Size,
		"uploaded_at":   version.UploadedAt.Format(time.RFC3339),
		"uploaded_by":   version.UploadedBy,
		"comment":       version.Comment,
		"is_current":    version.IsCurrent,
		"stored_path":   version.StoredPath,
		"mime_type":     version.MimeType,
		"original_name": version.OriginalName,
	})
}

// handleRevertVersion reverts a file to a previous version.
func (s *Server) handleRevertVersion(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	var req struct {
		Version    int    `json:"version"`
		Comment    string `json:"comment"`
		UploadedBy string `json:"uploaded_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if req.Version <= 0 {
		httpError(w, http.StatusBadRequest, "version must be positive")
		return
	}

	if req.UploadedBy == "" {
		req.UploadedBy = "anonymous"
	}

	comment := req.Comment
	if comment == "" {
		comment = fmt.Sprintf("Reverted to version %d", req.Version)
	}

	version, err := s.storage.RevertFileToVersion(fileID, req.Version, comment, req.UploadedBy)
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("revert failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":      "File reverted successfully",
		"new_version":  version.Version,
		"reverted_to":  req.Version,
		"hash":         version.Hash,
		"size":         version.Size,
		"uploaded_at":  version.UploadedAt.Format(time.RFC3339),
		"uploaded_by":  version.UploadedBy,
		"comment":      version.Comment,
	})
}

// handleCompareVersions compares two versions of a file.
func (s *Server) handleCompareVersions(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fileID == "" || fromStr == "" || toStr == "" {
		httpError(w, http.StatusBadRequest, "file_id, from, and to are required")
		return
	}

	fromVersion, err := strconv.Atoi(fromStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid from version")
		return
	}

	toVersion, err := strconv.Atoi(toStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid to version")
		return
	}

	v1, err := s.storage.GetFileVersion(fileID, fromVersion)
	if err != nil {
		httpError(w, http.StatusNotFound, fmt.Sprintf("from version not found: %v", err))
		return
	}

	v2, err := s.storage.GetFileVersion(fileID, toVersion)
	if err != nil {
		httpError(w, http.StatusNotFound, fmt.Sprintf("to version not found: %v", err))
		return
	}

	diff := compareVersionMetadata(v1, v2)
	
	writeJSON(w, http.StatusOK, map[string]any{
		"file_id": fileID,
		"from": map[string]any{
			"version":     v1.Version,
			"hash":        v1.Hash,
			"size":        v1.Size,
			"uploaded_at": v1.UploadedAt.Format(time.RFC3339),
			"uploaded_by": v1.UploadedBy,
			"comment":     v1.Comment,
		},
		"to": map[string]any{
			"version":     v2.Version,
			"hash":        v2.Hash,
			"size":        v2.Size,
			"uploaded_at": v2.UploadedAt.Format(time.RFC3339),
			"uploaded_by": v2.UploadedBy,
			"comment":     v2.Comment,
		},
		"differences": diff,
	})
}

// compareVersionMetadata compares two versions and returns differences.
func compareVersionMetadata(v1, v2 *storage.FileVersion) map[string]any {
	diff := make(map[string]any)

	if v1.Hash != v2.Hash {
		diff["content_changed"] = true
		diff["hash_from"] = v1.Hash
		diff["hash_to"] = v2.Hash
	} else {
		diff["content_changed"] = false
	}

	if v1.Size != v2.Size {
		diff["size_changed"] = true
		diff["size_delta"] = v2.Size - v1.Size
		diff["size_from"] = v1.Size
		diff["size_to"] = v2.Size
	}

	timeDiff := v2.UploadedAt.Sub(v1.UploadedAt)
	diff["time_between"] = timeDiff.String()
	diff["time_between_seconds"] = int64(timeDiff.Seconds())

	if v1.UploadedBy != v2.UploadedBy {
		diff["uploader_changed"] = true
		diff["uploader_from"] = v1.UploadedBy
		diff["uploader_to"] = v2.UploadedBy
	}

	if v1.OriginalName != v2.OriginalName {
		diff["filename_changed"] = true
		diff["filename_from"] = v1.OriginalName
		diff["filename_to"] = v2.OriginalName
	}

	if v1.MimeType != v2.MimeType {
		diff["mimetype_changed"] = true
		diff["mimetype_from"] = v1.MimeType
		diff["mimetype_to"] = v2.MimeType
	}

	// Compare comments
	diff["comment_from"] = v1.Comment
	diff["comment_to"] = v2.Comment
	if strings.TrimSpace(v1.Comment) != strings.TrimSpace(v2.Comment) {
		diff["comment_changed"] = true
	}

	return diff
}
