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
	"time"

	"github.com/Muneer320/RhinoBox/internal/config"
	"github.com/Muneer320/RhinoBox/internal/jsonschema"
	"github.com/Muneer320/RhinoBox/internal/media"
	"github.com/Muneer320/RhinoBox/internal/storage"
	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server wires everything together.
type Server struct {
	cfg         config.Config
	logger      *slog.Logger
	router      chi.Router
	storage     *storage.Manager
	categorizer *media.Categorizer
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
		categorizer: media.NewCategorizer(),
	}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	r := s.router
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", s.handleHealth)
	r.Post("/ingest/media", s.handleMediaIngest)
	r.Post("/ingest/json", s.handleJSONIngest)
}

// Run starts the HTTP server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	s.server = &http.Server{Addr: s.cfg.Addr, Handler: s.router}

	errCh := make(chan error, 1)
	go func() {
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

	mediaType, category := s.categorizer.Classify(mimeType, header.Filename, categoryHint)
	relPath, err := s.storage.StoreMedia([]string{mediaType, category}, header.Filename, reader)
	if err != nil {
		return nil, err
	}

	record := map[string]any{
		"path":          relPath,
		"mime_type":     mimeType,
		"media_type":    mediaType,
		"category":      category,
		"comment":       comment,
		"original_name": header.Filename,
		"uploaded_at":   time.Now().UTC().Format(time.RFC3339),
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
	decision := jsonschema.DecideStorage(req.Namespace, summary)

	batchRel := s.storage.NextJSONBatchPath(decision.Engine, req.Namespace)
	if _, err := s.storage.AppendNDJSON(batchRel, docs); err != nil {
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("store batch: %v", err))
		return
	}

	schemaPath := ""
	if decision.Engine == "sql" {
		schemaPayload := map[string]any{
			"table":   decision.Table,
			"ddl":     decision.Schema,
			"summary": decision.Summary,
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

func init() {
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr.MaxIdleConnsPerHost = 32
	}
}
