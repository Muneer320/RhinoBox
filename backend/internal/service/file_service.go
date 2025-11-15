package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// FileService provides a clean interface for file operations, abstracting storage details from HTTP handlers.
type FileService struct {
	storage *storage.Manager
	logger  *slog.Logger
}

// NewFileService creates a new FileService instance.
func NewFileService(storage *storage.Manager, logger *slog.Logger) *FileService {
	return &FileService{
		storage: storage,
		logger:  logger,
	}
}

// StoreFile stores a file and returns a frontend-friendly response.
func (s *FileService) StoreFile(req FileStoreRequest) (*FileStoreResponse, error) {
	reader, ok := req.Reader.(io.Reader)
	if !ok {
		return nil, errors.New("reader must implement io.Reader")
	}

	storeReq := storage.StoreRequest{
		Reader:       reader,
		Filename:     req.Filename,
		MimeType:     req.MimeType,
		Size:         req.Size,
		Metadata:     req.Metadata,
		CategoryHint: req.CategoryHint,
	}

	result, err := s.storage.StoreFile(storeReq)
	if err != nil {
		return nil, fmt.Errorf("storage error: %w", err)
	}

	return &FileStoreResponse{
		Hash:         result.Metadata.Hash,
		OriginalName: result.Metadata.OriginalName,
		StoredPath:   result.Metadata.StoredPath,
		Category:     result.Metadata.Category,
		MimeType:     result.Metadata.MimeType,
		Size:         result.Metadata.Size,
		UploadedAt:   result.Metadata.UploadedAt,
		Metadata:     result.Metadata.Metadata,
		Duplicate:    result.Duplicate,
	}, nil
}

// GetFileByHash retrieves a file by hash and returns metadata.
func (s *FileService) GetFileByHash(hash string) (*storage.FileRetrievalResult, error) {
	if hash == "" {
		return nil, errors.New("hash is required")
	}

	result, err := s.storage.GetFileByHash(hash)
	if err != nil {
		return nil, fmt.Errorf("retrieval error: %w", err)
	}

	return result, nil
}

// GetFileByPath retrieves a file by path and returns metadata.
func (s *FileService) GetFileByPath(path string) (*storage.FileRetrievalResult, error) {
	if path == "" {
		return nil, errors.New("path is required")
	}

	result, err := s.storage.GetFileByPath(path)
	if err != nil {
		return nil, fmt.Errorf("retrieval error: %w", err)
	}

	return result, nil
}

// GetFileMetadata retrieves file metadata without opening the file.
func (s *FileService) GetFileMetadata(hash string) (*FileMetadataResponse, error) {
	if hash == "" {
		return nil, errors.New("hash is required")
	}

	metadata, err := s.storage.GetFileMetadata(hash)
	if err != nil {
		return nil, fmt.Errorf("metadata retrieval error: %w", err)
	}

	return &FileMetadataResponse{
		Hash:         metadata.Hash,
		OriginalName: metadata.OriginalName,
		StoredPath:   metadata.StoredPath,
		Category:     metadata.Category,
		MimeType:     metadata.MimeType,
		Size:         metadata.Size,
		UploadedAt:   metadata.UploadedAt,
		Metadata:     metadata.Metadata,
	}, nil
}

// RenameFile renames a file and returns a frontend-friendly response.
func (s *FileService) RenameFile(req FileRenameRequest) (*FileRenameResponse, error) {
	if req.Hash == "" {
		return nil, errors.New("hash is required")
	}
	if req.NewName == "" {
		return nil, errors.New("new_name is required")
	}

	storageReq := storage.RenameRequest{
		Hash:             req.Hash,
		NewName:          req.NewName,
		UpdateStoredFile: req.UpdateStoredFile,
	}

	result, err := s.storage.RenameFile(storageReq)
	if err != nil {
		return nil, fmt.Errorf("rename error: %w", err)
	}

	return &FileRenameResponse{
		Hash:          result.OldMetadata.Hash,
		OldName:       result.OldMetadata.OriginalName,
		NewName:       result.NewMetadata.OriginalName,
		OldStoredPath: result.OldMetadata.StoredPath,
		NewStoredPath: result.NewMetadata.StoredPath,
		Renamed:       result.Renamed,
		Message:       result.Message,
	}, nil
}

// DeleteFile deletes a file and returns a frontend-friendly response.
func (s *FileService) DeleteFile(req FileDeleteRequest) (*FileDeleteResponse, error) {
	if req.Hash == "" {
		return nil, errors.New("hash is required")
	}

	storageReq := storage.DeleteRequest{
		Hash: req.Hash,
	}

	result, err := s.storage.DeleteFile(storageReq)
	if err != nil {
		return nil, fmt.Errorf("delete error: %w", err)
	}

	return &FileDeleteResponse{
		Hash:         result.Hash,
		OriginalName: result.OriginalName,
		StoredPath:   result.StoredPath,
		Deleted:      result.Deleted,
		DeletedAt:    result.DeletedAt,
		Message:      result.Message,
	}, nil
}

// UpdateFileMetadata updates file metadata and returns a frontend-friendly response.
func (s *FileService) UpdateFileMetadata(req MetadataUpdateRequest) (*MetadataUpdateResponse, error) {
	if req.Hash == "" {
		return nil, errors.New("hash is required")
	}

	// Default to merge action if not specified
	action := req.Action
	if action == "" {
		action = "merge"
	}

	storageReq := storage.MetadataUpdateRequest{
		Hash:     req.Hash,
		Action:   action,
		Metadata: req.Metadata,
		Fields:   req.Fields,
	}

	result, err := s.storage.UpdateFileMetadata(storageReq)
	if err != nil {
		return nil, fmt.Errorf("metadata update error: %w", err)
	}

	// Add timestamp
	result.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	return &MetadataUpdateResponse{
		Hash:        result.Hash,
		OldMetadata: result.OldMetadata,
		NewMetadata: result.NewMetadata,
		Action:      result.Action,
		UpdatedAt:   result.UpdatedAt,
	}, nil
}

// BatchUpdateFileMetadata updates metadata for multiple files.
func (s *FileService) BatchUpdateFileMetadata(req BatchMetadataUpdateRequest) (*BatchMetadataUpdateResponse, error) {
	if len(req.Updates) == 0 {
		return nil, errors.New("no updates provided")
	}

	if len(req.Updates) > 100 {
		return nil, errors.New("too many updates (max 100)")
	}

	storageUpdates := make([]storage.MetadataUpdateRequest, len(req.Updates))
	for i, update := range req.Updates {
		storageUpdates[i] = storage.MetadataUpdateRequest{
			Hash:     update.Hash,
			Action:   update.Action,
			Metadata: update.Metadata,
			Fields:   update.Fields,
		}
	}

	results, errs := s.storage.BatchUpdateFileMetadata(storageUpdates)

	// Transform results to frontend-friendly format
	timestamp := time.Now().UTC().Format(time.RFC3339)
	successCount := 0
	failureCount := 0

	responseItems := make([]BatchMetadataUpdateItem, len(results))
	for i := range results {
		if errs[i] != nil {
			responseItems[i] = BatchMetadataUpdateItem{
				Hash:    req.Updates[i].Hash,
				Success: false,
				Error:   errs[i].Error(),
			}
			failureCount++
		} else {
			results[i].UpdatedAt = timestamp
			responseItems[i] = BatchMetadataUpdateItem{
				Hash:        results[i].Hash,
				Success:     true,
				OldMetadata: results[i].OldMetadata,
				NewMetadata: results[i].NewMetadata,
				Action:      results[i].Action,
				UpdatedAt:   results[i].UpdatedAt,
			}
			successCount++
		}
	}

	return &BatchMetadataUpdateResponse{
		Results:      responseItems,
		Total:        len(req.Updates),
		SuccessCount: successCount,
		FailureCount: failureCount,
	}, nil
}

// SearchFiles searches for files by original name.
func (s *FileService) SearchFiles(req FileSearchRequest) (*FileSearchResponse, error) {
	if req.Query == "" {
		return nil, errors.New("query is required")
	}

	results := s.storage.FindByOriginalName(req.Query)

	// Transform to frontend-friendly format
	responseResults := make([]FileMetadataResponse, len(results))
	for i, meta := range results {
		responseResults[i] = FileMetadataResponse{
			Hash:         meta.Hash,
			OriginalName: meta.OriginalName,
			StoredPath:   meta.StoredPath,
			Category:     meta.Category,
			MimeType:     meta.MimeType,
			Size:         meta.Size,
			UploadedAt:   meta.UploadedAt,
			Metadata:     meta.Metadata,
		}
	}

	return &FileSearchResponse{
		Query:   req.Query,
		Results: responseResults,
		Count:   len(responseResults),
	}, nil
}

// StoreMediaFile stores a media file using StoreMedia method.
func (s *FileService) StoreMediaFile(subdirs []string, originalName string, reader io.Reader) (string, error) {
	if originalName == "" {
		return "", errors.New("original_name is required")
	}
	if reader == nil {
		return "", errors.New("reader is required")
	}

	relPath, err := s.storage.StoreMedia(subdirs, originalName, reader)
	if err != nil {
		return "", fmt.Errorf("store media error: %w", err)
	}

	return relPath, nil
}

// StoreFileFromMultipart stores a file from multipart form data.
func (s *FileService) StoreFileFromMultipart(header *multipart.FileHeader, categoryHint, comment string) (*FileStoreResponse, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Sniff MIME type
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

	req := FileStoreRequest{
		Reader:       reader,
		Filename:     header.Filename,
		MimeType:     mimeType,
		Size:         header.Size,
		Metadata:     metadata,
		CategoryHint: categoryHint,
	}

	return s.StoreFile(req)
}

// LogDownload logs a download event.
func (s *FileService) LogDownload(log storage.DownloadLog) error {
	return s.storage.LogDownload(log)
}

// AppendNDJSON appends newline-delimited JSON documents.
func (s *FileService) AppendNDJSON(relPath string, docs []map[string]any) (string, error) {
	return s.storage.AppendNDJSON(relPath, docs)
}

// WriteJSONFile writes a JSON file.
func (s *FileService) WriteJSONFile(relPath string, payload any) (string, error) {
	return s.storage.WriteJSONFile(relPath, payload)
}

// NextJSONBatchPath returns a timestamped file path for new JSON batches.
func (s *FileService) NextJSONBatchPath(engine, namespace string) string {
	return s.storage.NextJSONBatchPath(engine, namespace)
}

// TransformStoreResultToRecord transforms a StoreFile result to a record format used in ingest logs.
func (s *FileService) TransformStoreResultToRecord(result *FileStoreResponse, comment string) map[string]any {
	mediaType := result.Category
	if idx := strings.Index(mediaType, "/"); idx > 0 {
		mediaType = mediaType[:idx]
	}

	record := map[string]any{
		"path":          result.StoredPath,
		"mime_type":     result.MimeType,
		"category":      result.Category,
		"media_type":    mediaType,
		"comment":       comment,
		"original_name": result.OriginalName,
		"uploaded_at":   result.UploadedAt.Format(time.RFC3339),
		"hash":          result.Hash,
		"size":          result.Size,
	}
	if result.Duplicate {
		record["duplicate"] = true
	}
	return record
}

// Notes operations

// GetNotes retrieves all notes for a file.
func (s *FileService) GetNotes(fileID string) ([]storage.Note, error) {
	return s.storage.GetNotes(fileID)
}

// AddNote adds a new note to a file.
func (s *FileService) AddNote(fileID, text, author string) (*storage.Note, error) {
	return s.storage.AddNote(fileID, text, author)
}

// UpdateNote updates an existing note.
func (s *FileService) UpdateNote(fileID, noteID, text string) (*storage.Note, error) {
	return s.storage.UpdateNote(fileID, noteID, text)
}

// DeleteNote removes a note.
func (s *FileService) DeleteNote(fileID, noteID string) error {
	return s.storage.DeleteNote(fileID, noteID)
}

