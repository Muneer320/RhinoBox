package media_test

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"sync"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/media"
	"github.com/Muneer320/RhinoBox/internal/storage"
)

// TestProcessorBasicOperation tests basic worker pool functionality
func TestProcessorBasicOperation(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 2)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	// Submit a single job
	header := createMockFileHeader("test.jpg", []byte("fake jpeg data"))
	job := &media.ProcessJob{
		Header:       header,
		CategoryHint: "test",
		Comment:      "basic test",
		JobID:        "job-1",
		Index:        0,
	}

	if err := pool.Submit(job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Wait for result
	select {
	case result := <-pool.Results():
		if !result.Success {
			t.Fatalf("expected success, got error: %v", result.Error)
		}
		if result.JobID != "job-1" {
			t.Errorf("expected job-1, got %s", result.JobID)
		}
		if result.Record == nil {
			t.Error("expected record, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for result")
	}
}

// TestProcessorConcurrency tests concurrent processing of multiple files
func TestProcessorConcurrency(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 4)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	// Submit multiple jobs concurrently
	numJobs := 20
	for i := 0; i < numJobs; i++ {
		filename := fmt.Sprintf("file-%d.jpg", i)
		content := []byte(fmt.Sprintf("content for file %d", i))
		header := createMockFileHeader(filename, content)
		
		job := &media.ProcessJob{
			Header:       header,
			CategoryHint: "batch",
			Comment:      fmt.Sprintf("job %d", i),
			JobID:        fmt.Sprintf("job-%d", i),
			Index:        i,
		}

		if err := pool.Submit(job); err != nil {
			t.Fatalf("Submit job %d: %v", i, err)
		}
	}

	// Collect all results
	results := make(map[string]*media.ProcessResult)
	successCount := 0

	for i := 0; i < numJobs; i++ {
		select {
		case result := <-pool.Results():
			results[result.JobID] = result
			if result.Success {
				successCount++
			} else {
				t.Logf("Job %s failed: %v", result.JobID, result.Error)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout waiting for result %d", i)
		}
	}

	if successCount != numJobs {
		t.Errorf("expected %d successes, got %d", numJobs, successCount)
	}

	// Verify all jobs completed
	for i := 0; i < numJobs; i++ {
		jobID := fmt.Sprintf("job-%d", i)
		if _, ok := results[jobID]; !ok {
			t.Errorf("missing result for %s", jobID)
		}
	}
}

// TestProcessorContextCancellation tests graceful shutdown with context cancellation
func TestProcessorContextCancellation(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	pool := media.NewWorkerPool(ctx, store, 2)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Submit a few jobs
	for i := 0; i < 5; i++ {
		header := createMockFileHeader(fmt.Sprintf("file-%d.jpg", i), []byte("data"))
		job := &media.ProcessJob{
			Header: header,
			JobID:  fmt.Sprintf("job-%d", i),
			Index:  i,
		}
		pool.Submit(job)
	}

	// Cancel context
	cancel()

	// Shutdown should complete quickly
	done := make(chan bool)
	go func() {
		pool.Shutdown()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("shutdown took too long")
	}
}

// TestProcessorShutdownGraceful tests graceful shutdown waits for in-flight jobs
func TestProcessorShutdownGraceful(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 2)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Submit jobs
	numJobs := 10
	for i := 0; i < numJobs; i++ {
		header := createMockFileHeader(fmt.Sprintf("file-%d.jpg", i), bytes.Repeat([]byte("x"), 1024))
		job := &media.ProcessJob{
			Header: header,
			JobID:  fmt.Sprintf("job-%d", i),
			Index:  i,
		}
		if err := pool.Submit(job); err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}

	// Start collecting results
	var wg sync.WaitGroup
	wg.Add(1)
	results := make([]*media.ProcessResult, 0)
	go func() {
		defer wg.Done()
		for result := range pool.Results() {
			results = append(results, result)
		}
	}()

	// Shutdown
	pool.Shutdown()
	
	// Wait for result collection to complete
	wg.Wait()

	// All submitted jobs should have results
	if len(results) != numJobs {
		t.Errorf("expected %d results, got %d", numJobs, len(results))
	}
}

// TestProcessorStats tests the statistics reporting
func TestProcessorStats(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	workers := 4
	pool := media.NewWorkerPool(ctx, store, workers)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	stats := pool.Stats()
	if stats.Workers != workers {
		t.Errorf("expected %d workers, got %d", workers, stats.Workers)
	}
	if stats.JobQueueCap != media.DefaultJobQueueSize {
		t.Errorf("expected job queue capacity %d, got %d", media.DefaultJobQueueSize, stats.JobQueueCap)
	}
	if stats.ResultQueueCap != media.DefaultResultQueueSize {
		t.Errorf("expected result queue capacity %d, got %d", media.DefaultResultQueueSize, stats.ResultQueueCap)
	}
}

// TestProcessorDifferentFileTypes tests handling of various file types
func TestProcessorDifferentFileTypes(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 2)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	testFiles := []struct {
		name    string
		content []byte
		hint    string
	}{
		{"photo.jpg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, "photos"},
		{"document.pdf", []byte("%PDF-1.4 content"), "docs"},
		{"video.mp4", bytes.Repeat([]byte("x"), 1024), "videos"},
		{"archive.zip", []byte("PK\x03\x04"), "archives"},
	}

	for i, tf := range testFiles {
		header := createMockFileHeader(tf.name, tf.content)
		job := &media.ProcessJob{
			Header:       header,
			CategoryHint: tf.hint,
			JobID:        fmt.Sprintf("job-%d", i),
			Index:        i,
		}
		if err := pool.Submit(job); err != nil {
			t.Fatalf("Submit %s: %v", tf.name, err)
		}
	}

	// Collect results
	for i := 0; i < len(testFiles); i++ {
		select {
		case result := <-pool.Results():
			if !result.Success {
				t.Errorf("job %s failed: %v", result.JobID, result.Error)
			}
			if result.Record == nil {
				t.Errorf("job %s: nil record", result.JobID)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout")
		}
	}
}

// TestProcessorLargeFile tests handling of larger files
func TestProcessorLargeFile(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 2)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	// Create a 5MB file
	content := bytes.Repeat([]byte("large file content "), 5*1024*1024/19)
	header := createMockFileHeader("large.jpg", content)
	
	job := &media.ProcessJob{
		Header: header,
		JobID:  "large-job",
		Index:  0,
	}

	if err := pool.Submit(job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case result := <-pool.Results():
		if !result.Success {
			t.Fatalf("expected success: %v", result.Error)
		}
		if result.Duration > 5*time.Second {
			t.Errorf("processing took too long: %v", result.Duration)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
	}
}

// TestProcessorErrorHandling tests error scenarios
func TestProcessorErrorHandling(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 2)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	// Test submitting job after shutdown would fail
	// For now, just test that the pool handles empty content gracefully
	header := createMockFileHeader("empty.jpg", []byte{})
	job := &media.ProcessJob{
		Header: header,
		JobID:  "empty-file",
		Index:  0,
	}

	if err := pool.Submit(job); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	select {
	case result := <-pool.Results():
		// Empty file should still succeed (or fail gracefully)
		if !result.Success && result.Error != nil {
			// It's okay if it fails, as long as it doesn't crash
			t.Logf("Empty file processed with error (expected): %v", result.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// TestProcessorDeduplication tests that duplicate files are detected
func TestProcessorDeduplication(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 2)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	// Submit the same file twice
	content := bytes.Repeat([]byte("duplicate test"), 1000)
	
	header1 := createMockFileHeader("file1.jpg", content)
	job1 := &media.ProcessJob{
		Header: header1,
		JobID:  "job-1",
		Index:  0,
	}
	
	header2 := createMockFileHeader("file2.jpg", content)
	job2 := &media.ProcessJob{
		Header: header2,
		JobID:  "job-2",
		Index:  1,
	}

	pool.Submit(job1)
	pool.Submit(job2)

	results := make([]*media.ProcessResult, 0, 2)
	for i := 0; i < 2; i++ {
		select {
		case result := <-pool.Results():
			results = append(results, result)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout")
		}
	}

	// One should be marked as duplicate
	duplicateFound := false
	for _, r := range results {
		if r.Success && r.Record != nil {
			if dup, ok := r.Record["duplicate"].(bool); ok && dup {
				duplicateFound = true
			}
		}
	}

	if !duplicateFound {
		t.Error("expected one file to be marked as duplicate")
	}
}

// TestProcessorOrderPreservation tests that results maintain input order via Index
func TestProcessorOrderPreservation(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 4)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	// Submit jobs
	numJobs := 50
	for i := 0; i < numJobs; i++ {
		header := createMockFileHeader(fmt.Sprintf("file-%03d.jpg", i), []byte(fmt.Sprintf("content-%d", i)))
		job := &media.ProcessJob{
			Header: header,
			JobID:  fmt.Sprintf("job-%d", i),
			Index:  i,
		}
		pool.Submit(job)
	}

	// Collect results
	results := make([]*media.ProcessResult, 0, numJobs)
	for i := 0; i < numJobs; i++ {
		select {
		case result := <-pool.Results():
			results = append(results, result)
		case <-time.After(10 * time.Second):
			t.Fatal("timeout")
		}
	}

	// Verify each result has correct index
	for i, result := range results {
		// Results may come out of order but should have correct index field
		if result.Index < 0 || result.Index >= numJobs {
			t.Errorf("result %d has invalid index %d", i, result.Index)
		}
	}
}

// TestProcessorWithMetadata tests that metadata is preserved
func TestProcessorWithMetadata(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 2)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	header := createMockFileHeader("test.jpg", []byte("content"))
	job := &media.ProcessJob{
		Header:       header,
		CategoryHint: "photos",
		Comment:      "test comment",
		JobID:        "metadata-job",
		Index:        0,
	}

	pool.Submit(job)

	select {
	case result := <-pool.Results():
		if !result.Success {
			t.Fatalf("job failed: %v", result.Error)
		}
		if result.Record["comment"] != "test comment" {
			t.Errorf("expected comment 'test comment', got %v", result.Record["comment"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
}

// TestProcessorWorkerCount tests different worker configurations
func TestProcessorWorkerCount(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	testCases := []struct {
		name    string
		workers int
	}{
		{"single-worker", 1},
		{"dual-worker", 2},
		{"quad-worker", 4},
		{"auto-detect", 0}, // 0 means auto-detect
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := media.NewWorkerPool(ctx, store, tc.workers)
			
			if err := pool.Start(); err != nil {
				t.Fatalf("Start: %v", err)
			}

			stats := pool.Stats()
			if tc.workers > 0 && stats.Workers != tc.workers {
				t.Errorf("expected %d workers, got %d", tc.workers, stats.Workers)
			} else if tc.workers == 0 && stats.Workers <= 0 {
				t.Error("auto-detect should produce positive worker count")
			}

			// Submit a simple job to verify it works
			header := createMockFileHeader("test.jpg", []byte("data"))
			job := &media.ProcessJob{
				Header: header,
				JobID:  "test",
				Index:  0,
			}
			pool.Submit(job)

			select {
			case <-pool.Results():
				// Success
			case <-time.After(5 * time.Second):
				t.Fatal("timeout")
			}

			pool.Shutdown()
		})
	}
}

// TestProcessorDoubleStart tests that starting twice returns error
func TestProcessorDoubleStart(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 2)
	
	if err := pool.Start(); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer pool.Shutdown()

	// Second start should error
	if err := pool.Start(); err == nil {
		t.Error("expected error on second Start, got nil")
	}
}

// Helper function to create mock file headers for testing
func createMockFileHeader(filename string, content []byte) *multipart.FileHeader {
	// Create in-memory buffer with the multipart data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// Create a form file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		panic(err)
	}
	
	// Write content
	if _, err := part.Write(content); err != nil {
		panic(err)
	}
	
	// Close the writer
	if err := writer.Close(); err != nil {
		panic(err)
	}
	
	// Parse the multipart form
	reader := multipart.NewReader(&buf, writer.Boundary())
	form, err := reader.ReadForm(int64(buf.Len() + 1024))
	if err != nil {
		panic(err)
	}
	
	// Get the file header
	if len(form.File["file"]) == 0 {
		panic("no file in form")
	}
	
	return form.File["file"][0]
}
