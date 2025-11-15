package media_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/media"
	"github.com/Muneer320/RhinoBox/internal/storage"
)

// BenchmarkProcessSingleFile benchmarks single file processing
func BenchmarkProcessSingleFile(b *testing.B) {
	dir := b.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		b.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 0)
	
	if err := pool.Start(); err != nil {
		b.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	// Create test content (10KB)
	content := bytes.Repeat([]byte("test content "), 1024)
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		header := createMockFileHeader(fmt.Sprintf("file-%d.jpg", i), content)
		job := &media.ProcessJob{
			Header:       header,
			CategoryHint: "benchmark",
			JobID:        fmt.Sprintf("job-%d", i),
			Index:        i,
		}

		if err := pool.Submit(job); err != nil {
			b.Fatalf("Submit: %v", err)
		}

		select {
		case result := <-pool.Results():
			if !result.Success {
				b.Fatalf("job failed: %v", result.Error)
			}
		case <-time.After(5 * time.Second):
			b.Fatal("timeout")
		}
	}
}

// BenchmarkProcessBatch100 benchmarks processing 100 files concurrently
func BenchmarkProcessBatch100(b *testing.B) {
	dir := b.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		b.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	content := bytes.Repeat([]byte("test content "), 1024) // 10KB files
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pool := media.NewWorkerPool(ctx, store, 0)
		if err := pool.Start(); err != nil {
			b.Fatalf("Start: %v", err)
		}

		// Submit 100 files
		batchSize := 100
		for j := 0; j < batchSize; j++ {
			header := createMockFileHeader(fmt.Sprintf("file-%d-%d.jpg", i, j), content)
			job := &media.ProcessJob{
				Header: header,
				JobID:  fmt.Sprintf("job-%d-%d", i, j),
				Index:  j,
			}
			if err := pool.Submit(job); err != nil {
				b.Fatalf("Submit: %v", err)
			}
		}

		// Collect results
		for j := 0; j < batchSize; j++ {
			select {
			case result := <-pool.Results():
				if !result.Success {
					b.Fatalf("job failed: %v", result.Error)
				}
			case <-time.After(10 * time.Second):
				b.Fatal("timeout")
			}
		}

		pool.Shutdown()
	}
}

// BenchmarkProcessBatch1000 benchmarks processing 1000 files concurrently
func BenchmarkProcessBatch1000(b *testing.B) {
	dir := b.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		b.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	content := bytes.Repeat([]byte("test content "), 1024) // 10KB files
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pool := media.NewWorkerPool(ctx, store, 0)
		if err := pool.Start(); err != nil {
			b.Fatalf("Start: %v", err)
		}

		// Submit 1000 files
		batchSize := 1000
		for j := 0; j < batchSize; j++ {
			header := createMockFileHeader(fmt.Sprintf("file-%d-%d.jpg", i, j), content)
			job := &media.ProcessJob{
				Header: header,
				JobID:  fmt.Sprintf("job-%d-%d", i, j),
				Index:  j,
			}
			if err := pool.Submit(job); err != nil {
				b.Fatalf("Submit: %v", err)
			}
		}

		// Collect results
		for j := 0; j < batchSize; j++ {
			select {
			case result := <-pool.Results():
				if !result.Success {
					b.Fatalf("job failed: %v", result.Error)
				}
			case <-time.After(30 * time.Second):
				b.Fatal("timeout")
			}
		}

		pool.Shutdown()
	}
}

// BenchmarkProcessDifferentSizes benchmarks files of varying sizes
func BenchmarkProcessDifferentSizes(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			dir := b.TempDir()
			store, err := storage.NewManager(dir)
			if err != nil {
				b.Fatalf("NewManager: %v", err)
			}

			ctx := context.Background()
			pool := media.NewWorkerPool(ctx, store, 0)
			
			if err := pool.Start(); err != nil {
				b.Fatalf("Start: %v", err)
			}
			defer pool.Shutdown()

			content := bytes.Repeat([]byte("x"), size.size)
			
			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(size.size))

			for i := 0; i < b.N; i++ {
				header := createMockFileHeader(fmt.Sprintf("file-%d.jpg", i), content)
				job := &media.ProcessJob{
					Header: header,
					JobID:  fmt.Sprintf("job-%d", i),
					Index:  i,
				}

				if err := pool.Submit(job); err != nil {
					b.Fatalf("Submit: %v", err)
				}

				select {
				case result := <-pool.Results():
					if !result.Success {
						b.Fatalf("job failed: %v", result.Error)
					}
				case <-time.After(10 * time.Second):
					b.Fatal("timeout")
				}
			}
		})
	}
}

// BenchmarkWorkerPoolScalability benchmarks different worker counts
func BenchmarkWorkerPoolScalability(b *testing.B) {
	workerCounts := []int{1, 2, 4, 8, 16}
	
	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("%d-workers", workers), func(b *testing.B) {
			dir := b.TempDir()
			store, err := storage.NewManager(dir)
			if err != nil {
				b.Fatalf("NewManager: %v", err)
			}

			ctx := context.Background()
			content := bytes.Repeat([]byte("test "), 2048) // 10KB
			
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				pool := media.NewWorkerPool(ctx, store, workers)
				if err := pool.Start(); err != nil {
					b.Fatalf("Start: %v", err)
				}

				// Process 50 files
				batchSize := 50
				for j := 0; j < batchSize; j++ {
					header := createMockFileHeader(fmt.Sprintf("file-%d.jpg", j), content)
					job := &media.ProcessJob{
						Header: header,
						JobID:  fmt.Sprintf("job-%d", j),
						Index:  j,
					}
					pool.Submit(job)
				}

				for j := 0; j < batchSize; j++ {
					<-pool.Results()
				}

				pool.Shutdown()
			}
		})
	}
}

// BenchmarkProcessorOverhead measures the overhead of the worker pool
func BenchmarkProcessorOverhead(b *testing.B) {
	dir := b.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		b.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pool := media.NewWorkerPool(ctx, store, 4)
		if err := pool.Start(); err != nil {
			b.Fatalf("Start: %v", err)
		}
		pool.Shutdown()
	}
}

// BenchmarkMemoryPooling benchmarks the buffer pool efficiency
func BenchmarkMemoryPooling(b *testing.B) {
	dir := b.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		b.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 8)
	
	if err := pool.Start(); err != nil {
		b.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	// Small files to emphasize buffer pool usage
	content := []byte("small file")
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		header := createMockFileHeader(fmt.Sprintf("file-%d.jpg", i), content)
		job := &media.ProcessJob{
			Header: header,
			JobID:  fmt.Sprintf("job-%d", i),
			Index:  i,
		}

		pool.Submit(job)
		<-pool.Results()
	}
}

// BenchmarkThroughput measures files per second
func BenchmarkThroughput(b *testing.B) {
	dir := b.TempDir()
	store, err := storage.NewManager(dir)
	if err != nil {
		b.Fatalf("NewManager: %v", err)
	}

	ctx := context.Background()
	pool := media.NewWorkerPool(ctx, store, 0)
	
	if err := pool.Start(); err != nil {
		b.Fatalf("Start: %v", err)
	}
	defer pool.Shutdown()

	content := bytes.Repeat([]byte("test "), 2048) // ~10KB
	
	// Pre-create all jobs
	numFiles := 100
	jobs := make([]*media.ProcessJob, numFiles)
	for i := 0; i < numFiles; i++ {
		header := createMockFileHeader(fmt.Sprintf("file-%d.jpg", i), content)
		jobs[i] = &media.ProcessJob{
			Header: header,
			JobID:  fmt.Sprintf("job-%d", i),
			Index:  i,
		}
	}

	b.ResetTimer()
	
	start := time.Now()
	
	// Submit all jobs
	for _, job := range jobs {
		if err := pool.Submit(job); err != nil {
			b.Fatalf("Submit: %v", err)
		}
	}

	// Collect all results
	successCount := 0
	for i := 0; i < numFiles; i++ {
		result := <-pool.Results()
		if result.Success {
			successCount++
		}
	}

	elapsed := time.Since(start)
	
	b.ReportMetric(float64(successCount)/elapsed.Seconds(), "files/sec")
	b.ReportMetric(float64(elapsed.Milliseconds())/float64(successCount), "ms/file")
}
