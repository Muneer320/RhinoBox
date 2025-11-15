package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// JobType represents the type of job to process
type JobType string

const (
	JobTypeMedia JobType = "media"
	JobTypeJSON  JobType = "json"
	JobTypeBatch JobType = "batch"
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	StatusQueued     JobStatus = "queued"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusCancelled  JobStatus = "cancelled"
)

// JobItem represents a single item to process in a job
type JobItem struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Name         string                 `json:"name"`
	Size         int64                  `json:"size"`
	Data         interface{}            `json:"data,omitempty"`
	Result       *JobItemResult         `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// JobItemResult holds the result of processing a single item
type JobItemResult struct {
	StoredPath  string                 `json:"stored_path"`
	Hash        string                 `json:"hash,omitempty"`
	Category    string                 `json:"category,omitempty"`
	IsDuplicate bool                   `json:"is_duplicate"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Job represents an asynchronous processing job
type Job struct {
	ID          string      `json:"id"`
	Type        JobType     `json:"type"`
	Status      JobStatus   `json:"status"`
	Items       []JobItem   `json:"items"`
	Progress    int         `json:"progress"`
	Total       int         `json:"total"`
	CreatedAt   time.Time   `json:"created_at"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Error       string      `json:"error,omitempty"`
	Namespace   string      `json:"namespace,omitempty"`
	Comment     string      `json:"comment,omitempty"`
	RetryCount  int         `json:"retry_count"`
	MaxRetries  int         `json:"max_retries"`
}

// JobResult aggregates the final result of a job
type JobResult struct {
	JobID       string        `json:"job_id"`
	Status      JobStatus     `json:"status"`
	Total       int           `json:"total"`
	Succeeded   int           `json:"succeeded"`
	Failed      int           `json:"failed"`
	Duration    time.Duration `json:"duration_ms"`
	Results     []JobItem     `json:"results"`
	Error       string        `json:"error,omitempty"`
}

// JobQueue manages asynchronous job processing
type JobQueue struct {
	pending     chan *Job
	processing  map[string]*Job
	completed   map[string]*JobResult
	mu          sync.RWMutex
	maxWorkers  int
	workers     []*Worker
	persistPath string
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// Worker processes jobs from the queue
type Worker struct {
	id         int
	queue      *JobQueue
	processor  JobProcessor
	stopCh     chan struct{}
}

// JobProcessor defines the interface for processing job items
type JobProcessor interface {
	ProcessItem(job *Job, item *JobItem) error
}

// Config holds job queue configuration
type Config struct {
	MaxWorkers  int
	PersistPath string
	MaxRetries  int
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxWorkers:  10,
		PersistPath: "./data/jobs",
		MaxRetries:  3,
	}
}

// New creates a new job queue
func New(cfg Config, processor JobProcessor) (*JobQueue, error) {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 10
	}

	// Ensure persist directory exists
	if err := os.MkdirAll(cfg.PersistPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create persist directory: %w", err)
	}

	jq := &JobQueue{
		pending:     make(chan *Job, 1000), // Buffer 1000 jobs
		processing:  make(map[string]*Job),
		completed:   make(map[string]*JobResult),
		maxWorkers:  cfg.MaxWorkers,
		workers:     make([]*Worker, cfg.MaxWorkers),
		persistPath: cfg.PersistPath,
		stopCh:      make(chan struct{}),
	}

	// Start workers
	for i := 0; i < cfg.MaxWorkers; i++ {
		worker := &Worker{
			id:        i,
			queue:     jq,
			processor: processor,
			stopCh:    make(chan struct{}),
		}
		jq.workers[i] = worker
		jq.wg.Add(1)
		go worker.start()
	}

	// Restore incomplete jobs on startup
	if err := jq.restore(); err != nil {
		return nil, fmt.Errorf("failed to restore jobs: %w", err)
	}

	return jq, nil
}

// Enqueue adds a job to the queue
func (jq *JobQueue) Enqueue(job *Job) error {
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	job.Status = StatusQueued
	job.Total = len(job.Items)

	// Persist job immediately
	if err := jq.persistJob(job); err != nil {
		return fmt.Errorf("failed to persist job: %w", err)
	}

	// Add to pending queue
	select {
	case jq.pending <- job:
		return nil
	default:
		return fmt.Errorf("queue is full")
	}
}

// Get retrieves a job by ID
func (jq *JobQueue) Get(jobID string) (*Job, bool) {
	jq.mu.RLock()
	defer jq.mu.RUnlock()

	// Check processing jobs
	if job, ok := jq.processing[jobID]; ok {
		return job, true
	}

	// Check completed jobs
	if result, ok := jq.completed[jobID]; ok {
		// Reconstruct job from result
		job := &Job{
			ID:     result.JobID,
			Status: result.Status,
			Total:  result.Total,
			Items:  result.Results,
			Error:  result.Error,
		}
		return job, true
	}

	// Try loading from disk
	job, err := jq.loadJob(jobID)
	if err == nil {
		return job, true
	}

	return nil, false
}

// GetResult retrieves the final result of a completed job
func (jq *JobQueue) GetResult(jobID string) (*JobResult, bool) {
	jq.mu.RLock()
	defer jq.mu.RUnlock()

	result, ok := jq.completed[jobID]
	return result, ok
}

// ListJobs returns all jobs (processing + completed)
func (jq *JobQueue) ListJobs() []*Job {
	jq.mu.RLock()
	defer jq.mu.RUnlock()

	jobs := make([]*Job, 0, len(jq.processing))
	for _, job := range jq.processing {
		jobs = append(jobs, job)
	}

	return jobs
}

// Stats returns queue statistics
func (jq *JobQueue) Stats() map[string]interface{} {
	jq.mu.RLock()
	defer jq.mu.RUnlock()

	return map[string]interface{}{
		"pending":    len(jq.pending),
		"processing": len(jq.processing),
		"completed":  len(jq.completed),
		"workers":    jq.maxWorkers,
	}
}

// Stop gracefully shuts down the queue
func (jq *JobQueue) Stop() {
	close(jq.stopCh)
	
	// Stop all workers
	for _, worker := range jq.workers {
		close(worker.stopCh)
	}
	
	// Wait for workers to finish
	jq.wg.Wait()
	
	// Persist remaining jobs
	jq.mu.RLock()
	for _, job := range jq.processing {
		jq.persistJob(job)
	}
	jq.mu.RUnlock()
}

// persistJob saves a job to disk
func (jq *JobQueue) persistJob(job *Job) error {
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}

	filename := filepath.Join(jq.persistPath, fmt.Sprintf("%s.json", job.ID))
	return os.WriteFile(filename, data, 0644)
}

// loadJob loads a job from disk
func (jq *JobQueue) loadJob(jobID string) (*Job, error) {
	filename := filepath.Join(jq.persistPath, fmt.Sprintf("%s.json", jobID))
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}

	return &job, nil
}

// deleteJob removes a job file from disk
func (jq *JobQueue) deleteJob(jobID string) error {
	filename := filepath.Join(jq.persistPath, fmt.Sprintf("%s.json", jobID))
	return os.Remove(filename)
}

// restore loads incomplete jobs from disk and re-queues them
func (jq *JobQueue) restore() error {
	entries, err := os.ReadDir(jq.persistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No jobs to restore
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		jobID := strings.TrimSuffix(entry.Name(), ".json")
		job, err := jq.loadJob(jobID)
		if err != nil {
			continue // Skip corrupted jobs
		}

		// Only restore incomplete jobs
		if job.Status == StatusQueued || job.Status == StatusProcessing {
			job.Status = StatusQueued // Reset to queued
			job.StartedAt = nil
			
			select {
			case jq.pending <- job:
			default:
				// Queue full, skip this job
			}
		}
	}

	return nil
}

// Worker implementation
func (w *Worker) start() {
	defer w.queue.wg.Done()

	for {
		select {
		case <-w.stopCh:
			return
		case job := <-w.queue.pending:
			w.processJob(job)
		}
	}
}

func (w *Worker) processJob(job *Job) {
	now := time.Now()
	job.StartedAt = &now
	job.Status = StatusProcessing

	// Move to processing map
	w.queue.mu.Lock()
	w.queue.processing[job.ID] = job
	w.queue.mu.Unlock()

	// Persist state
	w.queue.persistJob(job)

	// Process each item
	succeeded := 0
	failed := 0

	for i := range job.Items {
		item := &job.Items[i]
		
		err := w.processor.ProcessItem(job, item)
		if err != nil {
			item.Error = err.Error()
			failed++
		} else {
			succeeded++
		}

		job.Progress++
		
		// Persist progress every 10 items
		if job.Progress%10 == 0 {
			w.queue.persistJob(job)
		}
	}

	// Complete job
	completedAt := time.Now()
	job.CompletedAt = &completedAt
	
	if failed == 0 {
		job.Status = StatusCompleted
	} else if succeeded == 0 {
		job.Status = StatusFailed
		job.Error = fmt.Sprintf("all %d items failed", failed)
	} else {
		job.Status = StatusCompleted
		job.Error = fmt.Sprintf("partial success: %d succeeded, %d failed", succeeded, failed)
	}

	// Create result
	result := &JobResult{
		JobID:     job.ID,
		Status:    job.Status,
		Total:     job.Total,
		Succeeded: succeeded,
		Failed:    failed,
		Duration:  completedAt.Sub(*job.StartedAt),
		Results:   job.Items,
		Error:     job.Error,
	}

	// Move to completed
	w.queue.mu.Lock()
	delete(w.queue.processing, job.ID)
	w.queue.completed[job.ID] = result
	w.queue.mu.Unlock()

	// Persist final state
	w.queue.persistJob(job)
}
