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
	r.Get("/files/search", s.handleFileSearch)
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

func (s *Server) handleFileSearch(w http.ResponseWriter, r *http.Request) {
// Get search query from URL parameter
query := r.URL.Query().Get("name")
if query == "" {
httpError(w, http.StatusBadRequest, "name query parameter is required")
return
}

results := s.storage.FindByOriginalName(query)

writeJSON(w, http.StatusOK, map[string]any{
"query":   query,
"results": results,
"count":   len(results),
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

func init() {
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr.MaxIdleConnsPerHost = 32
	}
}
