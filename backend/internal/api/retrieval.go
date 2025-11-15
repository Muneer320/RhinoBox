package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// DownloadTracker tracks download events for analytics
type DownloadTracker struct {
	mu        sync.Mutex
	downloads []DownloadEvent
}

type DownloadEvent struct {
	Hash      string    `json:"hash"`
	Path      string    `json:"path"`
	ClientIP  string    `json:"client_ip"`
	Timestamp time.Time `json:"timestamp"`
	BytesSent int64     `json:"bytes_sent"`
}

var downloadTracker = &DownloadTracker{
	downloads: make([]DownloadEvent, 0),
}

// handleFileDownload serves files by hash or stored path
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	path := r.URL.Query().Get("path")
	disposition := r.URL.Query().Get("disposition") // "inline" or "attachment"

	if hash == "" && path == "" {
		httpError(w, http.StatusBadRequest, "missing required parameter: hash or path")
		return
	}

	var metadata *storage.FileMetadata
	var filePath string

	if hash != "" {
		metadata = s.storage.GetMetadataByHash(hash)
		if metadata == nil {
			httpError(w, http.StatusNotFound, "file not found")
			return
		}
		filePath = filepath.Join(s.cfg.DataDir, filepath.FromSlash(metadata.StoredPath))
	} else {
		// Download by path
		filePath = filepath.Join(s.cfg.DataDir, filepath.FromSlash(path))
		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			httpError(w, http.StatusNotFound, "file not found")
			return
		}
		// Try to get metadata for this path
		metadata = s.storage.GetMetadataByPath(path)
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			httpError(w, http.StatusNotFound, "file not found")
		} else {
			s.logger.Error("failed to open file", slog.Any("err", err))
			httpError(w, http.StatusInternalServerError, "failed to open file")
		}
		return
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		s.logger.Error("failed to stat file", slog.Any("err", err))
		httpError(w, http.StatusInternalServerError, "failed to read file info")
		return
	}

	// Set Content-Type
	contentType := "application/octet-stream"
	if metadata != nil && metadata.MimeType != "" {
		contentType = metadata.MimeType
	}

	// Set Content-Disposition
	filename := filepath.Base(filePath)
	if metadata != nil && metadata.OriginalName != "" {
		filename = metadata.OriginalName
	}
	if disposition == "" {
		disposition = "attachment"
	}
	contentDisposition := fmt.Sprintf(`%s; filename="%s"`, disposition, filename)

	// Set ETag (use file hash if available, otherwise generate from path + modtime)
	etag := ""
	if metadata != nil && metadata.Hash != "" {
		etag = fmt.Sprintf(`"%s"`, metadata.Hash)
	} else {
		etag = fmt.Sprintf(`"%s-%d"`, filepath.Base(filePath), fileInfo.ModTime().Unix())
	}

	// Check If-None-Match (ETag)
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// Check If-Modified-Since
	if modifiedSince := r.Header.Get("If-Modified-Since"); modifiedSince != "" {
		t, err := time.Parse(http.TimeFormat, modifiedSince)
		if err == nil {
			// Truncate to seconds for comparison (HTTP time format has second precision)
			modTime := fileInfo.ModTime().UTC().Truncate(time.Second)
			sinceTime := t.UTC().Truncate(time.Second)
			if !modTime.After(sinceTime) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	// Set response headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition)
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	w.Header().Set("Last-Modified", fileInfo.ModTime().UTC().Format(http.TimeFormat))
	w.Header().Set("ETag", etag)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=31536000")

	// Add custom metadata headers if available
	if metadata != nil {
		if metadata.UploadedAt.Unix() > 0 {
			w.Header().Set("X-Upload-Date", metadata.UploadedAt.Format(time.RFC3339))
		}
		w.Header().Set("X-File-Hash", metadata.Hash)
		w.Header().Set("X-File-Category", metadata.Category)
	}

	// Check for range request
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		s.serveFileRange(w, r, file, fileInfo.Size(), contentType, rangeHeader)
	} else {
		// Serve full file
		w.WriteHeader(http.StatusOK)
		bytesSent, err := io.Copy(w, file)
		if err != nil {
			s.logger.Error("failed to send file", slog.Any("err", err))
			return
		}

		// Track download event
		s.trackDownload(hash, path, r.RemoteAddr, bytesSent)
	}
}

// serveFileRange handles HTTP range requests for partial content delivery
func (s *Server) serveFileRange(w http.ResponseWriter, r *http.Request, file *os.File, fileSize int64, contentType, rangeHeader string) {
	// Parse Range header (e.g., "bytes=0-1023")
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		httpError(w, http.StatusRequestedRangeNotSatisfiable, "invalid range format")
		return
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		httpError(w, http.StatusRequestedRangeNotSatisfiable, "invalid range format")
		return
	}

	var start, end int64
	var err error

	// Parse start
	if parts[0] != "" {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 || start >= fileSize {
			httpError(w, http.StatusRequestedRangeNotSatisfiable, "invalid range start")
			return
		}
	}

	// Parse end
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start || end >= fileSize {
			httpError(w, http.StatusRequestedRangeNotSatisfiable, "invalid range end")
			return
		}
	} else {
		end = fileSize - 1
	}

	// Seek to start position
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		s.logger.Error("failed to seek file", slog.Any("err", err))
		httpError(w, http.StatusInternalServerError, "failed to seek file")
		return
	}

	// Set response headers for partial content
	contentLength := end - start + 1
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusPartialContent)

	// Send the requested range
	bytesSent, err := io.CopyN(w, file, contentLength)
	if err != nil && err != io.EOF {
		s.logger.Error("failed to send file range", slog.Any("err", err))
		return
	}

	// Track download event
	s.trackDownload("", "", r.RemoteAddr, bytesSent)
}

// handleFileMetadata returns file metadata without downloading the file
func (s *Server) handleFileMetadata(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	path := r.URL.Query().Get("path")

	if hash == "" && path == "" {
		httpError(w, http.StatusBadRequest, "missing required parameter: hash or path")
		return
	}

	var metadata *storage.FileMetadata
	if hash != "" {
		metadata = s.storage.GetMetadataByHash(hash)
	} else {
		metadata = s.storage.GetMetadataByPath(path)
	}

	if metadata == nil {
		httpError(w, http.StatusNotFound, "file not found")
		return
	}

	// Check if file still exists
	filePath := filepath.Join(s.cfg.DataDir, filepath.FromSlash(metadata.StoredPath))
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		httpError(w, http.StatusNotFound, "file deleted")
		return
	}

	response := map[string]any{
		"hash":          metadata.Hash,
		"original_name": metadata.OriginalName,
		"stored_path":   metadata.StoredPath,
		"category":      metadata.Category,
		"mime_type":     metadata.MimeType,
		"size":          metadata.Size,
		"uploaded_at":   metadata.UploadedAt.Format(time.RFC3339),
		"metadata":      metadata.Metadata,
	}

	if err == nil {
		response["last_modified"] = fileInfo.ModTime().UTC().Format(time.RFC3339)
		response["current_size"] = fileInfo.Size()
	}

	writeJSON(w, http.StatusOK, response)
}

// handleFileStream is an alias for handleFileDownload with streaming optimizations
func (s *Server) handleFileStream(w http.ResponseWriter, r *http.Request) {
	// Ensure disposition is inline for streaming
	if r.URL.Query().Get("disposition") == "" {
		q := r.URL.Query()
		q.Set("disposition", "inline")
		r.URL.RawQuery = q.Encode()
	}
	s.handleFileDownload(w, r)
}

// trackDownload logs download events for analytics
func (s *Server) trackDownload(hash, path, clientIP string, bytesSent int64) {
	event := DownloadEvent{
		Hash:      hash,
		Path:      path,
		ClientIP:  clientIP,
		Timestamp: time.Now().UTC(),
		BytesSent: bytesSent,
	}

	downloadTracker.mu.Lock()
	downloadTracker.downloads = append(downloadTracker.downloads, event)
	
	// Keep only last 10000 events in memory to prevent memory growth
	if len(downloadTracker.downloads) > 10000 {
		downloadTracker.downloads = downloadTracker.downloads[len(downloadTracker.downloads)-10000:]
	}
	downloadTracker.mu.Unlock()

	// Log to file
	logRecord := map[string]any{
		"hash":       hash,
		"path":       path,
		"client_ip":  clientIP,
		"timestamp":  event.Timestamp.Format(time.RFC3339),
		"bytes_sent": bytesSent,
	}
	
	if _, err := s.storage.AppendNDJSON(filepath.Join("downloads", "download_log.ndjson"), []map[string]any{logRecord}); err != nil {
		s.logger.Warn("failed to log download event", slog.Any("err", err))
	}
}


