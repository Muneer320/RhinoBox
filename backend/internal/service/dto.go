package service

import (
	"time"
)

// FileStoreRequest represents a request to store a file.
type FileStoreRequest struct {
	Reader       interface{}       // io.Reader
	Filename     string
	MimeType     string
	Size         int64
	Metadata     map[string]string
	CategoryHint string
}

// FileStoreResponse represents the response after storing a file.
type FileStoreResponse struct {
	Hash         string            `json:"hash"`
	OriginalName string            `json:"original_name"`
	StoredPath   string            `json:"stored_path"`
	Category     string            `json:"category"`
	MimeType     string            `json:"mime_type"`
	Size         int64             `json:"size"`
	UploadedAt   time.Time         `json:"uploaded_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Duplicate    bool              `json:"duplicate"`
}

// FileRetrievalResponse represents file retrieval response.
type FileRetrievalResponse struct {
	Hash         string            `json:"hash"`
	OriginalName string            `json:"original_name"`
	StoredPath   string            `json:"stored_path"`
	Category     string            `json:"category"`
	MimeType     string            `json:"mime_type"`
	Size         int64             `json:"size"`
	UploadedAt   time.Time         `json:"uploaded_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// FileMetadataResponse represents file metadata response.
type FileMetadataResponse struct {
	Hash         string            `json:"hash"`
	OriginalName string            `json:"original_name"`
	StoredPath   string            `json:"stored_path"`
	Category     string            `json:"category"`
	MimeType     string            `json:"mime_type"`
	Size         int64             `json:"size"`
	UploadedAt   time.Time         `json:"uploaded_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// FileRenameRequest represents a request to rename a file.
type FileRenameRequest struct {
	Hash             string `json:"hash"`
	NewName          string `json:"new_name"`
	UpdateStoredFile bool   `json:"update_stored_file"`
}

// FileRenameResponse represents the response after renaming a file.
type FileRenameResponse struct {
	Hash         string            `json:"hash"`
	OldName      string            `json:"old_name"`
	NewName      string            `json:"new_name"`
	OldStoredPath string           `json:"old_stored_path"`
	NewStoredPath string           `json:"new_stored_path"`
	Renamed      bool              `json:"renamed"`
	Message      string            `json:"message,omitempty"`
}

// FileDeleteRequest represents a request to delete a file.
type FileDeleteRequest struct {
	Hash string `json:"hash"`
}

// FileDeleteResponse represents the response after deleting a file.
type FileDeleteResponse struct {
	Hash         string    `json:"hash"`
	OriginalName string    `json:"original_name"`
	StoredPath   string    `json:"stored_path"`
	Deleted      bool      `json:"deleted"`
	DeletedAt    time.Time `json:"deleted_at"`
	Message      string    `json:"message,omitempty"`
}

// MetadataUpdateRequest represents a request to update file metadata.
type MetadataUpdateRequest struct {
	Hash     string            `json:"hash"`
	Action   string            `json:"action"`   // "replace", "merge", "remove"
	Metadata map[string]string `json:"metadata"` // For replace/merge
	Fields   []string          `json:"fields"`   // For remove
}

// MetadataUpdateResponse represents the response after updating metadata.
type MetadataUpdateResponse struct {
	Hash        string            `json:"hash"`
	OldMetadata map[string]string `json:"old_metadata"`
	NewMetadata map[string]string `json:"new_metadata"`
	Action      string            `json:"action"`
	UpdatedAt   string            `json:"updated_at"`
}

// BatchMetadataUpdateRequest represents a batch metadata update request.
type BatchMetadataUpdateRequest struct {
	Updates []MetadataUpdateRequest `json:"updates"`
}

// BatchMetadataUpdateResponse represents the response after batch metadata update.
type BatchMetadataUpdateResponse struct {
	Results      []BatchMetadataUpdateItem `json:"results"`
	Total        int                       `json:"total"`
	SuccessCount int                       `json:"success_count"`
	FailureCount int                       `json:"failure_count"`
}

// BatchMetadataUpdateItem represents a single item in batch update response.
type BatchMetadataUpdateItem struct {
	Hash        string            `json:"hash"`
	Success     bool              `json:"success"`
	OldMetadata map[string]string `json:"old_metadata,omitempty"`
	NewMetadata map[string]string `json:"new_metadata,omitempty"`
	Action      string            `json:"action,omitempty"`
	UpdatedAt   string            `json:"updated_at,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// FileSearchRequest represents a file search request.
type FileSearchRequest struct {
	Query string `json:"query"`
}

// FileSearchResponse represents the response after searching files.
type FileSearchResponse struct {
	Query   string                 `json:"query"`
	Results []FileMetadataResponse `json:"results"`
	Count   int                    `json:"count"`
}

// FileDownloadRequest represents a file download request.
type FileDownloadRequest struct {
	Hash string
	Path string
}

// FileStreamRequest represents a file stream request.
type FileStreamRequest struct {
	Hash string
	Path string
}

