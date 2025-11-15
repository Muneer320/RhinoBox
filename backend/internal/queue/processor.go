package queue

import (
	"fmt"
	"mime/multipart"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

// MediaProcessor processes media upload jobs
type MediaProcessor struct {
	storage *storage.Manager
}

// NewMediaProcessor creates a new media processor
func NewMediaProcessor(storage *storage.Manager) *MediaProcessor {
	return &MediaProcessor{storage: storage}
}

// ProcessItem implements JobProcessor for media files
func (mp *MediaProcessor) ProcessItem(job *Job, item *JobItem) error {
	// Extract file handle from item data
	fileHeader, ok := item.Data.(*multipart.FileHeader)
	if !ok {
		return fmt.Errorf("invalid item data type")
	}

	// Open file
	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get category from item metadata or job namespace
	category := job.Namespace
	if item.Metadata != nil {
		if cat, ok := item.Metadata["category"].(string); ok && cat != "" {
			category = cat
		}
	}

	// Store file using storage manager
	req := storage.StoreRequest{
		Reader:       file,
		Filename:     fileHeader.Filename,
		MimeType:     fileHeader.Header.Get("Content-Type"),
		Size:         fileHeader.Size,
		Metadata:     map[string]string{
			"job_id":    job.ID,
			"namespace": job.Namespace,
		},
		CategoryHint: category,
	}

	result, err := mp.storage.StoreFile(req)
	if err != nil {
		return fmt.Errorf("failed to store file: %w", err)
	}

	// Store result in item
	item.Result = &JobItemResult{
		StoredPath:  result.Metadata.StoredPath,
		Hash:        result.Metadata.Hash,
		Category:    result.Metadata.Category,
		IsDuplicate: result.Duplicate,
		Metadata: map[string]interface{}{
			"mime_type": result.Metadata.MimeType,
			"size":      result.Metadata.Size,
		},
	}

	return nil
}
