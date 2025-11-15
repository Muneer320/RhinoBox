package stress_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

// TestSearchPerformance measures search endpoint latency and throughput
func TestSearchPerformance(t *testing.T) {
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Upload test files for searching
	const numFiles = 100
	fileHashes := make([]string, 0, numFiles)
	extensions := []string{"pdf", "jpg", "png", "txt", "doc"}

	for i := 0; i < numFiles; i++ {
		ext := extensions[i%len(extensions)]
		content := fmt.Sprintf("test file content %d unique data %d", i, time.Now().UnixNano())
		filename := fmt.Sprintf("test_file_%d.%s", i, ext)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d: %s", resp.Code, resp.Body.String())
		}

		var uploadResp struct {
			Stored []map[string]any `json:"stored"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err == nil && len(uploadResp.Stored) > 0 {
			if hash, ok := uploadResp.Stored[0]["hash"].(string); ok {
				fileHashes = append(fileHashes, hash)
			}
		}
	}

	t.Logf("Uploaded %d files for performance testing", len(fileHashes))

	// Test single search query performance
	t.Run("single_query_latency", func(t *testing.T) {
		const iterations = 100
		var totalDuration time.Duration
		var minDuration, maxDuration time.Duration = time.Hour, 0

		for i := 0; i < iterations; i++ {
			start := time.Now()
			req := httptest.NewRequest(http.MethodGet, "/files/search?extension=pdf", nil)
			resp := httptest.NewRecorder()
			srv.Router().ServeHTTP(resp, req)

			duration := time.Since(start)
			totalDuration += duration

			if duration < minDuration {
				minDuration = duration
			}
			if duration > maxDuration {
				maxDuration = duration
			}

			if resp.Code != http.StatusOK {
				t.Fatalf("search failed: %d: %s", resp.Code, resp.Body.String())
			}
		}

		avgDuration := totalDuration / iterations
		t.Logf("Single query performance (100 iterations):")
		t.Logf("  Average latency: %v", avgDuration)
		t.Logf("  Min latency: %v", minDuration)
		t.Logf("  Max latency: %v", maxDuration)
		t.Logf("  Throughput: %.2f queries/sec", float64(iterations)/totalDuration.Seconds())

		// Performance assertions
		if avgDuration > 100*time.Millisecond {
			t.Errorf("average latency too high: %v (expected < 100ms)", avgDuration)
		}
		if maxDuration > 500*time.Millisecond {
			t.Errorf("max latency too high: %v (expected < 500ms)", maxDuration)
		}
	})

	// Test concurrent search queries
	t.Run("concurrent_queries", func(t *testing.T) {
		const numConcurrent = 50
		const queriesPerGoroutine = 10

		var wg sync.WaitGroup
		start := time.Now()
		errors := make(chan error, numConcurrent*queriesPerGoroutine)

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < queriesPerGoroutine; j++ {
					ext := extensions[workerID%len(extensions)]
					req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/files/search?extension=%s", ext), nil)
					resp := httptest.NewRecorder()
					srv.Router().ServeHTTP(resp, req)

					if resp.Code != http.StatusOK {
						errors <- fmt.Errorf("search failed: %d", resp.Code)
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)
		totalDuration := time.Since(start)

		// Check for errors
		errorCount := 0
		for err := range errors {
			if err != nil {
				errorCount++
				t.Logf("Error: %v", err)
			}
		}

		totalQueries := numConcurrent * queriesPerGoroutine
		t.Logf("Concurrent queries performance (%d concurrent workers, %d queries each):", numConcurrent, queriesPerGoroutine)
		t.Logf("  Total queries: %d", totalQueries)
		t.Logf("  Total duration: %v", totalDuration)
		t.Logf("  Throughput: %.2f queries/sec", float64(totalQueries)/totalDuration.Seconds())
		t.Logf("  Errors: %d", errorCount)

		if errorCount > 0 {
			t.Errorf("encountered %d errors during concurrent queries", errorCount)
		}
		if totalDuration > 5*time.Second {
			t.Errorf("concurrent queries took too long: %v (expected < 5s)", totalDuration)
		}
	})

	// Test different filter combinations performance
	t.Run("filter_combinations", func(t *testing.T) {
		filterTests := []struct {
			name string
			url  string
		}{
			{"name_only", "/files/search?name=test"},
			{"extension_only", "/files/search?extension=pdf"},
			{"type_only", "/files/search?type=image"},
			{"name_and_extension", "/files/search?name=test&extension=pdf"},
			{"name_and_type", "/files/search?name=test&type=image"},
			{"extension_and_type", "/files/search?extension=jpg&type=image"},
		}

		for _, ft := range filterTests {
			t.Run(ft.name, func(t *testing.T) {
				const iterations = 50
				var totalDuration time.Duration

				for i := 0; i < iterations; i++ {
					start := time.Now()
					req := httptest.NewRequest(http.MethodGet, ft.url, nil)
					resp := httptest.NewRecorder()
					srv.Router().ServeHTTP(resp, req)

					if resp.Code != http.StatusOK {
						t.Fatalf("search failed: %d: %s", resp.Code, resp.Body.String())
					}

					totalDuration += time.Since(start)
				}

				avgDuration := totalDuration / iterations
				t.Logf("  Average latency: %v", avgDuration)
				t.Logf("  Throughput: %.2f queries/sec", float64(iterations)/totalDuration.Seconds())

				if avgDuration > 200*time.Millisecond {
					t.Errorf("average latency too high for %s: %v (expected < 200ms)", ft.name, avgDuration)
				}
			})
		}
	})
}

// TestSearchReliability tests search endpoint reliability under load
func TestSearchReliability(t *testing.T) {
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 100 * 1024 * 1024,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Upload files
	const numFiles = 50
	for i := 0; i < numFiles; i++ {
		content := fmt.Sprintf("reliability test file %d", i)
		filename := fmt.Sprintf("reliability_%d.txt", i)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		fileWriter, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		fileWriter.Write([]byte(content))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp := httptest.NewRecorder()
		srv.Router().ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("upload failed: %d", resp.Code)
		}
	}

	// Run reliability test
	const totalQueries = 1000
	const numWorkers = 20
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			queriesPerWorker := totalQueries / numWorkers
			for j := 0; j < queriesPerWorker; j++ {
				req := httptest.NewRequest(http.MethodGet, "/files/search?name=reliability", nil)
				resp := httptest.NewRecorder()
				srv.Router().ServeHTTP(resp, req)

				mu.Lock()
				if resp.Code == http.StatusOK {
					successCount++
				} else {
					errorCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(start)

	successRate := float64(successCount) / float64(totalQueries) * 100
	t.Logf("Reliability test results:")
	t.Logf("  Total queries: %d", totalQueries)
	t.Logf("  Successful: %d", successCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Success rate: %.2f%%", successRate)
	t.Logf("  Total duration: %v", totalDuration)
	t.Logf("  Average QPS: %.2f", float64(totalQueries)/totalDuration.Seconds())

	if successRate < 99.0 {
		t.Errorf("success rate too low: %.2f%% (expected >= 99%%)", successRate)
	}
}

// TestSearchScalability tests search performance with varying dataset sizes
func TestSearchScalability(t *testing.T) {
	datasetSizes := []int{10, 50, 100, 500}

	for _, size := range datasetSizes {
		t.Run(fmt.Sprintf("dataset_size_%d", size), func(t *testing.T) {
			cfg := config.Config{
				Addr:           ":0",
				DataDir:        t.TempDir(),
				MaxUploadBytes: 100 * 1024 * 1024,
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			srv, err := api.NewServer(cfg, logger)
			if err != nil {
				t.Fatalf("failed to create server: %v", err)
			}

			// Upload files
			for i := 0; i < size; i++ {
				content := fmt.Sprintf("scalability test file %d", i)
				filename := fmt.Sprintf("scalability_%d.txt", i)

				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				fileWriter, err := writer.CreateFormFile("file", filename)
				if err != nil {
					t.Fatalf("create form file: %v", err)
				}
				fileWriter.Write([]byte(content))
				writer.Close()

				req := httptest.NewRequest(http.MethodPost, "/ingest/media", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				resp := httptest.NewRecorder()
				srv.Router().ServeHTTP(resp, req)

				if resp.Code != http.StatusOK {
					t.Fatalf("upload failed: %d", resp.Code)
				}
			}

			// Measure search performance
			const iterations = 20
			var totalDuration time.Duration

			for i := 0; i < iterations; i++ {
				start := time.Now()
				req := httptest.NewRequest(http.MethodGet, "/files/search?name=scalability", nil)
				resp := httptest.NewRecorder()
				srv.Router().ServeHTTP(resp, req)

				if resp.Code != http.StatusOK {
					t.Fatalf("search failed: %d", resp.Code)
				}

				totalDuration += time.Since(start)
			}

			avgDuration := totalDuration / iterations
			t.Logf("Dataset size: %d files", size)
			t.Logf("  Average search latency: %v", avgDuration)
			t.Logf("  Throughput: %.2f queries/sec", float64(iterations)/totalDuration.Seconds())

			// Scalability check: latency should not grow too much with dataset size
			maxExpectedLatency := time.Duration(size/10) * time.Millisecond
			if maxExpectedLatency < 10*time.Millisecond {
				maxExpectedLatency = 10 * time.Millisecond
			}
			if maxExpectedLatency > 200*time.Millisecond {
				maxExpectedLatency = 200 * time.Millisecond
			}

			if avgDuration > maxExpectedLatency {
				t.Logf("  Warning: latency %v exceeds expected %v for dataset size %d", avgDuration, maxExpectedLatency, size)
			}
		})
	}
}
