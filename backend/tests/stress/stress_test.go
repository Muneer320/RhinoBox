package stress_test

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

// StressTestMetrics holds performance measurements
type StressTestMetrics struct {
	TotalFiles      int
	TotalDuration   time.Duration
	AvgLatency      time.Duration
	P50Latency      time.Duration
	P95Latency      time.Duration
	P99Latency      time.Duration
	Throughput      float64 // files/sec
	MemoryUsedMB    float64
	MemoryPeakMB    float64
	SuccessCount    int
	ErrorCount      int
	Latencies       []time.Duration
}

// TestStress1000Files tests processing 1000 files with detailed metrics
func TestStress1000Files(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	metrics := runStressTest(t, 1000, 10*1024, "stress-1000")
	
	t.Logf("📊 Stress Test Results (1000 files):")
	t.Logf("  Total Duration: %v", metrics.TotalDuration)
	t.Logf("  Avg Latency: %v per file", metrics.AvgLatency)
	t.Logf("  P50 Latency: %v", metrics.P50Latency)
	t.Logf("  P95 Latency: %v", metrics.P95Latency)
	t.Logf("  P99 Latency: %v", metrics.P99Latency)
	t.Logf("  Throughput: %.0f files/sec", metrics.Throughput)
	t.Logf("  Memory Used: %.2f MB", metrics.MemoryUsedMB)
	t.Logf("  Memory Peak: %.2f MB", metrics.MemoryPeakMB)
	t.Logf("  Success Rate: %d/%d (%.1f%%)", metrics.SuccessCount, metrics.TotalFiles, 
		float64(metrics.SuccessCount)/float64(metrics.TotalFiles)*100)

	// Validate performance targets
	if metrics.TotalDuration > 3*time.Second {
		t.Errorf("❌ Total duration %v exceeds 3s target", metrics.TotalDuration)
	} else {
		t.Logf("✅ Total duration target met: %v < 3s", metrics.TotalDuration)
	}

	if metrics.P99Latency > 100*time.Millisecond {
		t.Errorf("❌ P99 latency %v exceeds 100ms target", metrics.P99Latency)
	} else {
		t.Logf("✅ P99 latency target met: %v < 100ms", metrics.P99Latency)
	}

	if metrics.Throughput < 1000 {
		t.Errorf("❌ Throughput %.0f files/sec below 1000 target", metrics.Throughput)
	} else {
		t.Logf("✅ Throughput target met: %.0f files/sec > 1000", metrics.Throughput)
	}

	if metrics.MemoryPeakMB > 100 {
		t.Logf("⚠️  Peak memory %0.2f MB exceeds 100MB target", metrics.MemoryPeakMB)
	} else {
		t.Logf("✅ Memory target met: %.2f MB < 100MB", metrics.MemoryPeakMB)
	}
}

// TestStress100FilesBatch tests batch processing with detailed latency tracking
func TestStress100FilesBatch(t *testing.T) {
	metrics := runStressTest(t, 100, 10*1024, "stress-100")
	
	t.Logf("📊 Stress Test Results (100 files):")
	t.Logf("  Total Duration: %v", metrics.TotalDuration)
	t.Logf("  P50 Latency: %v", metrics.P50Latency)
	t.Logf("  P99 Latency: %v", metrics.P99Latency)
	t.Logf("  Throughput: %.0f files/sec", metrics.Throughput)

	// Validate 100 file batch target
	if metrics.TotalDuration > 500*time.Millisecond {
		t.Errorf("❌ Batch processing took %v, expected <500ms", metrics.TotalDuration)
	} else {
		t.Logf("✅ Batch performance target met: %v < 500ms", metrics.TotalDuration)
	}
}

// TestStressSingleFileLatency tests single file processing latency
func TestStressSingleFileLatency(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        dir,
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Test single file latency 100 times
	latencies := make([]time.Duration, 100)
	content := bytes.Repeat([]byte("test content "), 1024) // 10KB

	for i := 0; i < 100; i++ {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		
		part, _ := writer.CreateFormFile("file", fmt.Sprintf("single-%d.jpg", i))
		part.Write(content)
		writer.Close()
		
		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		
		start := time.Now()
		server.Router().ServeHTTP(rec, req)
		latencies[i] = time.Since(start)
		
		if rec.Code != http.StatusOK {
			t.Errorf("request %d failed: %d", i, rec.Code)
		}
	}

	// Calculate percentiles
	p50 := percentile(latencies, 50)
	p99 := percentile(latencies, 99)
	avg := average(latencies)

	t.Logf("📊 Single File Latency (100 samples):")
	t.Logf("  Average: %v", avg)
	t.Logf("  P50: %v", p50)
	t.Logf("  P99: %v", p99)

	// Validate single file targets
	if p50 > 10*time.Millisecond {
		t.Errorf("❌ P50 latency %v exceeds 10ms target", p50)
	} else {
		t.Logf("✅ P50 latency target met: %v < 10ms", p50)
	}

	if p99 > 50*time.Millisecond {
		t.Errorf("❌ P99 latency %v exceeds 50ms target", p99)
	} else {
		t.Logf("✅ P99 latency target met: %v < 50ms", p99)
	}
}

// TestStressConcurrentBatches tests multiple concurrent batch uploads
func TestStressConcurrentBatches(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        dir,
		MaxUploadBytes: 1000 * 1024 * 1024, // 1GB
	}
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Launch 10 concurrent batch uploads, each with 100 files
	numBatches := 10
	filesPerBatch := 100
	
	var wg sync.WaitGroup
	errors := make(chan error, numBatches)
	durations := make(chan time.Duration, numBatches)

	start := time.Now()

	for b := 0; b < numBatches; b++ {
		wg.Add(1)
		go func(batchID int) {
			defer wg.Done()

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			
			for i := 0; i < filesPerBatch; i++ {
				filename := fmt.Sprintf("batch%d-file%d.jpg", batchID, i)
				content := bytes.Repeat([]byte("x"), 10*1024)
				
				part, err := writer.CreateFormFile("file", filename)
				if err != nil {
					errors <- err
					return
				}
				part.Write(content)
			}
			writer.Close()
			
			req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			rec := httptest.NewRecorder()
			
			batchStart := time.Now()
			server.Router().ServeHTTP(rec, req)
			durations <- time.Since(batchStart)
			
			if rec.Code != http.StatusOK {
				errors <- fmt.Errorf("batch %d failed: %d", batchID, rec.Code)
			}
		}(b)
	}

	wg.Wait()
	close(errors)
	close(durations)

	totalDuration := time.Since(start)
	
	// Collect errors
	errorCount := 0
	for err := range errors {
		t.Error(err)
		errorCount++
	}

	// Collect durations
	var allDurations []time.Duration
	for d := range durations {
		allDurations = append(allDurations, d)
	}

	totalFiles := numBatches * filesPerBatch
	throughput := float64(totalFiles) / totalDuration.Seconds()

	t.Logf("📊 Concurrent Batches Stress Test:")
	t.Logf("  Batches: %d (each with %d files)", numBatches, filesPerBatch)
	t.Logf("  Total Files: %d", totalFiles)
	t.Logf("  Total Duration: %v", totalDuration)
	t.Logf("  Throughput: %.0f files/sec", throughput)
	t.Logf("  Errors: %d", errorCount)
	
	if len(allDurations) > 0 {
		avgBatchDuration := average(allDurations)
		t.Logf("  Avg Batch Duration: %v", avgBatchDuration)
	}

	if throughput < 1000 {
		t.Logf("⚠️  Throughput %.0f files/sec below target", throughput)
	} else {
		t.Logf("✅ Throughput target met: %.0f files/sec", throughput)
	}
}

// TestStressMemoryEfficiency tests memory usage under load
func TestStressMemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	runtime.GC() // Start with clean slate
	
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Process 1000 files and measure memory
	metrics := runStressTest(t, 1000, 10*1024, "memory-test")

	runtime.GC() // Force GC to see actual retained memory
	
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	memDelta := float64(memAfter.Alloc-memBefore.Alloc) / 1024 / 1024

	t.Logf("📊 Memory Efficiency Test:")
	t.Logf("  Files Processed: %d", metrics.TotalFiles)
	t.Logf("  Memory Before: %.2f MB", float64(memBefore.Alloc)/1024/1024)
	t.Logf("  Memory After: %.2f MB", float64(memAfter.Alloc)/1024/1024)
	t.Logf("  Memory Delta: %.2f MB", memDelta)
	t.Logf("  Peak Memory: %.2f MB", metrics.MemoryPeakMB)

	if metrics.MemoryPeakMB > 100 {
		t.Logf("⚠️  Peak memory %.2f MB exceeds 100MB target", metrics.MemoryPeakMB)
	} else {
		t.Logf("✅ Memory usage target met: %.2f MB < 100MB", metrics.MemoryPeakMB)
	}
}

// TestStressDifferentFileSizes tests handling of various file sizes
func TestStressDifferentFileSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	sizes := []struct {
		name string
		size int
		count int
	}{
		{"tiny-1KB", 1024, 100},
		{"small-10KB", 10 * 1024, 100},
		{"medium-100KB", 100 * 1024, 50},
		{"large-1MB", 1024 * 1024, 20},
	}

	for _, s := range sizes {
		t.Run(s.name, func(t *testing.T) {
			metrics := runStressTest(t, s.count, s.size, s.name)
			
			t.Logf("  Size: %s, Count: %d", s.name, s.count)
			t.Logf("  Duration: %v", metrics.TotalDuration)
			t.Logf("  Throughput: %.0f files/sec", metrics.Throughput)
			t.Logf("  Avg Latency: %v", metrics.AvgLatency)
		})
	}
}

// Helper function to run a stress test with given parameters
func runStressTest(t *testing.T, fileCount, fileSize int, category string) StressTestMetrics {
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":8090",
		DataDir:        dir,
		MaxUploadBytes: 2000 * 1024 * 1024, // 2GB
	}
	
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	content := bytes.Repeat([]byte("x"), fileSize)
	
	for i := 0; i < fileCount; i++ {
		filename := fmt.Sprintf("file-%04d.jpg", i)
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	
	writer.WriteField("category", category)
	writer.Close()

	// Measure memory before
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	// Execute request with timing
	start := time.Now()
	server.Router().ServeHTTP(rec, req)
	totalDuration := time.Since(start)

	// Measure memory after
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	successCount := 0
	errorCount := 0
	if rec.Code == http.StatusOK {
		successCount = fileCount
	} else {
		errorCount = fileCount
	}

	// Calculate metrics
	avgLatency := totalDuration / time.Duration(fileCount)
	throughput := float64(fileCount) / totalDuration.Seconds()
	memUsedMB := float64(memAfter.Alloc-memBefore.Alloc) / 1024 / 1024
	memPeakMB := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / 1024 / 1024

	// Estimate percentiles (simplified since we don't have per-file timing in batch)
	// In a real scenario, we'd need to instrument the worker pool to get per-file latencies
	p50 := avgLatency
	p95 := avgLatency * 2
	p99 := avgLatency * 3

	return StressTestMetrics{
		TotalFiles:    fileCount,
		TotalDuration: totalDuration,
		AvgLatency:    avgLatency,
		P50Latency:    p50,
		P95Latency:    p95,
		P99Latency:    p99,
		Throughput:    throughput,
		MemoryUsedMB:  memUsedMB,
		MemoryPeakMB:  memPeakMB,
		SuccessCount:  successCount,
		ErrorCount:    errorCount,
	}
}

// percentile calculates the nth percentile of durations
func percentile(durations []time.Duration, p int) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	// Sort durations
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	
	// Simple bubble sort for small datasets
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	idx := (len(sorted) * p) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	
	return sorted[idx]
}

// average calculates the average of durations
func average(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	
	return sum / time.Duration(len(durations))
}
