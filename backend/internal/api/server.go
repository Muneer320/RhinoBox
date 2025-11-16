package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Muneer320/RhinoBox/internal/cache"
	"github.com/Muneer320/RhinoBox/internal/config"
	apierrors "github.com/Muneer320/RhinoBox/internal/errors"
	"github.com/Muneer320/RhinoBox/internal/jsonschema"
	"github.com/Muneer320/RhinoBox/internal/media"
	"github.com/Muneer320/RhinoBox/internal/middleware"
	errormiddleware "github.com/Muneer320/RhinoBox/internal/middleware"
	respmw "github.com/Muneer320/RhinoBox/internal/middleware"
	validationmw "github.com/Muneer320/RhinoBox/internal/middleware"
	"github.com/Muneer320/RhinoBox/internal/queue"
	"github.com/Muneer320/RhinoBox/internal/service"
	"github.com/Muneer320/RhinoBox/internal/services"
	"github.com/Muneer320/RhinoBox/internal/storage"
	chi "github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// Server wires everything together.
type Server struct {
	cfg              config.Config
	logger           *slog.Logger
	router           chi.Router
	storage          *storage.Manager
	fileService      *service.FileService
	collectionService *services.CollectionService
	jobQueue         *queue.JobQueue
	server           *http.Server
	errorHandler     *errormiddleware.ErrorHandler
	rateLimiter      *middleware.RateLimiter
}

// NewServer constructs the HTTP server with routing and dependencies.
func NewServer(cfg config.Config, logger *slog.Logger) (*Server, error) {
	store, err := storage.NewManager(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	fileService := service.NewFileService(store, logger)
	errorHandler := errormiddleware.NewErrorHandler(logger)

	// Initialize cache for collection service
	cacheConfig := cache.DefaultConfig()
	cacheConfig.L3Path = filepath.Join(cfg.DataDir, "cache", "collections")
	cacheInstance, err := cache.New(cacheConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize collection cache: %w", err)
	}

	// Initialize collection service
	collectionService := services.NewCollectionService(store, cacheInstance, logger)

	s := &Server{
		cfg:              cfg,
		logger:           logger,
		router:           chi.NewRouter(),
		storage:          store,
		fileService:      fileService,
		collectionService: collectionService,
		jobQueue:         nil, // TODO: Initialize when async endpoints are needed
		errorHandler:      errorHandler,
	}
	s.routes()
	return s, nil
}

// setupValidation configures validation middleware
func (s *Server) setupValidation() *validationmw.Validator {
	validator := validationmw.NewValidator(s.logger)
	validationmw.RegisterAllSchemas(validator, s.cfg.MaxUploadBytes)
	return validator
}

// Stop gracefully stops the server and cleans up resources.
func (s *Server) Stop() {
	// Stop rate limiter cleanup goroutine
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
	// Job queue shutdown will be implemented when async endpoints are added
	if s.jobQueue != nil {
		// s.jobQueue.Shutdown() // TODO: Implement when queue is initialized
	}
}

func (s *Server) routes() {
	r := s.router

	// Lightweight middleware for performance
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)

	// Security middleware - order matters!
	// 1. IP Filter (first - block/allow before processing)
	ipFilter := middleware.NewIPFilterMiddleware(s.cfg.Security, s.logger)
	r.Use(ipFilter.Handler)

	// 2. Request Size Limit (early - before body is read)
	requestSizeLimit := middleware.NewRequestSizeLimitMiddleware(s.cfg.Security, s.logger)
	r.Use(requestSizeLimit.Handler)

	// 3. Rate Limiting (after IP filter, before processing)
	s.rateLimiter = middleware.NewRateLimiter(s.cfg.Security, s.logger)
	r.Use(s.rateLimiter.Handler)

	// 4. CORS (before other headers) - replaces old handleCORS
	cors := middleware.NewCORSMiddleware(s.cfg.Security, s.logger)
	r.Use(cors.Handler)

	// 5. Security Headers (after CORS)
	securityHeaders := middleware.NewSecurityHeadersMiddleware(s.cfg.Security, s.logger)
	r.Use(securityHeaders.Handler)
	
	// Response transformation middleware (keep for response formatting)
	responseConfig := respmw.DefaultResponseConfig(s.logger)
	responseConfig.EnableCORS = false // Disable CORS here since we have dedicated middleware
	r.Use(respmw.NewResponseMiddleware(responseConfig).Handler)
	
	r.Use(s.customLogger)                    // Custom lightweight logger
	r.Use(s.errorHandler.Handler)            // Centralized error handling with panic recovery
	r.Use(chimw.Compress(5))                 // gzip level 5 (balance speed/compression)

	// Setup validation as global middleware
	// Validation will check route context after chi matches routes
	validator := s.setupValidation()
	r.Use(validator.Validate)

	// Endpoints
	r.Get("/healthz", s.handleHealth)
	r.Get("/api/config", s.handleConfig)
	r.Post("/ingest", s.handleUnifiedIngest)
	r.Post("/ingest/media", s.handleMediaIngest)
	r.Post("/ingest/json", s.handleJSONIngest)
	r.Patch("/files/rename", s.handleFileRename)
	// More specific routes must come before parameterized routes
	r.Get("/files", s.handleGetFiles)
	r.Get("/files/type/{type}", s.handleGetFilesByType)
	r.Get("/files/search", s.handleFileSearch)
	r.Get("/files/download", s.handleFileDownload)
	r.Get("/files/metadata", s.handleFileMetadata)
	r.Get("/files/stream", s.handleFileStream)
	r.Delete("/files/{file_id}", s.handleFileDelete)
	r.Patch("/files/{file_id}/metadata", s.handleMetadataUpdate)
	r.Post("/files/metadata/batch", s.handleBatchMetadataUpdate)

	// Notes endpoints
	r.Get("/files/{file_id}/notes", s.handleGetNotes)
	r.Post("/files/{file_id}/notes", s.handleAddNote)
	r.Patch("/files/{file_id}/notes/{note_id}", s.handleUpdateNote)
	r.Delete("/files/{file_id}/notes/{note_id}", s.handleDeleteNote)

	r.Get("/statistics", s.handleStatistics)
	r.Get("/collections", s.handleGetCollections)
	r.Get("/collections/{type}/stats", s.handleGetCollectionStats)
}


// customLogger is a lightweight logger middleware for high-performance scenarios
func (s *Server) customLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		
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
		s.handleError(w, r, apierrors.BadRequestf("invalid multipart payload: %v", err))
		return
	}

	if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
		s.handleError(w, r, apierrors.BadRequest("no files provided"))
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
				s.handleError(w, r, err)
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
		s.handleError(w, r, apierrors.InternalServerErrorf("start worker pool: %v", err))
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
				s.handleError(w, r, apierrors.InternalServerErrorf("submit job: %v", err))
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
			s.handleError(w, r, apierrors.Timeout("processing timeout"))
			return
		}
	}

	// If any failures occurred, return error
	if firstError != nil {
		s.handleError(w, r, apierrors.BadRequestf("processing error: %v", firstError))
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
		s.handleError(w, r, apierrors.BadRequestf("invalid JSON: %v", err))
		return
	}

	docs := req.Documents
	if len(docs) == 0 && req.Document != nil {
		docs = append(docs, req.Document)
	}
	if len(docs) == 0 {
		s.handleError(w, r, apierrors.BadRequest("no JSON documents provided"))
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
		s.handleError(w, r, apierrors.InternalServerErrorf("store batch: %v", err))
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
			s.handleError(w, r, apierrors.InternalServerErrorf("write schema: %v", err))
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
		s.handleError(w, r, apierrors.BadRequestf("invalid JSON: %v", err))
		return
	}

	// Validate required fields
	if req.Hash == "" {
		s.handleError(w, r, apierrors.BadRequest("hash is required"))
		return
	}
	if req.NewName == "" {
		s.handleError(w, r, apierrors.BadRequest("new_name is required"))
		return
	}

	result, err := s.storage.RenameFile(req)
	if err != nil {
		s.handleError(w, r, err)
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
		s.handleError(w, r, apierrors.BadRequest("file_id is required"))
		return
	}

	req := storage.DeleteRequest{
		Hash: fileID,
	}

	result, err := s.storage.DeleteFile(req)
	if err != nil {
		s.handleError(w, r, err)
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
		s.handleError(w, r, apierrors.BadRequest("file_id is required"))
		return
	}

	var req storage.MetadataUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.handleError(w, r, apierrors.BadRequestf("invalid JSON: %v", err))
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
		s.handleError(w, r, err)
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
		s.handleError(w, r, apierrors.BadRequestf("invalid JSON: %v", err))
		return
	}

	if len(req.Updates) == 0 {
		s.handleError(w, r, apierrors.BadRequest("no updates provided"))
		return
	}

	if len(req.Updates) > 100 {
		s.handleError(w, r, apierrors.BadRequest("too many updates (max 100)"))
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
	// Get search query from URL parameter
	query := r.URL.Query().Get("name")
	if query == "" {
		s.handleError(w, r, apierrors.BadRequest("name query parameter is required"))
		return
	}

	results := s.storage.FindByOriginalName(query)

	// Transform results to include frontend-friendly field names
	formattedResults := make([]map[string]any, len(results))
	for i, meta := range results {
		formattedResults[i] = map[string]any{
			"id":            meta.Hash,
			"hash":          meta.Hash,
			"original_name": meta.OriginalName,
			"name":          meta.OriginalName,
			"stored_path":   meta.StoredPath,
			"path":          meta.StoredPath,
			"category":      meta.Category,
			"type":          meta.MimeType,
			"mime_type":     meta.MimeType,
			"size":          meta.Size,
			"uploaded_at":   meta.UploadedAt,
			"modified_at":   meta.UploadedAt,
			"ingested_at":   meta.UploadedAt,
			"metadata":      meta.Metadata,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"query":   query,
		"results": formattedResults,
		"count":   len(formattedResults),
	})
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
		s.handleError(w, r, apierrors.BadRequest("hash or path query parameter is required"))
		return
	}

	if err != nil {
		s.handleError(w, r, err)
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
		s.handleError(w, r, apierrors.BadRequest("hash query parameter is required"))
		return
	}

	metadata, err := s.storage.GetFileMetadata(hash)
	if err != nil {
		s.handleError(w, r, err)
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
		s.handleError(w, r, apierrors.BadRequest("hash or path query parameter is required"))
		return
	}

	if err != nil {
		s.handleError(w, r, err)
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
				s.handleError(w, r, apierrors.NewAPIError(apierrors.ErrorCodeRangeNotSatisfiable, parseErr.Error()))
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
			s.handleError(w, r, apierrors.InternalServerErrorf("failed to seek file: %v", err))
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

// handleStatistics returns dashboard statistics.
func (s *Server) handleStatistics(w http.ResponseWriter, r *http.Request) {
	stats, err := s.storage.GetStatistics()
	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get statistics: %v", err))
		return
	}

	// Format response to match frontend expectations
	response := map[string]any{
		"totalFiles":   stats.TotalFiles,
		"files":        stats.TotalFiles, // Alias for compatibility
		"storageUsed":  stats.StorageUsedFormatted,
		"storage":      stats.StorageUsedFormatted, // Alias for compatibility
		"collections":  stats.CollectionCount,
		"collectionCount": stats.CollectionCount, // Alias for compatibility
		"storageUsedBytes": stats.StorageUsed,
		"collectionDetails": stats.Collections,
	}

	writeJSON(w, http.StatusOK, response)
}

// handleGetCollections returns all collections with their statistics.
func (s *Server) handleGetCollections(w http.ResponseWriter, r *http.Request) {
	response, err := s.collectionService.GetAllCollections()
	if err != nil {
		s.logger.Error("failed to get collections", slog.Any("err", err))
		s.handleError(w, r, apierrors.InternalServerErrorf("failed to get collections: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// handleGetCollectionStats returns statistics for a specific collection type.
func (s *Server) handleGetCollectionStats(w http.ResponseWriter, r *http.Request) {
	collectionType := chi.URLParam(r, "type")
	if collectionType == "" {
		s.handleError(w, r, apierrors.BadRequest("collection type is required"))
		return
	}

	response, err := s.collectionService.GetCollectionStats(collectionType)
	if err != nil {
		if strings.Contains(err.Error(), "invalid collection type") {
			s.handleError(w, r, apierrors.BadRequest(err.Error()))
			return
		}
		s.logger.Error("failed to get collection stats",
			slog.String("type", collectionType),
			slog.Any("err", err))
		s.handleError(w, r, apierrors.InternalServerErrorf("failed to get collection stats: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// handleGetFilesByType returns files filtered by collection type with pagination.
func (s *Server) handleGetFilesByType(w http.ResponseWriter, r *http.Request) {
	collectionType := chi.URLParam(r, "type")
	if collectionType == "" {
		s.handleError(w, r, apierrors.BadRequest("collection type is required"))
		return
	}

	// Parse pagination parameters
	page := 1
	limit := 50
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Parse category filter
	category := r.URL.Query().Get("category")

	// Build request
	req := storage.GetFilesByTypeRequest{
		Type:     collectionType,
		Page:     page,
		Limit:    limit,
		Category: category,
	}

	// Get files
	result, err := s.storage.GetFilesByType(req)
	if err != nil {
		s.handleError(w, r, apierrors.InternalServerError(fmt.Sprintf("failed to get files: %v", err)))
		return
	}

	// Convert FileMetadata to response format
	files := make([]map[string]any, 0, len(result.Files))
	for _, file := range result.Files {
		// Extract optional fields from metadata map
		dimensions := ""
		description := ""
		namespace := ""
		engine := ""
		if file.Metadata != nil {
			if dims, ok := file.Metadata["dimensions"]; ok {
				dimensions = dims
			}
			if desc, ok := file.Metadata["description"]; ok {
				description = desc
			}
			if comment, ok := file.Metadata["comment"]; ok && description == "" {
				description = comment
			}
			if ns, ok := file.Metadata["namespace"]; ok {
				namespace = ns
			}
			if eng, ok := file.Metadata["engine"]; ok {
				engine = eng
			}
		}

		files = append(files, map[string]any{
			"id":            file.Hash, // Use hash as ID
			"name":          file.OriginalName,
			"fileName":      file.OriginalName, // Alias for compatibility
			"path":          file.StoredPath,
			"filePath":      file.StoredPath, // Alias for compatibility
			"storedPath":    file.StoredPath,
			"size":          file.Size,
			"fileSize":      file.Size, // Alias for compatibility
			"type":          file.MimeType,
			"fileType":      file.MimeType, // Alias for compatibility
			"date":          file.UploadedAt.Format(time.RFC3339),
			"uploadedAt":    file.UploadedAt.Format(time.RFC3339),
			"hash":          file.Hash,
			"url":           fmt.Sprintf("/files/download?hash=%s", file.Hash),
			"downloadUrl":   fmt.Sprintf("/files/download?hash=%s", file.Hash),
			"dimensions":    dimensions,
			"fileDimensions": dimensions, // Alias for compatibility
			"description":   description,
			"comment":       description, // Alias for compatibility
			"namespace":     namespace,
			"collection":    namespace,
			"engine":        engine,
		})
	}

	// Build response matching frontend expectations
	response := map[string]any{
		"files":      files,
		"type":       collectionType,
		"total":      result.Total,
		"page":       result.Page,
		"limit":      result.Limit,
		"total_pages": result.TotalPages,
	}

	writeJSON(w, http.StatusOK, response)
}

// handleGetFiles returns a paginated list of files and supports common filters.
func (s *Server) handleGetFiles(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	page := 1
	limit := 50
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Filters
	opts := storage.ListOptions{
		Page:     page,
		Limit:    limit,
		Category: r.URL.Query().Get("category"),
		Type:     r.URL.Query().Get("type"),
		MimeType: r.URL.Query().Get("mime_type"),
		Extension: r.URL.Query().Get("extension"),
		Name:     r.URL.Query().Get("name"),
		SortBy:   r.URL.Query().Get("sort_by"),
		Order:    r.URL.Query().Get("order"),
	}

	// Optional date filtering (RFC3339)
	if df := r.URL.Query().Get("date_from"); df != "" {
		if t, err := time.Parse(time.RFC3339, df); err == nil {
			opts.DateFrom = t
		}
	}
	if dt := r.URL.Query().Get("date_to"); dt != "" {
		if t, err := time.Parse(time.RFC3339, dt); err == nil {
			opts.DateTo = t
		}
	}

	// Fetch from storage manager
	result, err := s.storage.ListFiles(opts)
	if err != nil {
		s.handleError(w, r, apierrors.InternalServerError(fmt.Sprintf("failed to list files: %v", err)))
		return
	}

	// Convert to response format expected by frontend
	files := make([]map[string]any, 0, len(result.Files))
	for _, file := range result.Files {
		dimensions := ""
		description := ""
		namespace := ""
		engine := ""
		if file.Metadata != nil {
			if dims, ok := file.Metadata["dimensions"]; ok {
				dimensions = dims
			}
			if desc, ok := file.Metadata["description"]; ok {
				description = desc
			}
			if comment, ok := file.Metadata["comment"]; ok && description == "" {
				description = comment
			}
			if ns, ok := file.Metadata["namespace"]; ok {
				namespace = ns
			}
			if eng, ok := file.Metadata["engine"]; ok {
				engine = eng
			}
		}

		files = append(files, map[string]any{
			"id":            file.Hash,
			"name":          file.OriginalName,
			"fileName":      file.OriginalName,
			"path":          file.StoredPath,
			"filePath":      file.StoredPath,
			"storedPath":    file.StoredPath,
			"size":          file.Size,
			"fileSize":      file.Size,
			"type":          file.MimeType,
			"fileType":      file.MimeType,
			"date":          file.UploadedAt.Format(time.RFC3339),
			"uploadedAt":    file.UploadedAt.Format(time.RFC3339),
			"hash":          file.Hash,
			"url":           fmt.Sprintf("/files/download?hash=%s", file.Hash),
			"downloadUrl":   fmt.Sprintf("/files/download?hash=%s", file.Hash),
			"dimensions":    dimensions,
			"fileDimensions": dimensions,
			"description":   description,
			"comment":       description,
			"namespace":     namespace,
			"collection":    namespace,
			"engine":        engine,
		})
	}

	resp := map[string]any{
		"files": files,
		"pagination": map[string]any{
			"page":       result.Pagination.Page,
			"limit":      result.Pagination.Limit,
			"total":      result.Pagination.Total,
			"total_pages": result.Pagination.TotalPages,
			"has_next":   result.Pagination.HasNext,
			"has_prev":   result.Pagination.HasPrev,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// Helper structs

type jsonIngestRequest struct {
Document  map[string]any   `json:"document"`
Documents []map[string]any `json:"documents"`
Namespace string           `json:"namespace"`
Comment   string           `json:"comment"`
Metadata  map[string]any   `json:"metadata"`
}

// getRequestID extracts the request ID from the context
func getRequestID(r *http.Request) string {
	if id := r.Context().Value(chimw.RequestIDKey); id != nil {
		if str, ok := id.(string); ok {
			return str
		}
	}
	return ""
}

// handleError processes errors through the centralized error handler
func (s *Server) handleError(w http.ResponseWriter, r *http.Request, err error) {
	s.errorHandler.HandleError(w, r, err)
}

// httpError is kept for backward compatibility but now uses centralized error handling
func (s *Server) httpError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	var errorCode apierrors.ErrorCode
	switch code {
	case http.StatusBadRequest:
		errorCode = apierrors.ErrorCodeBadRequest
	case http.StatusNotFound:
		errorCode = apierrors.ErrorCodeNotFound
	case http.StatusConflict:
		errorCode = apierrors.ErrorCodeConflict
	case http.StatusRequestTimeout:
		errorCode = apierrors.ErrorCodeTimeout
	default:
		errorCode = apierrors.ErrorCodeInternalServerError
	}
	apiErr := apierrors.NewAPIError(errorCode, msg)
	s.errorHandler.HandleError(w, r, apiErr)
}

// Legacy httpError for compatibility - creates a simple error
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
