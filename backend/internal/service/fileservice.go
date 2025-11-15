package service

import (
	"github.com/Muneer320/RhinoBox/internal/storage"
)

// FileService provides business logic for file operations.
// It abstracts storage operations and provides a clean interface for handlers.
type FileService interface {
	// StoreFile stores a file and returns metadata
	StoreFile(req storage.StoreRequest) (*storage.StoreResult, error)
	
	// RenameFile renames a file by hash
	RenameFile(req storage.RenameRequest) (*storage.RenameResult, error)
	
	// DeleteFile deletes a file by hash
	DeleteFile(req storage.DeleteRequest) (*storage.DeleteResult, error)
	
	// UpdateFileMetadata updates metadata for a file
	UpdateFileMetadata(req storage.MetadataUpdateRequest) (*storage.MetadataUpdateResult, error)
	
	// BatchUpdateFileMetadata updates metadata for multiple files
	BatchUpdateFileMetadata(updates []storage.MetadataUpdateRequest) ([]storage.MetadataUpdateResult, []error)
	
	// GetFileByHash retrieves a file by hash
	GetFileByHash(hash string) (*storage.FileRetrievalResult, error)
	
	// GetFileByPath retrieves a file by stored path
	GetFileByPath(path string) (*storage.FileRetrievalResult, error)
	
	// GetFileMetadata retrieves file metadata without opening the file
	GetFileMetadata(hash string) (*storage.FileMetadata, error)
	
	// SearchFiles searches for files by original name
	SearchFiles(query string) []storage.FileMetadata
	
	// LogDownload logs a download event
	LogDownload(log storage.DownloadLog) error

	// Notes operations
	GetNotes(fileID string) ([]storage.Note, error)
	AddNote(fileID, text, author string) (*storage.Note, error)
	UpdateNote(fileID, noteID, text string) (*storage.Note, error)
	DeleteNote(fileID, noteID string) error
}

// fileService implements FileService using storage.Manager
type fileService struct {
	storage *storage.Manager
}

// NewFileService creates a new FileService instance
func NewFileService(storage *storage.Manager) FileService {
	return &fileService{
		storage: storage,
	}
}

// StoreFile stores a file and returns metadata
func (s *fileService) StoreFile(req storage.StoreRequest) (*storage.StoreResult, error) {
	return s.storage.StoreFile(req)
}

// RenameFile renames a file by hash
func (s *fileService) RenameFile(req storage.RenameRequest) (*storage.RenameResult, error) {
	return s.storage.RenameFile(req)
}

// DeleteFile deletes a file by hash
func (s *fileService) DeleteFile(req storage.DeleteRequest) (*storage.DeleteResult, error) {
	return s.storage.DeleteFile(req)
}

// UpdateFileMetadata updates metadata for a file
func (s *fileService) UpdateFileMetadata(req storage.MetadataUpdateRequest) (*storage.MetadataUpdateResult, error) {
	return s.storage.UpdateFileMetadata(req)
}

// BatchUpdateFileMetadata updates metadata for multiple files
func (s *fileService) BatchUpdateFileMetadata(updates []storage.MetadataUpdateRequest) ([]storage.MetadataUpdateResult, []error) {
	return s.storage.BatchUpdateFileMetadata(updates)
}

// GetFileByHash retrieves a file by hash
func (s *fileService) GetFileByHash(hash string) (*storage.FileRetrievalResult, error) {
	return s.storage.GetFileByHash(hash)
}

// GetFileByPath retrieves a file by stored path
func (s *fileService) GetFileByPath(path string) (*storage.FileRetrievalResult, error) {
	return s.storage.GetFileByPath(path)
}

// GetFileMetadata retrieves file metadata without opening the file
func (s *fileService) GetFileMetadata(hash string) (*storage.FileMetadata, error) {
	return s.storage.GetFileMetadata(hash)
}

// SearchFiles searches for files by original name
func (s *fileService) SearchFiles(query string) []storage.FileMetadata {
	return s.storage.FindByOriginalName(query)
}

// LogDownload logs a download event
func (s *fileService) LogDownload(log storage.DownloadLog) error {
	return s.storage.LogDownload(log)
}

// GetNotes retrieves all notes for a file
func (s *fileService) GetNotes(fileID string) ([]storage.Note, error) {
	return s.storage.GetNotes(fileID)
}

// AddNote adds a new note to a file
func (s *fileService) AddNote(fileID, text, author string) (*storage.Note, error) {
	return s.storage.AddNote(fileID, text, author)
}

// UpdateNote updates an existing note
func (s *fileService) UpdateNote(fileID, noteID, text string) (*storage.Note, error) {
	return s.storage.UpdateNote(fileID, noteID, text)
}

// DeleteNote removes a note
func (s *fileService) DeleteNote(fileID, noteID string) error {
	return s.storage.DeleteNote(fileID, noteID)
}

