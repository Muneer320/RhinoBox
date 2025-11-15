package queue

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// MockProcessor is a simple processor for testing
type MockProcessor struct {
	processedItems []string
	mu             sync.Mutex
	failItems      map[string]bool // Items that should fail
	delay          time.Duration
}

func NewMockProcessor() *MockProcessor {
	return &MockProcessor{
		processedItems: []string{},
		failItems:      make(map[string]bool),
	}
}

func (mp *MockProcessor) ProcessItem(job *Job, item *JobItem) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if mp.delay > 0 {
		time.Sleep(mp.delay)
	}

	mp.processedItems = append(mp.processedItems, item.ID)

	if mp.failItems[item.ID] {
		return fmt.Errorf("mock error for item %s", item.ID)
	}

	item.Result = &JobItemResult{
		StoredPath:  "/test/" + item.ID,
		Hash:        "hash_" + item.ID,
		IsDuplicate: false,
	}

	return nil
}

func (mp *MockProcessor) GetProcessedCount() int {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	return len(mp.processedItems)
}

func (mp *MockProcessor) SetFail(itemID string) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.failItems[itemID] = true
}

func TestJobQueueBasic(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		MaxWorkers:  2,
		PersistPath: tmpDir,
		MaxRetries:  3,
	}

	processor := NewMockProcessor()
	queue, err := New(cfg, processor)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer queue.Stop()

	// Create a simple job
	job := &Job{
		Type: JobTypeMedia,
		Items: []JobItem{
			{ID: "item1", Type: "test", Name: "test1.txt"},
			{ID: "item2", Type: "test", Name: "test2.txt"},
			{ID: "item3", Type: "test", Name: "test3.txt"},
		},
	}

	// Enqueue job
	if err := queue.Enqueue(job); err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Wait for processing with timeout
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	var result *JobResult
	var found bool

checkLoop:
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for job completion")
		case <-ticker.C:
			result, found = queue.GetResult(job.ID)
			if found {
				break checkLoop
			}
		}
	}

	if !found {
		t.Fatal("Job result not found")
	}

	if result.Succeeded != 3 {
		t.Errorf("Expected 3 succeeded, got %d", result.Succeeded)
	}

	if result.Failed != 0 {
		t.Errorf("Expected 0 failed, got %d", result.Failed)
	}
}

func TestJobQueuePartialFailure(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		MaxWorkers:  1,
		PersistPath: tmpDir,
		MaxRetries:  0,
	}

	processor := NewMockProcessor()
	processor.SetFail("item2")

	queue, err := New(cfg, processor)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer queue.Stop()

	job := &Job{
		Type: JobTypeMedia,
		Items: []JobItem{
			{ID: "item1", Type: "test"},
			{ID: "item2", Type: "test"}, // Will fail
			{ID: "item3", Type: "test"},
		},
	}

	if err := queue.Enqueue(job); err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	result, found := queue.GetResult(job.ID)
	if !found {
		t.Fatal("Job result not found")
	}

	if result.Succeeded != 2 {
		t.Errorf("Expected 2 succeeded, got %d", result.Succeeded)
	}

	if result.Failed != 1 {
		t.Errorf("Expected 1 failed, got %d", result.Failed)
	}

	if result.Status != StatusCompleted {
		t.Errorf("Expected partial success status %s, got %s", StatusCompleted, result.Status)
	}
}

func TestJobQueueConcurrency(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		MaxWorkers:  5,
		PersistPath: tmpDir,
	}

	processor := NewMockProcessor()
	processor.delay = 50 * time.Millisecond

	queue, err := New(cfg, processor)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer queue.Stop()

	// Enqueue multiple jobs concurrently
	numJobs := 10
	itemsPerJob := 5

	var wg sync.WaitGroup
	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(jobNum int) {
			defer wg.Done()

			items := make([]JobItem, itemsPerJob)
			for j := 0; j < itemsPerJob; j++ {
				items[j] = JobItem{
					ID:   fmt.Sprintf("job%d_item%d", jobNum, j),
					Type: "test",
				}
			}

			job := &Job{
				Type:  JobTypeMedia,
				Items: items,
			}

			if err := queue.Enqueue(job); err != nil {
				t.Errorf("Failed to enqueue job %d: %v", jobNum, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all jobs to complete with timeout
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	expectedTotal := numJobs * itemsPerJob

waitLoop:
	for {
		select {
		case <-timeout:
			actualCount := processor.GetProcessedCount()
			t.Fatalf("Timeout: Expected %d items processed, got %d", expectedTotal, actualCount)
		case <-ticker.C:
			if processor.GetProcessedCount() >= expectedTotal {
				break waitLoop
			}
		}
	}

	actualCount := processor.GetProcessedCount()
	if actualCount != expectedTotal {
		t.Errorf("Expected %d items processed, got %d", expectedTotal, actualCount)
	}

	// Check queue stats
	stats := queue.Stats()
	t.Logf("Queue stats: %+v", stats)
}

func TestJobQueuePersistence(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		MaxWorkers:  1,
		PersistPath: tmpDir,
	}

	// Create queue and enqueue a job
	processor1 := NewMockProcessor()
	queue1, err := New(cfg, processor1)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	job := &Job{
		Type: JobTypeMedia,
		Items: []JobItem{
			{ID: "item1", Type: "test"},
		},
	}

	if err := queue1.Enqueue(job); err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	queue1.Stop()

	// Create new queue - should restore job
	processor2 := NewMockProcessor()
	queue2, err := New(cfg, processor2)
	if err != nil {
		t.Fatalf("Failed to create second queue: %v", err)
	}
	defer queue2.Stop()

	// Job should exist
	retrievedJob, found := queue2.Get(job.ID)
	if !found {
		t.Error("Job not found after restart")
	}

	if retrievedJob.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, retrievedJob.ID)
	}
}

func TestJobQueueStats(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		MaxWorkers:  3,
		PersistPath: tmpDir,
	}

	processor := NewMockProcessor()
	processor.delay = 100 * time.Millisecond

	queue, err := New(cfg, processor)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer queue.Stop()

	// Enqueue several jobs
	for i := 0; i < 5; i++ {
		job := &Job{
			Type: JobTypeMedia,
			Items: []JobItem{
				{ID: fmt.Sprintf("item_%d", i), Type: "test"},
			},
		}
		queue.Enqueue(job)
	}

	stats := queue.Stats()

	if workers, ok := stats["workers"].(int); !ok || workers != 3 {
		t.Errorf("Expected 3 workers, got %v", stats["workers"])
	}

	t.Logf("Queue stats: %+v", stats)
}

func TestJobQueueListJobs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		MaxWorkers:  2,
		PersistPath: tmpDir,
	}

	processor := NewMockProcessor()
	processor.delay = 50 * time.Millisecond

	queue, err := New(cfg, processor)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}
	defer queue.Stop()

	// Enqueue jobs
	numJobs := 5
	for i := 0; i < numJobs; i++ {
		job := &Job{
			Type: JobTypeMedia,
			Items: []JobItem{
				{ID: fmt.Sprintf("item_%d", i), Type: "test"},
			},
		}
		queue.Enqueue(job)
	}

	// List jobs while processing
	time.Sleep(100 * time.Millisecond)
	jobs := queue.ListJobs()

	if len(jobs) > numJobs {
		t.Errorf("Expected at most %d jobs, got %d", numJobs, len(jobs))
	}

	t.Logf("Found %d active jobs", len(jobs))
}

func BenchmarkJobQueue(b *testing.B) {
	tmpDir := b.TempDir()

	cfg := Config{
		MaxWorkers:  10,
		PersistPath: tmpDir,
	}

	processor := NewMockProcessor()
	queue, err := New(cfg, processor)
	if err != nil {
		b.Fatalf("Failed to create queue: %v", err)
	}
	defer queue.Stop()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		job := &Job{
			Type: JobTypeMedia,
			Items: []JobItem{
				{ID: fmt.Sprintf("item_%d", i), Type: "test"},
			},
		}
		queue.Enqueue(job)
	}

	// Wait for all jobs to complete
	time.Sleep(time.Duration(b.N/10) * time.Millisecond)
}
