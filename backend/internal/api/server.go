package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Muneer320/RhinoBox/internal/config"
	"github.com/Muneer320/RhinoBox/internal/jsonschema"
	"github.com/Muneer320/RhinoBox/internal/media"
	"github.com/Muneer320/RhinoBox/internal/storage"
	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// Server wires everything together.
type Server struct {
	cfg         config.Config
	logger      *slog.Logger
	router      chi.Router
	storage     *storage.Manager
	server      *http.Server
}

// NewServer constructs the HTTP server with routing and dependencies.
func NewServer(cfg config.Config, logger *slog.Logger) (*Server, error) {
	store, err := storage.NewManager(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:         cfg,
		logger:      logger,
		router:      chi.NewRouter(),
		storage:     store,
	}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	r := s.router

	// Lightweight middleware for performance
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.customLogger)       // Custom lightweight logger
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5)) // gzip level 5 (balance speed/compression)

	// Endpoints
	r.Get("/healthz", s.handleHealth)
	r.Post("/ingest", s.handleUnifiedIngest)
	r.Post("/ingest/media", s.handleMediaIngest)
	r.Post("/ingest/json", s.handleJSONIngest)
	r.Patch("/files/rename", s.handleFileRename)
	r.Delete("/files/{file_id}", s.handleFileDelete)
	r.Patch("/files/{file_id}/metadata", s.handleMetadataUpdate)
	r.Post("/files/metadata/batch", s.handleBatchMetadataUpdate)
	r.Get("/files/search", s.handleFileSearch)
	r.Get("/files/download", s.handleFileDownload)
	r.Get("/files/metadata", s.handleFileMetadata)
	r.Get("/files/stream", s.handleFileStream)
}

// customLogger is a lightweight logger middleware for high-performance scenarios
func (s *Server) customLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		
		defer func() {
			duration := time.Since(start)
			s.logger.Debug("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Duration("duration", duration),
				slog.String("proto", r.Proto))
		}()
		
		next.ServeHTTP(ww, r)
	})
}

// Router exposes the HTTP router for testing and server setup.
func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) handleMediaIngest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid multipart payload: %v", err))
		return
	}

	if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
		httpError(w, http.StatusBadRequest, "no files provided")
		return
	}

	categoryHint := r.FormValue("category")
	comment := r.FormValue("comment")

	// Count total files
	totalFiles := 0
	for _, headers := range r.MultipartForm.File {
		totalFiles += len(headers)
	}

	// Use parallel processing for batches > 1 file
	if totalFiles > 1 {
		s.handleMediaIngestParallel(w, r, categoryHint, comment, totalFiles)
		return
	}

	// Single file - use sequential path for simplicity
	records := make([]map[string]any, 0)
	responses := make([]map[string]any, 0)

	for _, headers := range r.MultipartForm.File {
		for _, header := range headers {
			record, err := s.storeSingleFile(header, categoryHint, comment)
			if err != nil {
				httpError(w, http.StatusBadRequest, err.Error())
				return
			}
			records = append(records, record)
			responses = append(responses, record)
		}
	}

	if len(records) > 0 {
		if _, err := s.storage.AppendNDJSON(filepath.ToSlash(filepath.Join("media", "ingest_log.ndjson")), records); err != nil {
			// log but don't fail request
			s.logger.Warn("failed to append media log", slog.Any("err", err))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"stored": responses})
}

// handleMediaIngestParallel processes multiple files concurrently using worker pool
func (s *Server) handleMediaIngestParallel(w http.ResponseWriter, r *http.Request, categoryHint, comment string, totalFiles int) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// Create worker pool
	pool := media.NewWorkerPool(ctx, s.storage, 0) // 0 = auto-detect worker count
	if err := pool.Start(); err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("start worker pool: %v", err))
		return
	}
	defer pool.Shutdown()

	// Submit all jobs
	jobIndex := 0
	for _, headers := range r.MultipartForm.File {
		for _, header := range headers {
			job := &media.ProcessJob{
				Header:       header,
				CategoryHint: categoryHint,
				Comment:      comment,
				JobID:        uuid.New().String(),
				Index:        jobIndex,
			}
			if err := pool.Submit(job); err != nil {
				httpError(w, http.StatusInternalServerError, fmt.Sprintf("submit job: %v", err))
				return
			}
			jobIndex++
		}
	}

	// Collect results
	results := make([]*media.ProcessResult, 0, totalFiles)
	successCount := 0
	var firstError error

	for i := 0; i < totalFiles; i++ {
		select {
		case result := <-pool.Results():
			results = append(results, result)
			if result.Success {
				successCount++
			} else if firstError == nil {
				firstError = result.Error
			}
		case <-ctx.Done():
			httpError(w, http.StatusRequestTimeout, "processing timeout")
			return
		}
	}

	// If any failures occurred, return error
	if firstError != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("processing error: %v", firstError))
		return
	}

	// Sort results by original index to maintain order
	sort.Slice(results, func(i, j int) bool {
		return results[i].Index < results[j].Index
	})

	// Build response
	records := make([]map[string]any, 0, len(results))
	responses := make([]map[string]any, 0, len(results))
	for _, result := range results {
		if result.Success && result.Record != nil {
			records = append(records, result.Record)
			responses = append(responses, result.Record)
		}
	}

	// Log batch processing
	if len(records) > 0 {
		if _, err := s.storage.AppendNDJSON(filepath.ToSlash(filepath.Join("media", "ingest_log.ndjson")), records); err != nil {
			s.logger.Warn("failed to append media log", slog.Any("err", err))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"stored": responses})
}

func (s *Server) storeSingleFile(header *multipart.FileHeader, categoryHint, comment string) (map[string]any, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

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

	result, err := s.storage.StoreFile(storage.StoreRequest{
		Reader:       reader,
		Filename:     header.Filename,
		MimeType:     mimeType,
		Size:         header.Size,
		Metadata:     metadata,
		CategoryHint: categoryHint,
	})
	if err != nil {
		return nil, err
	}

	mediaType := result.Metadata.Category
	if idx := strings.Index(mediaType, "/"); idx > 0 {
		mediaType = mediaType[:idx]
	}

	record := map[string]any{
		"path":          result.Metadata.StoredPath,
		"mime_type":     result.Metadata.MimeType,
		"category":      result.Metadata.Category,
		"media_type":    mediaType,
		"comment":       comment,
		"original_name": result.Metadata.OriginalName,
		"uploaded_at":   result.Metadata.UploadedAt.Format(time.RFC3339),
		"hash":          result.Metadata.Hash,
		"size":          result.Metadata.Size,
	}
	if result.Duplicate {
		record["duplicate"] = true
	}
	return record, nil
}

func (s *Server) handleJSONIngest(w http.ResponseWriter, r *http.Request) {
	var req jsonIngestRequest
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	if err := dec.Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	docs := req.Documents
	if len(docs) == 0 && req.Document != nil {
		docs = append(docs, req.Document)
	}
	if len(docs) == 0 {
		httpError(w, http.StatusBadRequest, "no JSON documents provided")
		return
	}

	analyzer := jsonschema.NewAnalyzer(4, 256)
	analyzer.AnalyzeBatch(docs)
	summary := analyzer.BuildSummary()
	analysis := analyzer.AnalyzeStructure(docs, summary)
	analysis = jsonschema.IncorporateCommentHints(analysis, req.Comment)
	decision := jsonschema.DecideStorage(req.Namespace, docs, summary, analysis)

	batchRel := s.storage.NextJSONBatchPath(decision.Engine, req.Namespace)
	if _, err := s.storage.AppendNDJSON(batchRel, docs); err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("store batch: %v", err))
		return
	}

	schemaPath := ""
	if decision.Engine == "sql" {
		schemaPayload := map[string]any{
			"table":    decision.Table,
			"ddl":      decision.Schema,
			"columns":  decision.Columns,
			"summary":  decision.Summary,
			"analysis": decision.Analysis,
		}
		var err error
		schemaPath, err = s.storage.WriteJSONFile(filepath.Join("json", "sql", decision.Table, "schema.json"), schemaPayload)
		if err != nil {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("write schema: %v", err))
			return
		}
	}

	logRecord := map[string]any{
		"namespace":   req.Namespace,
		"comment":     req.Comment,
		"metadata":    req.Metadata,
		"decision":    decision.Engine,
		"confidence":  decision.Confidence,
		"documents":   len(docs),
		"batch_path":  batchRel,
		"schema_path": schemaPath,
		"ingested_at": time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := s.storage.AppendNDJSON(filepath.Join("json", "ingest_log.ndjson"), []map[string]any{logRecord}); err != nil {
		s.logger.Warn("failed to append json log", slog.Any("err", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"decision":    decision,
		"batch_path":  batchRel,
		"schema_path": schemaPath,
		"documents":   len(docs),
	})
}

func (s *Server) handleFileRename(w http.ResponseWriter, r *http.Request) {
var req storage.RenameRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
return
}

// Validate required fields
if req.Hash == "" {
httpError(w, http.StatusBadRequest, "hash is required")
return
}
if req.NewName == "" {
httpError(w, http.StatusBadRequest, "new_name is required")
return
}

result, err := s.storage.RenameFile(req)
if err != nil {
switch {
case errors.Is(err, storage.ErrFileNotFound):
httpError(w, http.StatusNotFound, err.Error())
case errors.Is(err, storage.ErrInvalidFilename):
httpError(w, http.StatusBadRequest, err.Error())
case errors.Is(err, storage.ErrNameConflict):
httpError(w, http.StatusConflict, err.Error())
default:
httpError(w, http.StatusInternalServerError, fmt.Sprintf("rename failed: %v", err))
}
return
}

s.logger.Info("file renamed",
slog.String("hash", req.Hash),
slog.String("old_name", result.OldMetadata.OriginalName),
slog.String("new_name", result.NewMetadata.OriginalName),
slog.Bool("updated_stored_file", req.UpdateStoredFile),
)

writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	req := storage.DeleteRequest{
		Hash: fileID,
	}

	result, err := s.storage.DeleteFile(req)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrInvalidInput):
			httpError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, storage.ErrFileNotFound):
			httpError(w, http.StatusNotFound, err.Error())
		default:
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("delete failed: %v", err))
		}
		return
	}

	s.logger.Info("file deleted",
		slog.String("hash", req.Hash),
		slog.String("original_name", result.OriginalName),
		slog.String("stored_path", result.StoredPath),
	)

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleMetadataUpdate(w http.ResponseWriter, r *http.Request) {
fileID := chi.URLParam(r, "file_id")
if fileID == "" {
httpError(w, http.StatusBadRequest, "file_id is required")
return
}

var req storage.MetadataUpdateRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
return
}

// Set the hash from URL parameter
req.Hash = fileID

// Default to merge action if not specified
if req.Action == "" {
req.Action = "merge"
}

result, err := s.storage.UpdateFileMetadata(req)
if err != nil {
	switch {
	case errors.Is(err, storage.ErrMetadataNotFound):
		httpError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, storage.ErrMetadataTooLarge),
		errors.Is(err, storage.ErrInvalidMetadataKey),
		errors.Is(err, storage.ErrProtectedField):
		httpError(w, http.StatusBadRequest, err.Error())
	default:
		// Check if error message contains validation keywords
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid action") ||
			strings.Contains(errMsg, "metadata is required") ||
			strings.Contains(errMsg, "fields is required") ||
			strings.Contains(errMsg, "too large") ||
			strings.Contains(errMsg, "invalid metadata key") ||
			strings.Contains(errMsg, "protected") {
			httpError(w, http.StatusBadRequest, err.Error())
		} else {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("metadata update failed: %v", err))
		}
	}
	return
}// Add timestamp
result.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

s.logger.Info("metadata updated",
slog.String("hash", req.Hash),
slog.String("action", req.Action),
slog.Int("field_count", len(result.NewMetadata)),
)

writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleBatchMetadataUpdate(w http.ResponseWriter, r *http.Request) {
var req struct {
Updates []storage.MetadataUpdateRequest `json:"updates"`
}

if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
return
}

if len(req.Updates) == 0 {
httpError(w, http.StatusBadRequest, "no updates provided")
return
}

if len(req.Updates) > 100 {
httpError(w, http.StatusBadRequest, "too many updates (max 100)")
return
}

results, errs := s.storage.BatchUpdateFileMetadata(req.Updates)

// Add timestamps and count successes/failures
successCount := 0
failureCount := 0
timestamp := time.Now().UTC().Format(time.RFC3339)

response := make([]map[string]any, len(results))
for i := range results {
if errs[i] != nil {
response[i] = map[string]any{
"hash":    req.Updates[i].Hash,
"success": false,
"error":   errs[i].Error(),
}
failureCount++
} else {
results[i].UpdatedAt = timestamp
response[i] = map[string]any{
"hash":         results[i].Hash,
"success":      true,
"old_metadata": results[i].OldMetadata,
"new_metadata": results[i].NewMetadata,
"action":       results[i].Action,
"updated_at":   results[i].UpdatedAt,
}
successCount++
}
}

s.logger.Info("batch metadata update",
slog.Int("total", len(req.Updates)),
slog.Int("success", successCount),
slog.Int("failed", failureCount),
)

writeJSON(w, http.StatusOK, map[string]any{
"results":       response,
"total":         len(req.Updates),
"success_count": successCount,
"failure_count": failureCount,
})
}

func (s *Server) handleFileSearch(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	filters := storage.SearchFilters{}

	// Name filter (optional, partial match)
	if name := r.URL.Query().Get("name"); name != "" {
		filters.Name = name
	}

	// Extension filter (optional, exact match)
	if ext := r.URL.Query().Get("extension"); ext != "" {
		filters.Extension = ext
	}

	// Type filter (optional, matches MIME type or category)
	if typ := r.URL.Query().Get("type"); typ != "" {
		filters.Type = typ
	}

	// Category filter (optional, partial match)
	if category := r.URL.Query().Get("category"); category != "" {
		filters.Category = category
	}

	// MIME type filter (optional, exact match)
	if mimeType := r.URL.Query().Get("mime_type"); mimeType != "" {
		filters.MimeType = mimeType
	}

	// Date range filters (optional)
	if dateFromStr := r.URL.Query().Get("date_from"); dateFromStr != "" {
		if dateFrom, err := time.Parse(time.RFC3339, dateFromStr); err == nil {
			filters.DateFrom = dateFrom
		} else if dateFrom, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			// Support date-only format (start of day)
			filters.DateFrom = dateFrom
		} else {
			httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid date_from format: %v (use RFC3339 or YYYY-MM-DD)", err))
			return
		}
	}

	if dateToStr := r.URL.Query().Get("date_to"); dateToStr != "" {
		if dateTo, err := time.Parse(time.RFC3339, dateToStr); err == nil {
			// RFC3339 format: use exact timestamp
			filters.DateTo = dateTo
		} else if dateTo, err := time.Parse("2006-01-02", dateToStr); err == nil {
			// YYYY-MM-DD format: extend to end of day (23:59:59.999...)
			filters.DateTo = dateTo.Add(24*time.Hour - time.Nanosecond)
		} else {
			httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid date_to format: %v (use RFC3339 or YYYY-MM-DD)", err))
			return
		}
	}

	// At least one filter must be provided
	if filters.Name == "" && filters.Extension == "" && filters.Type == "" &&
		filters.Category == "" && filters.MimeType == "" &&
		filters.DateFrom.IsZero() && filters.DateTo.IsZero() {
		httpError(w, http.StatusBadRequest, "at least one filter parameter is required (name, extension, type, category, mime_type, date_from, date_to)")
		return
	}

	results := s.storage.SearchFiles(filters)

	// Build response with filter summary
	response := map[string]any{
		"filters": map[string]any{},
		"results": results,
		"count":   len(results),
	}

	if filters.Name != "" {
		response["filters"].(map[string]any)["name"] = filters.Name
	}
	if filters.Extension != "" {
		response["filters"].(map[string]any)["extension"] = filters.Extension
	}
	if filters.Type != "" {
		response["filters"].(map[string]any)["type"] = filters.Type
	}
	if filters.Category != "" {
		response["filters"].(map[string]any)["category"] = filters.Category
	}
	if filters.MimeType != "" {
		response["filters"].(map[string]any)["mime_type"] = filters.MimeType
	}
	if !filters.DateFrom.IsZero() {
		response["filters"].(map[string]any)["date_from"] = filters.DateFrom.Format(time.RFC3339)
	}
	if !filters.DateTo.IsZero() {
		response["filters"].(map[string]any)["date_to"] = filters.DateTo.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, response)
}

// handleFileDownload downloads a file by hash or path.
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	path := r.URL.Query().Get("path")

	var result *storage.FileRetrievalResult
	var err error

	if hash != "" {
		result, err = s.storage.GetFileByHash(hash)
	} else if path != "" {
		result, err = s.storage.GetFileByPath(path)
	} else {
		httpError(w, http.StatusBadRequest, "hash or path query parameter is required")
		return
	}

	if err != nil {
		if errors.Is(err, storage.ErrFileNotFound) {
			httpError(w, http.StatusNotFound, err.Error())
		} else if errors.Is(err, storage.ErrInvalidPath) {
			httpError(w, http.StatusBadRequest, err.Error())
		} else {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to retrieve file: %v", err))
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
}

// handleFileMetadata returns file metadata without downloading the file.
func (s *Server) handleFileMetadata(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	if hash == "" {
		httpError(w, http.StatusBadRequest, "hash query parameter is required")
		return
	}

	metadata, err := s.storage.GetFileMetadata(hash)
	if err != nil {
		if errors.Is(err, storage.ErrFileNotFound) {
			httpError(w, http.StatusNotFound, err.Error())
		} else {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to retrieve metadata: %v", err))
		}
		return
	}

	writeJSON(w, http.StatusOK, metadata)
}

// handleFileStream streams a file with range request support for video/audio streaming.
func (s *Server) handleFileStream(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	path := r.URL.Query().Get("path")

	var result *storage.FileRetrievalResult
	var err error

	if hash != "" {
		result, err = s.storage.GetFileByHash(hash)
	} else if path != "" {
		result, err = s.storage.GetFileByPath(path)
	} else {
		httpError(w, http.StatusBadRequest, "hash or path query parameter is required")
		return
	}

	if err != nil {
		if errors.Is(err, storage.ErrFileNotFound) {
			httpError(w, http.StatusNotFound, err.Error())
		} else if errors.Is(err, storage.ErrInvalidPath) {
			httpError(w, http.StatusBadRequest, err.Error())
		} else {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to retrieve file: %v", err))
		}
		return
	}
	defer result.Reader.Close()

	// Check If-Modified-Since for 304 response
	if s.shouldReturn304(w, r, result) {
		return
	}

	// Parse range header (check If-Range condition first)
	rangeHeader := r.Header.Get("Range")
	var rangeStart, rangeEnd *int64

	if rangeHeader != "" {
		// Check If-Range: if condition fails, ignore Range header
		if !s.checkIfRange(r, result) {
			// If-Range condition failed, treat as no Range header
			rangeHeader = ""
		} else {
			// If-Range condition passed, parse the range
			start, end, parseErr := s.parseRangeHeader(rangeHeader, result.Size)
			if parseErr != nil {
				httpError(w, http.StatusRequestedRangeNotSatisfiable, parseErr.Error())
				return
			}
			rangeStart = &start
			rangeEnd = &end
		}
	}

	// Log download with range info
	_ = s.logDownload(r, result, rangeStart, rangeEnd)

	// Set headers for streaming
	if rangeStart != nil && rangeEnd != nil {
		// Partial content response - set all file headers first
		s.setFileHeaders(w, r, result, "inline")
		// Then override with range-specific headers
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", *rangeStart, *rangeEnd, result.Size))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", *rangeEnd-*rangeStart+1))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		// Full content response
		s.setFileHeaders(w, r, result, "inline")
	}

	// Seek to start position if range request
	if rangeStart != nil {
		if _, err := result.Reader.Seek(*rangeStart, io.SeekStart); err != nil {
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to seek file: %v", err))
			return
		}
	}

	// Copy file to response
	if rangeStart != nil && rangeEnd != nil {
		// Copy only the requested range
		limitedReader := io.LimitReader(result.Reader, *rangeEnd-*rangeStart+1)
		if _, err := io.Copy(w, limitedReader); err != nil {
			s.logger.Warn("failed to copy file range to response", slog.Any("err", err))
		}
	} else {
		// Copy entire file
		if _, err := io.Copy(w, result.Reader); err != nil {
			s.logger.Warn("failed to copy file to response", slog.Any("err", err))
		}
	}
}

// parseRangeHeader parses the Range header and returns start and end positions.
// Supports three forms: "bytes=start-end", "bytes=start-", and "bytes=-suffix".
// Returns an error if the range is invalid or malformed, which should result in 416.
func (s *Server) parseRangeHeader(rangeHeader string, fileSize int64) (start, end int64, err error) {
	if fileSize == 0 {
		return 0, 0, fmt.Errorf("file is empty")
	}

	// Must start with "bytes="
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, fmt.Errorf("invalid range unit")
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	if rangeSpec == "" {
		return 0, 0, fmt.Errorf("empty range spec")
	}

	// Handle "start-end" form (e.g., "bytes=0-499")
	if strings.Contains(rangeSpec, "-") && !strings.HasPrefix(rangeSpec, "-") {
		parts := strings.SplitN(rangeSpec, "-", 2)
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("malformed range")
		}

		var startVal, endVal int64
		if _, parseErr := fmt.Sscanf(parts[0], "%d", &startVal); parseErr != nil {
			return 0, 0, fmt.Errorf("invalid start position")
		}

		// Handle end position - can be empty (open-ended) or a number
		if parts[1] == "" {
			// Open-ended: "bytes=start-"
			if startVal < 0 || startVal >= fileSize {
				return 0, 0, fmt.Errorf("start position out of range")
			}
			endVal = fileSize - 1
		} else {
			// Both start and end specified: "bytes=start-end"
			if _, parseErr := fmt.Sscanf(parts[1], "%d", &endVal); parseErr != nil {
				return 0, 0, fmt.Errorf("invalid end position")
			}
			// Treat "bytes=0-0" as valid (requesting first byte)
			if startVal < 0 || startVal >= fileSize {
				return 0, 0, fmt.Errorf("start position out of range")
			}
			if endVal < startVal || endVal >= fileSize {
				// If end is out of range, clamp to file size
				if endVal >= fileSize {
					endVal = fileSize - 1
				} else {
					return 0, 0, fmt.Errorf("end position out of range")
				}
			}
		}

		return startVal, endVal, nil
	}

	// Handle "-suffix" form (e.g., "bytes=-500" for last 500 bytes)
	if strings.HasPrefix(rangeSpec, "-") {
		var suffix int64
		if _, parseErr := fmt.Sscanf(rangeSpec, "-%d", &suffix); parseErr != nil {
			return 0, 0, fmt.Errorf("invalid suffix length")
		}
		if suffix <= 0 {
			return 0, 0, fmt.Errorf("suffix must be positive")
		}
		if suffix > fileSize {
			// Request more bytes than available, return entire file
			return 0, fileSize - 1, nil
		}
		startVal := fileSize - suffix
		return startVal, fileSize - 1, nil
	}

	return 0, 0, fmt.Errorf("malformed range header")
}

// shouldReturn304 checks If-Modified-Since header and returns true if a 304 Not Modified response should be sent.
func (s *Server) shouldReturn304(w http.ResponseWriter, r *http.Request, result *storage.FileRetrievalResult) bool {
	// Check If-Modified-Since
	if ifModifiedSince := r.Header.Get("If-Modified-Since"); ifModifiedSince != "" {
		if t, err := http.ParseTime(ifModifiedSince); err == nil {
			if !result.Metadata.UploadedAt.After(t) {
				w.WriteHeader(http.StatusNotModified)
				return true
			}
		}
	}
	return false
}

// checkIfRange validates the If-Range header condition.
// Returns true if Range header should be honored, false if it should be ignored.
func (s *Server) checkIfRange(r *http.Request, result *storage.FileRetrievalResult) bool {
	ifRange := r.Header.Get("If-Range")
	if ifRange == "" {
		// No If-Range header, Range should be honored
		return true
	}

	// If-Range can be either an ETag or a Last-Modified date
	expectedETag := fmt.Sprintf(`"%s"`, result.Metadata.Hash)
	if ifRange == expectedETag {
		// ETag matches, Range should be honored
		return true
	}

	// Try parsing as date
	if t, err := http.ParseTime(ifRange); err == nil {
		// Compare with 1 second tolerance for clock skew
		modTime := result.Metadata.UploadedAt
		if modTime.Before(t.Add(-time.Second)) || modTime.After(t.Add(time.Second)) {
			// Resource has been modified, ignore Range header
			return false
		}
		// Date matches (within tolerance), Range should be honored
		return true
	}

	// If-Range value doesn't match ETag and isn't a valid date, ignore Range header
	return false
}

// setFileHeaders sets appropriate HTTP headers for file download/streaming.
func (s *Server) setFileHeaders(w http.ResponseWriter, r *http.Request, result *storage.FileRetrievalResult, disposition string) {
	w.Header().Set("Content-Type", result.Metadata.MimeType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", result.Size))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, result.Metadata.OriginalName))
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, result.Metadata.Hash))
	w.Header().Set("Last-Modified", result.Metadata.UploadedAt.Format(http.TimeFormat))
	w.Header().Set("X-File-Category", result.Metadata.Category)
	w.Header().Set("X-File-Hash", result.Metadata.Hash)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, max-age=3600")
}

// logDownload logs a download event for analytics.
func (s *Server) logDownload(r *http.Request, result *storage.FileRetrievalResult, rangeStart, rangeEnd *int64) error {
	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = forwarded
	}

	log := storage.DownloadLog{
		Hash:         result.Metadata.Hash,
		StoredPath:   result.Metadata.StoredPath,
		OriginalName: result.Metadata.OriginalName,
		MimeType:     result.Metadata.MimeType,
		Size:         result.Size,
		DownloadedAt: time.Now().UTC(),
		RangeStart:   rangeStart,
		RangeEnd:     rangeEnd,
		UserAgent:    r.UserAgent(),
		IPAddress:    ip,
	}

	return s.storage.LogDownload(log)
}

// Helper structs

type jsonIngestRequest struct {
Document  map[string]any   `json:"document"`
Documents []map[string]any `json:"documents"`
Namespace string           `json:"namespace"`
Comment   string           `json:"comment"`
Metadata  map[string]any   `json:"metadata"`
}

func httpError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func init() {
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr.MaxIdleConnsPerHost = 32
	}
}
