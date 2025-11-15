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
	"github.com/google/uuid"
	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", s.handleHealth)
	r.Post("/ingest", s.handleUnifiedIngest)
	r.Post("/ingest/media", s.handleMediaIngest)
	r.Post("/ingest/json", s.handleJSONIngest)
	r.Patch("/files/{file_id}/metadata", s.handleMetadataUpdate)
}

// Router exposes the HTTP router for testing.
func (s *Server) Router() http.Handler {
	return s.router
}

// Run starts the HTTP server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	s.server = &http.Server{Addr: s.cfg.Addr, Handler: s.router}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", slog.String("addr", s.cfg.Addr))
		errCh <- s.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
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

// handleMetadataUpdate handles PATCH /files/{file_id}/metadata requests
func (s *Server) handleMetadataUpdate(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "file_id")
	if fileID == "" {
		httpError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	var req metadataUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	// Validate action
	action := req.Action
	if action == "" {
		action = "replace" // default action
	}
	if action != "replace" && action != "merge" && action != "remove" {
		httpError(w, http.StatusBadRequest, "action must be 'replace', 'merge', or 'remove'")
		return
	}

	// Validate metadata size limit (10KB per entry, 100 total fields)
	if action != "remove" {
		if len(req.Metadata) > 100 {
			httpError(w, http.StatusBadRequest, "metadata cannot exceed 100 fields")
			return
		}
		totalSize := 0
		for k, v := range req.Metadata {
			if len(k) > 256 {
				httpError(w, http.StatusBadRequest, fmt.Sprintf("metadata key '%s' exceeds 256 characters", k))
				return
			}
			if len(v) > 10240 {
				httpError(w, http.StatusBadRequest, fmt.Sprintf("metadata value for '%s' exceeds 10KB", k))
				return
			}
			totalSize += len(k) + len(v)
		}
		if totalSize > 102400 { // 100KB total
			httpError(w, http.StatusBadRequest, "total metadata size exceeds 100KB")
			return
		}
	}

	// Check for protected system fields
	protectedFields := []string{"hash", "size", "uploaded_at", "mime_type", "original_name", "stored_path", "category"}
	if action != "remove" {
		for _, field := range protectedFields {
			if _, exists := req.Metadata[field]; exists {
				httpError(w, http.StatusBadRequest, fmt.Sprintf("cannot modify system field '%s'", field))
				return
			}
		}
	}

	// Find file by ID (file_id can be hash or stored path)
	var fileMeta *storage.FileMetadata
	fileMeta = s.storage.Index().FindByHash(fileID)
	if fileMeta == nil {
		fileMeta = s.storage.Index().FindByStoredPath(fileID)
	}
	if fileMeta == nil {
		httpError(w, http.StatusNotFound, "file not found")
		return
	}

	// Perform the metadata update
	var updatedMeta *storage.FileMetadata
	var err error

	switch action {
	case "replace":
		updatedMeta, err = s.storage.Index().UpdateMetadata(fileMeta.Hash, func(old map[string]string) map[string]string {
			return req.Metadata
		})
	case "merge":
		updatedMeta, err = s.storage.Index().UpdateMetadata(fileMeta.Hash, func(old map[string]string) map[string]string {
			if old == nil {
				old = make(map[string]string)
			}
			for k, v := range req.Metadata {
				old[k] = v
			}
			return old
		})
	case "remove":
		updatedMeta, err = s.storage.Index().UpdateMetadata(fileMeta.Hash, func(old map[string]string) map[string]string {
			if old == nil {
				return old
			}
			for _, field := range req.Fields {
				delete(old, field)
			}
			return old
		})
	}

	if err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("update failed: %v", err))
		return
	}

	// Log metadata change for audit
	auditLog := map[string]any{
		"file_id":     fileID,
		"hash":        fileMeta.Hash,
		"stored_path": fileMeta.StoredPath,
		"action":      action,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}
	if action == "remove" {
		auditLog["fields_removed"] = req.Fields
	} else {
		auditLog["metadata_updated"] = req.Metadata
	}

	// Append audit log (best effort, don't fail request if logging fails)
	logPath := filepath.ToSlash(filepath.Join("metadata", "audit_log.ndjson"))
	if _, err := s.storage.AppendNDJSON(logPath, []map[string]any{auditLog}); err != nil {
		s.logger.Warn("failed to append metadata audit log", slog.Any("err", err))
	}

	// Return updated metadata
	response := map[string]any{
		"file_id":       fileID,
		"hash":          updatedMeta.Hash,
		"original_name": updatedMeta.OriginalName,
		"stored_path":   updatedMeta.StoredPath,
		"category":      updatedMeta.Category,
		"mime_type":     updatedMeta.MimeType,
		"size":          updatedMeta.Size,
		"uploaded_at":   updatedMeta.UploadedAt.Format(time.RFC3339),
		"metadata":      updatedMeta.Metadata,
	}

	writeJSON(w, http.StatusOK, response)
}

type metadataUpdateRequest struct {
	Action   string            `json:"action"`   // "replace", "merge", or "remove"
	Metadata map[string]string `json:"metadata"` // for replace/merge
	Fields   []string          `json:"fields"`   // for remove
}

func init() {
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr.MaxIdleConnsPerHost = 32
	}
}
