package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// handleListFiles handles GET /files with pagination and filtering.
func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Parse pagination parameters
	page := parseInt(query.Get("page"), 1)
	limit := parseInt(query.Get("limit"), 50)
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	// Parse filter parameters
	category := query.Get("category")
	mimeType := query.Get("mime_type")
	minSize := parseInt64(query.Get("min_size"), 0)
	maxSize := parseInt64(query.Get("max_size"), 0)
	searchTerm := query.Get("search")

	// Parse date range
	var uploadedAfter, uploadedBefore time.Time
	if after := query.Get("uploaded_after"); after != "" {
		if t, err := time.Parse(time.RFC3339, after); err == nil {
			uploadedAfter = t
		}
	}
	if before := query.Get("uploaded_before"); before != "" {
		if t, err := time.Parse(time.RFC3339, before); err == nil {
			uploadedBefore = t
		}
	}

	// Parse sort parameters
	sortBy := query.Get("sort")
	if sortBy == "" {
		sortBy = "date"
	}
	sortOrder := query.Get("order")
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// Build query
	q := storage.FileQuery{
		Category:       category,
		MimeType:       mimeType,
		MinSize:        minSize,
		MaxSize:        maxSize,
		UploadedAfter:  uploadedAfter,
		UploadedBefore: uploadedBefore,
		SearchTerm:     searchTerm,
		SortBy:         sortBy,
		SortOrder:      sortOrder,
		Limit:          limit,
		Offset:         offset,
	}

	// Execute query
	files := s.storage.QueryFiles(q)
	total := s.storage.CountFiles(q)

	// Build response
	totalPages := (total + limit - 1) / limit
	response := map[string]any{
		"files": files,
		"pagination": map[string]any{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
			"has_next":    page < totalPages,
			"has_prev":    page > 1,
		},
	}

	writeJSON(w, http.StatusOK, response)
}

// handleBrowseFiles handles GET /files/browse for directory navigation.
func (s *Server) handleBrowseFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "storage"
	}

	// Resolve path relative to data directory
	absPath := filepath.Join(s.cfg.DataDir, filepath.FromSlash(path))

	// Security check: ensure path is within data directory
	relPath, err := filepath.Rel(s.cfg.DataDir, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		httpError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			httpError(w, http.StatusNotFound, "path not found")
			return
		}
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("stat path: %v", err))
		return
	}

	if !info.IsDir() {
		httpError(w, http.StatusBadRequest, "path is not a directory")
		return
	}

	// Read directory contents
	entries, err := os.ReadDir(absPath)
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("read directory: %v", err))
		return
	}

	// Build directory listing
	dirs := []map[string]any{}
	files := []map[string]any{}

	for _, entry := range entries {
		entryPath := filepath.Join(absPath, entry.Name())
		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		item := map[string]any{
			"name": entry.Name(),
			"path": filepath.ToSlash(filepath.Join(path, entry.Name())),
		}

		if entry.IsDir() {
			// Count files in subdirectory
			count := countFilesInDir(entryPath)
			item["file_count"] = count
			dirs = append(dirs, item)
		} else {
			item["size"] = entryInfo.Size()
			item["modified"] = entryInfo.ModTime().UTC().Format(time.RFC3339)
			files = append(files, item)
		}
	}

	// Build breadcrumb navigation
	breadcrumbs := buildBreadcrumbs(path)

	response := map[string]any{
		"path":        path,
		"breadcrumbs": breadcrumbs,
		"directories": dirs,
		"files":       files,
	}

	writeJSON(w, http.StatusOK, response)
}

// handleListCategories handles GET /files/categories.
func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	categoryStats := s.storage.GetCategories()

	// Convert to sorted list
	categories := []map[string]any{}
	for path, stats := range categoryStats {
		categories = append(categories, map[string]any{
			"path":  path,
			"count": stats.Count,
			"size":  stats.Size,
		})
	}

	response := map[string]any{
		"categories": categories,
	}

	writeJSON(w, http.StatusOK, response)
}

// handleGetStats handles GET /files/stats.
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := s.storage.GetStats()

	response := map[string]any{
		"total_files": stats.TotalFiles,
		"total_size":  stats.TotalSize,
		"categories":  stats.Categories,
		"file_types":  stats.FileTypes,
		"recent_uploads": map[string]int{
			"last_24h": stats.Recent24h,
			"last_7d":  stats.Recent7d,
			"last_30d": stats.Recent30d,
		},
	}

	writeJSON(w, http.StatusOK, response)
}

// Helper functions

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if val < 1 {
		return def
	}
	return val
}

func parseInt64(s string, def int64) int64 {
	if s == "" {
		return def
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	if val < 0 {
		return def
	}
	return val
}

func countFilesInDir(path string) int {
	count := 0
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func buildBreadcrumbs(path string) []map[string]any {
	parts := strings.Split(filepath.ToSlash(path), "/")
	breadcrumbs := []map[string]any{}

	currentPath := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = filepath.ToSlash(filepath.Join(currentPath, part))
		}
		breadcrumbs = append(breadcrumbs, map[string]any{
			"name": part,
			"path": currentPath,
		})
	}

	return breadcrumbs
}
