package media

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"runtime"
	"sync"
	"time"

	"github.com/Muneer320/RhinoBox/internal/storage"
)

const (
	// DefaultJobQueueSize is the buffer capacity for the job queue
	DefaultJobQueueSize = 10000
	// DefaultResultQueueSize is the buffer capacity for the result queue
	DefaultResultQueueSize = 10000
)

// ProcessJob represents a single file processing task
type ProcessJob struct {
	Header       *multipart.FileHeader
	CategoryHint string
	Comment      string
	JobID        string
	Index        int // Original position in batch for ordering
}

// ProcessResult represents the outcome of a file processing operation
type ProcessResult struct {
	JobID        string
	Index        int
	Success      bool
	Error        error
	Record       map[string]any
	Duration     time.Duration
}

// WorkerPool manages concurrent file processing operations
type WorkerPool struct {
	workers      int
	jobQueue     chan *ProcessJob
	resultQueue  chan *ProcessResult
	categorizer  *Categorizer
	storage      *storage.Manager
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	bufferPool   *sync.Pool
	started      bool
	mu           sync.Mutex
}

// NewWorkerPool creates a new worker pool with the specified configuration.
// If workers is 0, defaults to runtime.NumCPU() * 2
func NewWorkerPool(ctx context.Context, store *storage.Manager, workers int) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU() * 2
	}

	poolCtx, cancel := context.WithCancel(ctx)
	
	return &WorkerPool{
		workers:     workers,
		jobQueue:    make(chan *ProcessJob, DefaultJobQueueSize),
		resultQueue: make(chan *ProcessResult, DefaultResultQueueSize),
		categorizer: NewCategorizer(),
		storage:     store,
		ctx:         poolCtx,
		cancel:      cancel,
		bufferPool: &sync.Pool{
			New: func() interface{} {
				// 512 bytes is the standard size for MIME detection
				buf := make([]byte, 512)
				return &buf
			},
		},
	}
}

// Start initializes and starts all worker goroutines
func (wp *WorkerPool) Start() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	
	if wp.started {
		return fmt.Errorf("worker pool already started")
	}
	
	wp.started = true
	
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
	
	return nil
}

// Submit adds a new job to the processing queue.
// Returns error if the queue is full or the pool is shutting down.
func (wp *WorkerPool) Submit(job *ProcessJob) error {
	select {
	case wp.jobQueue <- job:
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	default:
		return fmt.Errorf("job queue is full")
	}
}

// Results returns the result channel for consuming processed results
func (wp *WorkerPool) Results() <-chan *ProcessResult {
	return wp.resultQueue
}

// Shutdown gracefully stops the worker pool.
// It closes the job queue, waits for workers to finish, and closes the result queue.
func (wp *WorkerPool) Shutdown() {
	wp.mu.Lock()
	if !wp.started {
		wp.mu.Unlock()
		return
	}
	wp.mu.Unlock()
	
	// Signal shutdown
	close(wp.jobQueue)
	
	// Wait for all workers to finish
	wp.wg.Wait()
	
	// Close result queue
	close(wp.resultQueue)
	
	// Cancel context
	wp.cancel()
}

// worker processes jobs from the queue until it's closed or context is cancelled
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	
	for {
		select {
		case <-wp.ctx.Done():
			return
		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}
			wp.processJob(job)
		}
	}
}

// processJob handles the actual file processing logic
func (wp *WorkerPool) processJob(job *ProcessJob) {
	startTime := time.Now()
	result := &ProcessResult{
		JobID:   job.JobID,
		Index:   job.Index,
		Success: false,
	}
	
	defer func() {
		result.Duration = time.Since(startTime)
		
		// Non-blocking send to result queue
		select {
		case wp.resultQueue <- result:
		case <-wp.ctx.Done():
		default:
			// Result queue full, log and drop (or could implement backpressure)
		}
	}()
	
	// Open the uploaded file
	file, err := job.Header.Open()
	if err != nil {
		result.Error = fmt.Errorf("open file: %w", err)
		return
	}
	defer file.Close()
	
	// Get buffer from pool for MIME detection
	bufPtr := wp.bufferPool.Get().(*[]byte)
	sniff := *bufPtr
	defer wp.bufferPool.Put(bufPtr)
	
	// Read first 512 bytes for MIME detection
	n, _ := io.ReadFull(file, sniff)
	
	// Detect MIME type
	mimeType := job.Header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = detectContentType(sniff[:n])
	}
	
	// Store the file using the storage manager
	metadata := map[string]string{}
	if job.Comment != "" {
		metadata["comment"] = job.Comment
	}
	
	// Create a reader that includes the sniffed bytes
	reader := io.MultiReader(
		newBytesReader(sniff[:n]),
		file,
	)
	
	storeResult, err := wp.storage.StoreFile(storage.StoreRequest{
		Reader:       reader,
		Filename:     job.Header.Filename,
		MimeType:     mimeType,
		Size:         job.Header.Size,
		Metadata:     metadata,
		CategoryHint: job.CategoryHint,
	})
	if err != nil {
		result.Error = fmt.Errorf("store file: %w", err)
		return
	}
	
	// Extract media type from category
	mediaType := storeResult.Metadata.Category
	if idx := len(mediaType); idx > 0 {
		for i, c := range mediaType {
			if c == '/' {
				mediaType = mediaType[:i]
				break
			}
		}
	}
	
	// Build result record
	record := map[string]any{
		"path":          storeResult.Metadata.StoredPath,
		"mime_type":     storeResult.Metadata.MimeType,
		"category":      storeResult.Metadata.Category,
		"media_type":    mediaType,
		"comment":       job.Comment,
		"original_name": storeResult.Metadata.OriginalName,
		"uploaded_at":   storeResult.Metadata.UploadedAt.Format(time.RFC3339),
		"hash":          storeResult.Metadata.Hash,
		"size":          storeResult.Metadata.Size,
	}
	if storeResult.Duplicate {
		record["duplicate"] = true
	}
	
	result.Record = record
	result.Success = true
}

// bytesReader wraps a byte slice to implement io.Reader
type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

func (br *bytesReader) Read(p []byte) (n int, err error) {
	if br.pos >= len(br.data) {
		return 0, io.EOF
	}
	n = copy(p, br.data[br.pos:])
	br.pos += n
	return n, nil
}

// detectContentType is a wrapper around http.DetectContentType
// Kept separate for easier testing and potential custom logic
func detectContentType(data []byte) string {
	// We use a simple approach here - in production might want
	// more sophisticated detection
	if len(data) < 12 {
		return "application/octet-stream"
	}
	
	// Basic MIME detection based on magic numbers
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8:
		return "image/jpeg"
	case len(data) >= 4 && string(data[:4]) == "\x89PNG":
		return "image/png"
	case len(data) >= 4 && string(data[:4]) == "GIF8":
		return "image/gif"
	case len(data) >= 12 && string(data[4:12]) == "ftypmp42":
		return "video/mp4"
	case len(data) >= 4 && string(data[:4]) == "RIFF":
		return "video/avi"
	case len(data) >= 4 && string(data[:4]) == "%PDF":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

// Stats returns current statistics about the worker pool
func (wp *WorkerPool) Stats() WorkerPoolStats {
	return WorkerPoolStats{
		Workers:         wp.workers,
		JobQueueLen:     len(wp.jobQueue),
		JobQueueCap:     cap(wp.jobQueue),
		ResultQueueLen:  len(wp.resultQueue),
		ResultQueueCap:  cap(wp.resultQueue),
	}
}

// WorkerPoolStats contains runtime statistics for the worker pool
type WorkerPoolStats struct {
	Workers        int
	JobQueueLen    int
	JobQueueCap    int
	ResultQueueLen int
	ResultQueueCap int
}
