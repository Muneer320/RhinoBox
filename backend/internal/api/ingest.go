package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Muneer320/RhinoBox/internal/jsonschema"
	"github.com/Muneer320/RhinoBox/internal/service"
)

// UnifiedIngestRequest represents the unified /ingest payload.
type UnifiedIngestRequest struct {
	Namespace string         `json:"namespace"`
	Comment   string         `json:"comment"`
	Metadata  map[string]any `json:"metadata"`
}

// UnifiedIngestResponse combines results from all processing pipelines.
type UnifiedIngestResponse struct {
	JobID   string               `json:"job_id"`
	Status  string               `json:"status"` // "completed", "processing", "queued"
	Results UnifiedIngestResults `json:"results"`
	Timing  map[string]int64     `json:"timing"`
	Errors  []string             `json:"errors,omitempty"`
}

type UnifiedIngestResults struct {
	Media []MediaResult   `json:"media,omitempty"`
	JSON  []JSONResult    `json:"json,omitempty"`
	Files []GenericResult `json:"files,omitempty"`
}

type MediaResult struct {
	OriginalName      string         `json:"original_name"`
	StoredPath        string         `json:"stored_path"`
	Category          string         `json:"category"`
	MimeType          string         `json:"mime_type"`
	DetectedMimeType  string         `json:"detected_mime_type,omitempty"`
	UserOverrideType  string         `json:"user_override_type,omitempty"`
	ActualCategory    string         `json:"actual_category,omitempty"`
	Size              int64          `json:"size"`
	Hash              string         `json:"hash,omitempty"`
	Duplicates        bool           `json:"duplicates"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type JSONResult struct {
	StorageType           string              `json:"storage_type"` // "sql" or "nosql"
	Database              string              `json:"database,omitempty"`
	TableOrCollection     string              `json:"table_or_collection"`
	RecordsInserted       int                 `json:"records_inserted"`
	SchemaCreated         bool                `json:"schema_created"`
	RelationshipsDetected []string            `json:"relationships_detected,omitempty"`
	Decision              jsonschema.Decision `json:"decision,omitempty"`
	BatchPath             string              `json:"batch_path"`
}

type GenericResult struct {
	OriginalName     string `json:"original_name"`
	StoredPath       string `json:"stored_path"`
	FileType         string `json:"file_type"`
	DetectedMimeType string `json:"detected_mime_type,omitempty"`
	UserOverrideType string `json:"user_override_type,omitempty"`
	ActualCategory   string `json:"actual_category,omitempty"`
	Size             int64  `json:"size"`
	Hash             string `json:"hash,omitempty"`
	Unrecognized     bool   `json:"unrecognized,omitempty"`    // true if format not recognized
	RequiresRouting  bool   `json:"requires_routing,omitempty"` // true if user needs to suggest routing
}

// handleUnifiedIngest routes incoming data to appropriate pipelines based on content type.
func (s *Server) handleUnifiedIngest(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	if err := r.ParseMultipartForm(s.cfg.MaxUploadBytes); err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid multipart payload: %v", err))
		return
	}

	namespace := r.FormValue("namespace")
	comment := r.FormValue("comment")
	metadataStr := r.FormValue("metadata")
	dataStr := r.FormValue("data")

	var metadata map[string]any
	if metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid metadata JSON: %v", err))
			return
		}
	}

	response := UnifiedIngestResponse{
		JobID:  generateJobID(),
		Status: "completed",
		Timing: make(map[string]int64),
		Results: UnifiedIngestResults{
			Media: []MediaResult{},
			JSON:  []JSONResult{},
			Files: []GenericResult{},
		},
	}

	// Get file type override from form
	overrideType := r.FormValue("file_type_override")
	if overrideType == "" {
		overrideType = "auto"
	}

	// Validate override
	if !isValidOverride(overrideType) {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("invalid file_type_override value: %s (must be one of: auto, image, video, audio, document, code)", overrideType))
		return
	}

	// Process files (media, JSON, or generic)
	if r.MultipartForm != nil && len(r.MultipartForm.File) > 0 {
		processingStart := time.Now()
		for fieldName, headers := range r.MultipartForm.File {
			for _, header := range headers {
				result, err := s.routeFile(header, fieldName, comment, namespace, overrideType)
				if err != nil {
					response.Errors = append(response.Errors, fmt.Sprintf("%s: %v", header.Filename, err))
					continue
				}

				switch res := result.(type) {
				case MediaResult:
					response.Results.Media = append(response.Results.Media, res)
				case JSONResult:
					response.Results.JSON = append(response.Results.JSON, res)
				case GenericResult:
					response.Results.Files = append(response.Results.Files, res)
				}
			}
		}
		response.Timing["processing_ms"] = time.Since(processingStart).Milliseconds()
	}

	// Process inline JSON data
	if dataStr != "" {
		jsonStart := time.Now()
		result, err := s.processInlineJSON(dataStr, namespace, comment, metadata)
		if err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("JSON processing: %v", err))
		} else {
			response.Results.JSON = append(response.Results.JSON, result)
		}
		response.Timing["json_ms"] = time.Since(jsonStart).Milliseconds()
	}

	response.Timing["total_ms"] = time.Since(startTime).Milliseconds()

	if len(response.Errors) > 0 && len(response.Results.Media) == 0 && len(response.Results.JSON) == 0 && len(response.Results.Files) == 0 {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("all items failed: %v", response.Errors))
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// routeFile determines content type and routes to appropriate pipeline.
func (s *Server) routeFile(header *multipart.FileHeader, fieldName, comment, namespace, overrideType string) (any, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	detectedMimeType := detectMIMEType(header)
	ext := strings.ToLower(filepath.Ext(header.Filename))

	s.logger.Debug("routing file",
		slog.String("name", header.Filename),
		slog.String("mime", detectedMimeType),
		slog.String("field", fieldName),
		slog.String("override", overrideType))

	// Check if format is recognized
	classifier := s.storage.Classifier()
	rulesMgr := s.storage.RoutingRules()
	
	isRecognized := classifier.IsRecognized(detectedMimeType, header.Filename)
	hasCustomRule := rulesMgr != nil && rulesMgr.FindRule(detectedMimeType, ext) != nil
	isUnrecognized := !isRecognized && !hasCustomRule

	// Route based on MIME type or override
	if isMediaType(detectedMimeType) || (overrideType != "auto" && overrideType != "" && (overrideType == "image" || overrideType == "video" || overrideType == "audio")) {
		return s.processMediaFile(header, comment, detectedMimeType, overrideType)
	}

	if isJSONType(detectedMimeType) {
		return s.processJSONFile(header, namespace, comment)
	}

	// For generic files, check if unrecognized
	result, err := s.processGenericFile(header, namespace, detectedMimeType, overrideType)
	if err != nil {
		return result, err
	}

	// Mark as unrecognized if needed
	if isUnrecognized {
		result.Unrecognized = true
		result.RequiresRouting = true
	}

	return result, nil
}

// processMediaFile handles images, videos, audio.
func (s *Server) processMediaFile(header *multipart.FileHeader, comment, detectedMimeType, overrideType string) (MediaResult, error) {
	file, err := header.Open()
	if err != nil {
		return MediaResult{}, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Determine actual category based on override
	actualCategory := determineStorageCategory(detectedMimeType, overrideType)
	
	// Sanitize user-controlled inputs to prevent path traversal
	sanitizedComment := sanitizePathSegment(comment)
	sanitizedFilename := sanitizePathSegment(header.Filename)
	if sanitizedFilename == "" {
		sanitizedFilename = "file" + strings.ToLower(filepath.Ext(header.Filename))
	}

	metadata := map[string]string{}
	if comment != "" {
		metadata["comment"] = comment
	}
	if overrideType != "" && overrideType != "auto" {
		metadata["user_override_type"] = overrideType
		metadata["detected_mime_type"] = detectedMimeType
	}

	// Use override category hint if override is specified
	categoryHint := sanitizedComment
	if overrideType != "" && overrideType != "auto" {
		categoryHint = overrideType
	}

	req := service.FileStoreRequest{
		Reader:       file,
		Filename:     sanitizedFilename,
		MimeType:     detectedMimeType,
		Size:         header.Size,
		Metadata:     metadata,
		CategoryHint: categoryHint,
	}

	// If override is specified, use it as category hint to force storage path
	if overrideType != "" && overrideType != "auto" {
		req.CategoryHint = overrideType
	}

	result, err := s.fileService.StoreFile(req)
	if err != nil {
		return MediaResult{}, err
	}

	// Log if override differs from detection
	if overrideType != "" && overrideType != "auto" {
		detectedCategory := categoryFromMime(detectedMimeType)
		if detectedCategory != actualCategory {
			s.logger.Warn("File type override",
				slog.String("detected", detectedMimeType),
				slog.String("override", overrideType),
				slog.String("category", actualCategory))
		}
	}

	return MediaResult{
		OriginalName:     header.Filename,
		StoredPath:       result.StoredPath,
		Category:         result.Category,
		MimeType:         result.MimeType,
		DetectedMimeType: detectedMimeType,
		UserOverrideType: overrideType,
		ActualCategory:   actualCategory,
		Size:             result.Size,
		Hash:             result.Hash,
		Duplicates:       result.Duplicate,
	}, nil
}

// processGenericFile handles PDFs, documents, archives, etc.
func (s *Server) processGenericFile(header *multipart.FileHeader, namespace, detectedMimeType, overrideType string) (GenericResult, error) {
	file, err := header.Open()
	if err != nil {
		return GenericResult{}, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Determine actual category based on override
	actualCategory := determineStorageCategory(detectedMimeType, overrideType)
	
	category := actualCategory
	if namespace != "" {
		category = namespace
	}

	// If override is specified, use it for storage path
	if overrideType != "" && overrideType != "auto" {
		category = overrideType
	}

	relPath, err := s.fileService.StoreMediaFile([]string{"files", category}, header.Filename, file)
	if err != nil {
		return GenericResult{}, err
	}

	return GenericResult{
		OriginalName:     header.Filename,
		StoredPath:       relPath,
		FileType:         detectedMimeType,
		DetectedMimeType: detectedMimeType,
		UserOverrideType: overrideType,
		ActualCategory:   actualCategory,
		Size:             header.Size,
	}, nil
}

// processJSONFile handles JSON files uploaded through multipart form.
func (s *Server) processJSONFile(header *multipart.FileHeader, namespace, comment string) (JSONResult, error) {
	file, err := header.Open()
	if err != nil {
		return JSONResult{}, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Read the JSON content
	data, err := io.ReadAll(file)
	if err != nil {
		return JSONResult{}, fmt.Errorf("read file: %w", err)
	}

	// Process using the inline JSON handler
	return s.processInlineJSON(string(data), namespace, comment, nil)
}

// processInlineJSON handles JSON data from request body or form field.
func (s *Server) processInlineJSON(dataStr, namespace, comment string, metadata map[string]any) (JSONResult, error) {
	var data any
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return JSONResult{}, fmt.Errorf("invalid JSON: %w", err)
	}

	var docs []map[string]any
	switch v := data.(type) {
	case map[string]any:
		docs = []map[string]any{v}
	case []any:
		for _, item := range v {
			if doc, ok := item.(map[string]any); ok {
				docs = append(docs, doc)
			}
		}
	default:
		return JSONResult{}, fmt.Errorf("data must be object or array of objects")
	}

	if len(docs) == 0 {
		return JSONResult{}, fmt.Errorf("no valid documents in data")
	}

	analyzer := jsonschema.NewAnalyzer(4, 256)
	analyzer.AnalyzeBatch(docs)
	summary := analyzer.BuildSummary()
	analysis := analyzer.AnalyzeStructure(docs, summary)
	analysis = jsonschema.IncorporateCommentHints(analysis, comment)
	decision := jsonschema.DecideStorage(namespace, docs, summary, analysis)

	batchRel := s.fileService.NextJSONBatchPath(decision.Engine, namespace)
	if _, err := s.fileService.AppendNDJSON(batchRel, docs); err != nil {
		return JSONResult{}, fmt.Errorf("store batch: %w", err)
	}

	schemaCreated := false
	if decision.Engine == "sql" && decision.Schema != "" {
		schemaCreated = true
	}

	return JSONResult{
		StorageType:       decision.Engine,
		TableOrCollection: decision.Table,
		RecordsInserted:   len(docs),
		SchemaCreated:     schemaCreated,
		Decision:          decision,
		BatchPath:         batchRel,
	}, nil
}

func detectMIMEType(header *multipart.FileHeader) string {
	if ct := header.Header.Get("Content-Type"); ct != "" && ct != "application/octet-stream" {
		return ct
	}
	
	// Fallback to extension-based detection
	ext := filepath.Ext(header.Filename)
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".mkv":
		return "video/x-matroska"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".pdf":
		return "application/pdf"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func isMediaType(mime string) bool {
	switch {
	case len(mime) >= 6 && mime[:6] == "image/":
		return true
	case len(mime) >= 6 && mime[:6] == "video/":
		return true
	case len(mime) >= 6 && mime[:6] == "audio/":
		return true
	default:
		return false
	}
}

func isJSONType(mime string) bool {
	return mime == "application/json" || mime == "text/json"
}

func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}

var pathSegmentPattern = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// sanitizePathSegment removes path separators, OS-specific characters, and enforces
// a safe character whitelist to prevent path traversal and invalid filenames.
func sanitizePathSegment(input string) string {
	// Trim whitespace
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}
	
	// Remove or replace path separators and dangerous characters
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "\\", "")
	s = strings.ReplaceAll(s, "..", "")
	
	// Enforce safe character whitelist (alphanumerics, hyphen, underscore, dot)
	s = pathSegmentPattern.ReplaceAllString(s, "_")
	
	// Trim leading/trailing separators that might have been converted
	s = strings.Trim(s, "_.-")
	
	// Cap length to 100 characters
	if len(s) > 100 {
		s = s[:100]
	}
	
	return s
}

// isValidOverride validates that the override type is one of the allowed values.
func isValidOverride(override string) bool {
	validTypes := []string{"auto", "image", "video", "audio", "document", "code"}
	for _, t := range validTypes {
		if override == t {
			return true
		}
	}
	return false
}

// determineStorageCategory determines the storage category based on MIME type and override.
func determineStorageCategory(mimeType, override string) string {
	if override != "" && override != "auto" {
		// Use override
		switch override {
		case "image":
			return "images"
		case "video":
			return "videos"
		case "audio":
			return "audio"
		case "document":
			return "documents"
		case "code":
			return "code"
		default:
			return categoryFromMime(mimeType)
		}
	}
	return categoryFromMime(mimeType)
}

// categoryFromMime determines category from MIME type.
func categoryFromMime(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "images"
	case strings.HasPrefix(mimeType, "video/"):
		return "videos"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case mimeType == "application/pdf",
		strings.Contains(mimeType, "document"),
		strings.Contains(mimeType, "text/"):
		return "documents"
	case strings.Contains(mimeType, "javascript"),
		strings.Contains(mimeType, "python"),
		strings.Contains(mimeType, "java"),
		strings.Contains(mimeType, "go"),
		strings.Contains(mimeType, "c++"):
		return "code"
	default:
		return "other"
	}
}
